package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/item"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
	"source/internal/words"
)

// newItemRun makes a newLoopRun where reviewer blocks at rounds 1 and 2, so D1 is a draft.
func newItemRun(t *testing.T) string {
	t.Helper()
	run := newLoopRun(t)
	fixRound(t, run, 1)
	submitFile(t, run, "a.md", "v2\n")
	openRound(t, run)
	writeVerdict(t, run, 2, "reviewer", "block", round.Finding{ID: "F1", Severity: "blocker"})
	closeRound(t, run, 2, "ACCEPT", "item D1 draft for reviewer")
	return run
}

// item open refuses a blank way back, an unknown key, the dev holder, and an unknown or path-like id, each with no
// write. A full stdin opens D1 with the three fields and records ITEM D1 for brain. A second open gives 2.
func TestItemOpen(t *testing.T) {
	run := newItemRun(t)
	d, p := store.Dir{Root: run}, item.Path(store.Dir{Root: run}, "D1")
	full := `{"default": "keep v1", "way_back": "phase back research", "affects": "design"}`
	for i, tc := range []struct {
		id, holder, stdin string
		code              int
		e                 string // stderr after "monet-source: ", or "" for none
	}{
		{"D1", "h1", strings.Replace(full, `"phase back research"`, `" \t "`, 1), exit.Refused, "item D1: no way back"},
		{"D1", "h1", strings.Replace(full, `}`, `, "note": "x"}`, 1), exit.Refused, `stdin: json: unknown field "note"`},
		{"D1", "d1", full, exit.Held, "lock held: brain is not held by d1"},
		{"D9", "h1", full, exit.Refused, "item D9: no such item"},
		{"../state", "h1", full, exit.Refused, "item ../state: no such item"},
		{"D1/../../state", "h1", full, exit.Refused, "item D1/../../state: no such item"},
		{"../items/D1", "h1", full, exit.Refused, "item ../items/D1: no such item"},
		{"D1", "h1", full, exit.OK, ""},
		{"D1", "h1", full, exit.Refused, "item D1 is open"},
	} {
		before, evs := readOr(t, p), readOr(t, d.Events())
		_, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(tc.stdin)}, "item", "open", tc.id, "--holder", tc.holder)
		if want := "monet-source: " + tc.e + "\n"; code != tc.code || (tc.e == "") != (e == "") || (tc.e != "" && e != want) {
			t.Fatalf("row %d: code %d, stderr %q; want %d, %q", i, code, e, tc.code, tc.e)
		}
		if code != exit.OK && (readOr(t, p) != before || readOr(t, d.Events()) != evs) {
			t.Fatalf("row %d: a refusal wrote D1 or an event", i)
		}
		if code != exit.OK {
			continue
		}
		var got item.Item
		want := item.Item{ID: "D1", Phase: "research", Persona: "reviewer", Round: 2, Findings: []string{"F1"}, Status: "open",
			Default: "keep v1", WayBack: "phase back research", Affects: "design"}
		lines, _, err := events.For(d, "brain", 0)
		if item.Load(d, "D1", &got) != nil || !reflect.DeepEqual(got, want) || err != nil || len(lines) == 0 || lines[len(lines)-1] != "ITEM D1" ||
			!strings.HasSuffix(readOr(t, d.Log()), "\tITEM D1\n") {
			t.Fatalf("row %d: D1 %+v, brain events %q (err %v); want %+v and ITEM D1 last, also in log.tsv", i, got, lines, err, want)
		}
	}
}

// item open with no id, two ids, or no subcommand is a usage error.
func TestItemOpenUsage(t *testing.T) {
	run := newItemRun(t)
	for _, args := range [][]string{{"item", "open", "--holder", "h1"}, {"item", "open", "D1", "D2", "--holder", "h1"}, {"item"}} {
		if _, _, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader("{}")}, args...); code != exit.Usage {
			t.Errorf("%q: code %d; want %d", args, code, exit.Usage)
		}
	}
}

// openD1 opens D1 on run as h1 and adds the words w2, so that item rule finds an open item and a words entry.
func openD1(t *testing.T, run string) {
	t.Helper()
	in := `{"default": "keep v1", "way_back": "phase back research", "affects": "design"}`
	if _, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(in)}, "item", "open", "D1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("item open: code %d, stderr %q", code, e)
	}
	if err := words.Append(store.Dir{Root: run}.Words(), words.Entry{ID: "w2", At: "2026-11-01T12:30:00Z", Text: "Keep v1."}); err != nil {
		t.Fatal(err)
	}
}

// ruleD1 runs item rule D1 with pick, the words id, and holder at now, and returns the exit code and stderr.
func ruleD1(t *testing.T, run, pick, wid, holder string, now time.Time) (int, string) {
	t.Helper()
	_, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return now }}, "item", "rule", "D1", "--pick="+pick, "--words", wid, "--holder", holder)
	return code, e
}

// runFiles is state.json, D1, and events.tsv of run, so that a test can tell a write.
func runFiles(t *testing.T, run string) string {
	d := store.Dir{Root: run}
	return readOr(t, d.State()) + readOr(t, item.Path(d, "D1")) + readOr(t, d.Events())
}

// A draft D1 gives 2. The rows then run in order on D1, open, with counts for reviewer and reader in research. An
// unknown words id and the dev holder d1 write nothing. --pick default renews brain, saves D1 as ruled with default
// and w2, deletes the count of reviewer only, keeps the phase, and records RULING D1 default w2 for dev. A second
// rule gives 2 and writes nothing.
func TestItemRuleDefault(t *testing.T) {
	run := newItemRun(t)
	d := store.Dir{Root: run}
	if code, e := ruleD1(t, run, "default", "w1", "h1", testNow); code != exit.Refused || e != "monet-source: item D1 is draft\n" {
		t.Fatalf("draft D1: code %d, stderr %q; want 2", code, e)
	}
	openD1(t, run)
	if err := state.Update(d, func(s *state.State) error { s.Counts["research"]["reader"] = 1; return nil }); err != nil {
		t.Fatal(err)
	}
	s0, err := state.Load(d)
	if _, ok := s0.Counts["research"]["reviewer"]; err != nil || !ok || s0.Phase != "research" {
		t.Fatalf("before: counts %v, phase %q (err %v); want a reviewer count in research", s0.Counts, s0.Phase, err)
	}
	for i, tc := range []struct {
		wid, holder string
		code        int
		e           string // stderr after "monet-source: ", or "" for none
	}{
		{"w9", "h1", exit.Refused, "owner words: no entry w9"},
		{"w2", "d1", exit.Held, "lock held: brain is not held by d1"},
		{"w2", "h1", exit.OK, ""},
		{"w2", "h1", exit.Refused, "item D1 is ruled"},
	} {
		now, before := testNow.Add(time.Duration(i+1)*time.Hour), runFiles(t, run)
		code, e := ruleD1(t, run, "default", tc.wid, tc.holder, now)
		if want := "monet-source: " + tc.e + "\n"; code != tc.code || (tc.e == "") != (e == "") || (tc.e != "" && e != want) {
			t.Fatalf("row %d: code %d, stderr %q; want %d, %q", i, code, e, tc.code, tc.e)
		}
		if code != exit.OK {
			if runFiles(t, run) != before {
				t.Fatalf("row %d: a refusal wrote state.json, D1, or an event", i)
			}
			continue
		}
		var got item.Item
		want := item.Item{ID: "D1", Phase: "research", Persona: "reviewer", Round: 2, Findings: []string{"F1"}, Status: "ruled",
			Default: "keep v1", WayBack: "phase back research", Affects: "design", Pick: "default", Words: "w2"}
		s, serr := state.Load(d)
		_, counted := s.Counts["research"]["reviewer"]
		lines, _, lerr := events.For(d, "dev", 0)
		if item.Load(d, "D1", &got) != nil || !reflect.DeepEqual(got, want) || serr != nil || counted || s.Counts["research"]["reader"] != 1 ||
			s.Phase != "research" || s.Moves != s0.Moves || lerr != nil || len(lines) == 0 || lines[len(lines)-1] != "RULING D1 default w2" ||
			!strings.HasSuffix(readOr(t, d.Log()), "\tRULING D1 default w2\n") {
			t.Fatalf("row %d: D1 %+v, counts %v, phase %q, moves %d, dev events %q (errs %v %v); want %+v and RULING D1 default w2 last",
				i, got, s.Counts, s.Phase, s.Moves, lines, serr, lerr, want)
		}
		checkLease(t, run, "brain", now.Add(leaseTTL))
	}
}

// With D1 open, --pick other from design moves the run back to research, the phase of D1, and the dev events and
// log.tsv end with RULING D1 other w2 and then PHASE <n> research. From research itself it moves nothing and
// records only the RULING line. --pick default from design with the words w3 keeps design and records only
// RULING D1 default w3. Each rule saves D1 as ruled with its pick and words id.
func TestItemRuleOtherSendsBack(t *testing.T) {
	for _, tc := range []struct {
		pick, wid string
		next      bool
	}{{"other", "w2", true}, {"other", "w2", false}, {"default", "w3", true}} {
		run, next := newItemRun(t), tc.next
		d := store.Dir{Root: run}
		openD1(t, run)
		if err := words.Append(d.Words(), words.Entry{ID: "w3", At: "2026-11-01T12:31:00Z", Text: "Keep v1 here."}); err != nil {
			t.Fatal(err)
		}
		if next {
			if _, e, code := runSource(t, &Ctx{Run: run}, "phase", "next", "--holder", "h1"); code != exit.OK {
				t.Fatalf("phase next: code %d, stderr %q", code, e)
			}
		}
		s0, err := state.Load(d)
		if err != nil || (s0.Phase == "design") != next {
			t.Fatalf("next %v: phase %q (err %v) before the rule", next, s0.Phase, err)
		}
		want, phase := []string{"RULING D1 " + tc.pick + " " + tc.wid}, s0.Phase
		if tc.pick == "other" && next {
			want, phase = append(want, fmt.Sprintf("PHASE %d research", s0.Moves+1)), "research"
		}
		if code, e := ruleD1(t, run, tc.pick, tc.wid, "h1", testNow); code != exit.OK || e != "" {
			t.Fatalf("%v: code %d, stderr %q; want 0", tc, code, e)
		}
		var got item.Item
		if err := item.Load(d, "D1", &got); err != nil || got.Status != "ruled" || got.Pick != tc.pick || got.Words != tc.wid {
			t.Fatalf("%v: D1 %+v (err %v); want ruled with the pick and the words id", tc, got, err)
		}
		s, serr := state.Load(d)
		lines, _, lerr := events.For(d, "dev", 0)
		logs := strings.Split(readOr(t, d.Log()), "\n")
		if serr != nil || lerr != nil || s.Phase != phase || s.Moves != s0.Moves+len(want)-1 || len(lines) < len(want) ||
			!reflect.DeepEqual(lines[len(lines)-len(want):], want) || len(logs) <= len(want) {
			t.Fatalf("%v: phase %q, moves %d, dev events %q (errs %v %v); want %s and %q last", tc, s.Phase, s.Moves, lines, serr, lerr, phase, want)
		}
		for i, w := range want {
			if l := logs[len(logs)-1-len(want)+i]; !strings.HasSuffix(l, "\t"+w) {
				t.Errorf("next %v: log.tsv row %q; want %q", next, l, w)
			}
		}
	}
}

// Each one-byte edit of default and other runs as item rule on one run, and gives 64 with the usage line exactly when
// the value is outside {default, other}. The first value, default itself, rules D1. A bad pick from the dev holder d1
// gives 64 too, since the pick check runs before the lease.
func TestItemRulePickSet(t *testing.T) {
	run := newItemRun(t)
	openD1(t, run)
	usage := "monet-source: usage: monet-source item rule <id> --pick default|other --words <words id> --holder <session>\n"
	for _, v := range append(oneByteEdits("default"), oneByteEdits("other")...) {
		code, e := ruleD1(t, run, v, "w2", "h1", testNow)
		if valid := v == "default" || v == "other"; (code == exit.Usage) == valid || (!valid && e != usage) {
			t.Fatalf("pick %q: code %d, stderr %q", v, code, e)
		}
	}
	var got item.Item
	if err := item.Load(store.Dir{Root: run}, "D1", &got); err != nil || got.Status != "ruled" || got.Pick != "default" {
		t.Fatalf("D1 %+v (err %v); want ruled with default", got, err)
	}
	if code, e := ruleD1(t, run, "defaulx", "w2", "d1", testNow); code != exit.Usage || e != usage {
		t.Fatalf("d1 with a bad pick: code %d, stderr %q; want %d", code, e, exit.Usage)
	}
}

// With the phase delivered, item rule gives 2, names /monet:source resume, and writes nothing.
func TestItemRuleAfterDelivery(t *testing.T) {
	run := newItemRun(t)
	openD1(t, run)
	if err := state.Update(store.Dir{Root: run}, func(s *state.State) error { s.Phase = phaseDelivered; return nil }); err != nil {
		t.Fatal(err)
	}
	before := runFiles(t, run)
	code, e := ruleD1(t, run, "other", "w2", "h1", testNow)
	if want := "monet-source: item rule: the run is delivered: reopen it with /monet:source resume\n"; code != exit.Refused || e != want || runFiles(t, run) != before {
		t.Fatalf("code %d, stderr %q; want %d, %q, and no write", code, e, exit.Refused, want)
	}
}
