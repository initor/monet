package main

import (
	"reflect"
	"strings"
	"testing"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/item"
	"source/internal/round"
	"source/internal/store"
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
