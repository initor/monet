package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"source/internal/charter"
	"source/internal/events"
	"source/internal/exit"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

// writeVerdict writes a valid verdict file for the check <n>-<persona> of round n.
func writeVerdict(t *testing.T, run string, n int, persona, verdict string, findings ...round.Finding) {
	t.Helper()
	id := fmt.Sprintf("%d-%s", n, persona)
	p := round.Path(run, n, "verdicts", id+".json")
	if err := store.WriteJSON(p, round.Verdict{Persona: persona, Check: id, Verdict: verdict, Findings: findings}); err != nil {
		t.Fatal(err)
	}
	if _, err := round.ParseVerdict(p, id, persona); err != nil {
		t.Fatalf("writeVerdict %s: %v", id, err)
	}
}

// roundFiles lists every file under <run>/rounds with its content, so a test can tell a write.
func roundFiles(t *testing.T, run string) string {
	t.Helper()
	var b strings.Builder
	filepath.WalkDir(filepath.Join(run, "rounds"), func(p string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() {
			b.WriteString(p + "\n" + readOr(t, p))
		}
		return nil
	})
	return b.String()
}

// round open on v1 of research writes the brief of its one check, round.json with the brief hash,
// and the state of an open round 1, and prints the check line (R13, R15).
func TestRoundOpenWritesChecks(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	submitFile(t, run, "a.md", "one\n")
	out, e, code := runSource(t, &Ctx{Run: run}, "round", "open", "--holder", "h1")
	brief, verdict, v1 := round.Path(run, 1, "briefs", "1-reviewer.md"), round.Path(run, 1, "verdicts", "1-reviewer.json"), filepath.Join(run, "artifacts", "research", "v1")
	if want := "check 1-reviewer reviewer medium " + brief + " " + verdict + "\n"; code != exit.OK || out != want || e != "" {
		t.Fatalf("round open: code %d, stdout %q, stderr %q; want 0 and %q", code, out, e, want)
	}
	text, md, persona := readOr(t, brief), readOr(t, d.CharterMD()), readOr(t, filepath.Join(d.Personas(), "reviewer.md"))
	want := round.Brief(round.BriefInput{Check: "1-reviewer", Persona: "reviewer", Effort: "medium", Charter: md, PersonaBrief: persona,
		Verdict: verdict, Sources: []string{"skills/source/cli"}, Versions: []string{v1}})
	if text != want || !strings.Contains(text, md) || !strings.Contains(text, persona) {
		t.Fatalf("brief\n%s\nwant\n%s", text, want)
	}
	r, err := round.Load(run, 1)
	wantR := round.Round{N: 1, Phase: "research", Version: 1, Versions: []string{v1}, Checks: []round.Check{{ID: "1-reviewer", Persona: "reviewer",
		Weight: "blocking", Effort: "medium", Brief: brief, BriefHash: charter.Hash([]byte(text)), Verdict: verdict}}}
	if err != nil || !reflect.DeepEqual(r, wantR) {
		t.Fatalf("round.json %+v, err %v; want %+v", r, err, wantR)
	}
	s, err := state.Load(d)
	if err != nil || s.Round != 1 || !s.RoundOpen || !reflect.DeepEqual(s.Checked, map[string]int{"research": 1}) || s.Picks != nil {
		t.Fatalf("state round %d, open %v, checked %v, picks %v, err %v; want 1, true, research 1, nil", s.Round, s.RoundOpen, s.Checked, s.Picks, err)
	}
	writeVerdict(t, run, 1, "reviewer", "accept")
	if _, err := round.ParseVerdict(r.Checks[0].Verdict, "1-reviewer", "reviewer"); err != nil {
		t.Fatalf("the verdict of writeVerdict at the stored path: %v", err)
	}
}

// Each row runs on a fresh run. No refusal writes state.json or a file under rounds/.
func TestRoundOpenRefuses(t *testing.T) {
	open := func(run string) {
		if _, e, code := runSource(t, &Ctx{Run: run}, "round", "open", "--holder", "h1"); code != exit.OK {
			t.Fatalf("first round open: code %d, stderr %q", code, e)
		}
	}
	for _, tc := range []struct {
		name  string
		setup func(run string)
		args  []string
		code  int
		e     string // stderr after "monet-source: "
	}{
		{"no version", func(string) {}, nil, exit.Refused, "round open: research has no version"},
		{"open round", func(run string) { submitFile(t, run, "a.md", "one\n"); open(run); submitFile(t, run, "a.md", "two\n") }, nil, exit.Refused, "round open: round 1 is open"},
		{"checked version", func(run string) {
			submitFile(t, run, "a.md", "one\n")
			open(run)
			if err := state.Update(store.Dir{Root: run}, func(s *state.State) error { s.RoundOpen = false; return nil }); err != nil {
				t.Fatal(err)
			}
		}, nil, exit.Refused, "round open: v1 of research was already checked"},
		{"no charter phase", func(run string) {
			submitFile(t, run, "a.md", "one\n")
			if err := state.Update(store.Dir{Root: run}, func(s *state.State) error { s.Phase = "acceptance"; return nil }); err != nil {
				t.Fatal(err)
			}
		}, nil, exit.Refused, "round open: phase acceptance takes no round"},
		{"no brief", func(run string) {
			submitFile(t, run, "a.md", "one\n")
			if err := state.Update(store.Dir{Root: run}, func(s *state.State) error { delete(s.Briefs, "reviewer"); return nil }); err != nil || os.Remove(filepath.Join(run, "personas", "reviewer.md")) != nil {
				t.Fatalf("drop the reviewer brief: %v", err)
			}
		}, nil, exit.Refused, "persona reviewer has no brief"},
		{"bad pick", func(run string) { submitFile(t, run, "a.md", "one\n") }, []string{"--pick", "reader"}, exit.Refused, "round open: pick reader: not a persona with no phases"},
		{"other holder", func(run string) { submitFile(t, run, "a.md", "one\n") }, []string{"--holder", "h2"}, exit.Held, "lock held: brain is not held by h2"},
		{"usage", func(run string) { submitFile(t, run, "a.md", "one\n") }, []string{"x"}, exit.Usage, "usage: monet-source round open --holder <session> [--pick <id,...>]"},
	} {
		run := newLoopRun(t)
		tc.setup(run)
		d := store.Dir{Root: run}
		before, files := readOr(t, d.State()), roundFiles(t, run)
		args := append([]string{"round", "open", "--holder", "h1"}, tc.args...)
		_, e, code := runSource(t, &Ctx{Run: run}, args...)
		if code != tc.code || e != "monet-source: "+tc.e+"\n" || readOr(t, d.State()) != before || roundFiles(t, run) != files {
			t.Errorf("%s: code %d, stderr %q, state or rounds changed %v; want %d, %q", tc.name, code, e, readOr(t, d.State()) != before || roundFiles(t, run) != files, tc.code, tc.e)
		}
	}
}

// The first round of a phase fixes the picks of its comma separated --pick (R12), and the next round
// on a new version checks the same panel as round 2, whatever its --pick, at max effort for a persona with one block.
func TestRoundOpenKeepsPanel(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	writeCharter(t, run, `"phases": ["design"]}]`, `"phases": []}, {"id": "skim", "description": "Skims the page.", "weight": "advisory", "sources": [], "phases": []}]`)
	setBrief(t, run, "skim", "# skim\nSkim the page.\n") // reader and skim have no phases, so they are picked
	submitFile(t, run, "a.md", "one\n")
	line := func(n int, id, effort string) string {
		c := fmt.Sprintf("%d-%s", n, id)
		return fmt.Sprintf("check %s %s %s %s %s\n", c, id, effort, round.Path(run, n, "briefs", c+".md"), round.Path(run, n, "verdicts", c+".json"))
	}
	if out, e, code := runSource(t, &Ctx{Run: run}, "round", "open", "--holder", "h1", "--pick", "reader,skim"); code != exit.OK || out != line(1, "reviewer", "medium")+line(1, "reader", "medium")+line(1, "skim", "medium") {
		t.Fatalf("round 1: code %d, stdout %q, stderr %q", code, out, e)
	}
	if err := state.Update(d, func(s *state.State) error {
		s.RoundOpen, s.Counts = false, map[string]map[string]int{"research": {"reviewer": 1}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	submitFile(t, run, "a.md", "two\n")
	if out, e, code := runSource(t, &Ctx{Run: run}, "round", "open", "--holder", "h1", "--pick", "nobody"); code != exit.OK || out != line(2, "reviewer", "max")+line(2, "reader", "medium")+line(2, "skim", "medium") {
		t.Fatalf("round 2: code %d, stdout %q, stderr %q", code, out, e)
	}
	r, err := round.Load(run, 2)
	s, serr := state.Load(d)
	if err != nil || serr != nil || r.Version != 2 || !reflect.DeepEqual(r.Versions, []string{filepath.Join(run, "artifacts", "research", "v2")}) ||
		s.Round != 2 || !s.RoundOpen || !reflect.DeepEqual(s.Checked, map[string]int{"research": 2}) || !reflect.DeepEqual(s.Picks, map[string][]string{"research": {"reader", "skim"}}) {
		t.Fatalf("round 2 %+v (err %v), state round %d, open %v, checked %v, picks %v (err %v)", r, err, s.Round, s.RoundOpen, s.Checked, s.Picks, serr)
	}
}

// A design round checks design v1 with the design panel: a brief per check at its own effort, each check stored.
func TestRoundOpenDesign(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	if err := state.Update(d, func(s *state.State) error {
		s.Phase, s.Counts = "design", map[string]map[string]int{"design": {"reviewer": 1}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	submitFile(t, run, "a.md", "one\n")
	out, e, code := runSource(t, &Ctx{Run: run}, "round", "open", "--holder", "h1")
	want, lines := round.Round{N: 1, Phase: "design", Version: 1, Versions: []string{filepath.Join(run, "artifacts", "design", "v1")}}, ""
	for _, p := range []struct{ id, weight, effort, src string }{{"reviewer", "blocking", "max", "skills/source/cli"}, {"reader", "advisory", "medium", ""}} {
		ck := round.Check{ID: "1-" + p.id, Persona: p.id, Weight: p.weight, Effort: p.effort, Brief: round.Path(run, 1, "briefs", "1-"+p.id+".md"), Verdict: round.Path(run, 1, "verdicts", "1-"+p.id+".json")}
		brief := round.Brief(round.BriefInput{Check: ck.ID, Persona: p.id, Effort: p.effort, Charter: readOr(t, d.CharterMD()),
			PersonaBrief: readOr(t, filepath.Join(d.Personas(), p.id+".md")), Verdict: ck.Verdict, Sources: strings.Fields(p.src), Versions: want.Versions})
		if got := readOr(t, ck.Brief); got != brief {
			t.Errorf("brief %s\n%s\nwant\n%s", ck.ID, got, brief)
		}
		ck.BriefHash = charter.Hash([]byte(brief))
		want.Checks, lines = append(want.Checks, ck), lines+fmt.Sprintf("check %s %s %s %s %s\n", ck.ID, p.id, p.effort, ck.Brief, ck.Verdict)
	}
	r, err := round.Load(run, 1)
	s, serr := state.Load(d)
	if code != exit.OK || out != lines || e != "" || err != nil || !reflect.DeepEqual(r, want) || serr != nil || !reflect.DeepEqual(s.Checked, map[string]int{"design": 1}) {
		t.Fatalf("code %d, stdout %q, stderr %q, round.json %+v (err %v), checked %v (err %v); want lines %q, round %+v", code, out, e, r, err, s.Checked, serr, lines, want)
	}
}

// openRound opens a round on the newest version as h1 and needs exit 0.
func openRound(t *testing.T, run string) {
	t.Helper()
	if _, e, code := runSource(t, &Ctx{Run: run}, "round", "open", "--holder", "h1"); code != exit.OK {
		t.Fatalf("round open: code %d, stderr %q", code, e)
	}
}

// closeRound closes round n as h1, needs exit 0, outcome on stdout and in round.json, and a closed
// round, and returns the state, round.json, and findings.md after the close.
func closeRound(t *testing.T, run string, n int, outcome string) (state.State, round.Round, string) {
	t.Helper()
	out, e, code := runSource(t, &Ctx{Run: run}, "round", "close", "--holder", "h1")
	s, err := state.Load(store.Dir{Root: run})
	r, rerr := round.Load(run, n)
	if code != exit.OK || out != outcome+"\n" || e != "" || err != nil || rerr != nil || s.RoundOpen || r.Outcome != outcome {
		t.Fatalf("round close %d: code %d, stdout %q, stderr %q, open %v, outcome %q (errs %v, %v); want 0 and %s", n, code, out, e, s.RoundOpen, r.Outcome, err, rerr, outcome)
	}
	return s, r, readOr(t, round.Path(run, n, "findings.md"))
}

// An accept gives ACCEPT, sets Accepted to the round version (not the newer v2), counts nothing, and
// records FINDINGS for dev. A final round that accepts leaves Accepted as it was.
func TestRoundCloseAccept(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	submitFile(t, run, "a.md", "one\n")
	openRound(t, run)
	submitFile(t, run, "a.md", "two\n")
	writeVerdict(t, run, 1, "reviewer", "accept")
	s, r, md := closeRound(t, run, 1, "ACCEPT")
	lines, _, err := events.For(d, "dev", 0)
	if r.Checks[0].Result != "accept" || md != "# Round 1 findings\n\n## reviewer (blocking): accept\n" || !reflect.DeepEqual(s.Accepted, map[string]int{"research": 1}) ||
		len(s.Counts) != 0 || err != nil || !reflect.DeepEqual(lines, []string{"FINDINGS 1 ACCEPT"}) || !strings.HasSuffix(readOr(t, d.Log()), "\tFINDINGS 1 ACCEPT\n") {
		t.Fatalf("result %q, findings.md %q, accepted %v, counts %v, dev events %q (err %v)", r.Checks[0].Result, md, s.Accepted, s.Counts, lines, err)
	}
	openRound(t, run)
	r, err = round.Load(run, 2)
	if r.Final = true; err != nil || round.Save(run, r) != nil {
		t.Fatalf("mark round 2 final: load err %v, or the save failed", err)
	}
	writeVerdict(t, run, 2, "reviewer", "accept")
	if s, _, _ = closeRound(t, run, 2, "ACCEPT"); !reflect.DeepEqual(s.Accepted, map[string]int{"research": 1}) {
		t.Fatalf("accepted after a final round %v; want research 1", s.Accepted)
	}
}

// A block with one blocker gives FIX, counts 1 for the blocking persona of the round phase, sets no
// Accepted, and copies the finding byte for byte. A block of the advisory reader counts nothing.
func TestRoundCloseBlockCounts(t *testing.T) {
	run := newLoopRun(t)
	f := round.Finding{ID: "F1", Severity: "blocker", Problem: "The page says `fsync`.\n\n- It never runs.", Evidence: "store.go:44", Ask: "Cite the *rename*."}
	sec := "## reviewer (blocking): block\n\n### F1 (blocker)\nProblem: The page says `fsync`.\n\n- It never runs.\nEvidence: store.go:44\nAsk: Cite the *rename*.\n"
	submitFile(t, run, "a.md", "one\n")
	openRound(t, run)
	writeVerdict(t, run, 1, "reviewer", "block", f)
	s, r, md := closeRound(t, run, 1, "FIX")
	if r.Checks[0].Result != "block" || md != "# Round 1 findings\n\n"+sec || !reflect.DeepEqual(s.Counts, map[string]map[string]int{"research": {"reviewer": 1}}) || s.Accepted != nil {
		t.Fatalf("result %q, findings.md %q, counts %v, accepted %v", r.Checks[0].Result, md, s.Counts, s.Accepted)
	}
	if err := state.Update(store.Dir{Root: run}, func(s *state.State) error { s.Phase = "design"; return nil }); err != nil {
		t.Fatal(err)
	}
	submitFile(t, run, "a.md", "two\n")
	openRound(t, run)
	writeVerdict(t, run, 2, "reviewer", "block", f)
	writeVerdict(t, run, 2, "reader", "block", f)
	s, _, md = closeRound(t, run, 2, "FIX")
	if want := map[string]map[string]int{"research": {"reviewer": 1}, "design": {"reviewer": 1}}; !reflect.DeepEqual(s.Counts, want) ||
		md != "# Round 2 findings\n\n"+sec+"\n"+strings.Replace(sec, "reviewer (blocking)", "reader (advisory)", 1) {
		t.Fatalf("design: counts %v, findings.md %q; want %v", s.Counts, md, want)
	}
}

// A missing verdict file gives the result missing, and a block verdict of check 9-reviewer at the
// path of 1-reviewer is invalid (R18). Each gives FIX with no count and no Accepted.
func TestRoundCloseMissingVerdictIsFix(t *testing.T) {
	for _, tc := range []struct {
		write  bool
		result string
	}{{false, "missing"}, {true, "invalid: check 9-reviewer: want 1-reviewer"}} {
		run := newLoopRun(t)
		submitFile(t, run, "a.md", "one\n")
		openRound(t, run)
		v := round.Verdict{Persona: "reviewer", Check: "9-reviewer", Verdict: "block", Findings: []round.Finding{{ID: "F1", Severity: "blocker"}}}
		if tc.write && store.WriteJSON(round.Path(run, 1, "verdicts", "1-reviewer.json"), v) != nil {
			t.Fatal("write the verdict of 9-reviewer")
		}
		s, r, md := closeRound(t, run, 1, "FIX")
		if res := r.Checks[0].Result; res != tc.result || md != "# Round 1 findings\n\n## reviewer (blocking): "+res+"\n" || len(s.Counts) != 0 || s.Accepted != nil {
			t.Errorf("result %q, findings.md %q, counts %v, accepted %v; want %q", res, md, s.Counts, s.Accepted, tc.result)
		}
	}
}

// round close checks usage, then renews brain, then needs an open round: none yet, or one that is
// closed, gives 2. No refusal writes state.json, a file under rounds/, or an event.
func TestRoundCloseNeedsOpenRound(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	submitFile(t, run, "a.md", "one\n")
	closed := func() {
		openRound(t, run)
		writeVerdict(t, run, 1, "reviewer", "accept")
		closeRound(t, run, 1, "ACCEPT")
	}
	for i, tc := range []struct {
		setup func()
		args  []string
		code  int
		e     string // stderr after "monet-source: "
	}{
		{nil, []string{"--holder", "h1"}, exit.Refused, "round close: no round is open"},
		{nil, []string{"--holder", "h2"}, exit.Held, "lock held: brain is not held by h2"},
		{nil, []string{"x", "--holder", "h1"}, exit.Usage, "usage: monet-source round close --holder <session>"},
		{closed, []string{"--holder", "h1"}, exit.Refused, "round close: no round is open"},
	} {
		if tc.setup != nil {
			tc.setup()
		}
		before, files, evs := readOr(t, d.State()), roundFiles(t, run), readOr(t, d.Events())
		_, e, code := runSource(t, &Ctx{Run: run}, append([]string{"round", "close"}, tc.args...)...)
		if code != tc.code || e != "monet-source: "+tc.e+"\n" || readOr(t, d.State()) != before || roundFiles(t, run) != files || readOr(t, d.Events()) != evs {
			t.Errorf("row %d: code %d, stderr %q, or a write; want %d, %q", i, code, e, tc.code, tc.e)
		}
	}
}

// Each block adds 1 to the count of its persona, and an ACCEPT keeps the Accepted entries of the
// other phases. Both key by the round phase, also when the state phase moves while the round is open.
// FINDINGS names the round number (round 1 checks v2) and the outcome of each round.
func TestRoundCloseAcrossRounds(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	phase := func(p string) {
		if err := state.Update(d, func(s *state.State) error { s.Phase = p; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	findings := map[string][]round.Finding{"block": {{ID: "F1", Severity: "blocker"}}}
	submitFile(t, run, "a.md", "one\n")
	for i, tc := range [][2]string{{"block", "FIX"}, {"block", "FIX"}, {"accept", "ACCEPT"}} {
		phase("research")
		submitFile(t, run, "a.md", fmt.Sprintf("research %d\n", i))
		openRound(t, run)
		writeVerdict(t, run, i+1, "reviewer", tc[0], findings[tc[0]]...)
		phase("design")
		closeRound(t, run, i+1, tc[1])
	}
	submitFile(t, run, "a.md", "design\n")
	openRound(t, run)
	writeVerdict(t, run, 4, "reviewer", "accept")
	writeVerdict(t, run, 4, "reader", "accept")
	s, _, _ := closeRound(t, run, 4, "ACCEPT")
	lines, _, err := events.For(d, "dev", 0)
	want := []string{"FINDINGS 1 FIX", "FINDINGS 2 FIX", "FINDINGS 3 ACCEPT", "FINDINGS 4 ACCEPT"}
	if !reflect.DeepEqual(s.Counts, map[string]map[string]int{"research": {"reviewer": 2}}) ||
		!reflect.DeepEqual(s.Accepted, map[string]int{"research": 4, "design": 1}) || err != nil || !reflect.DeepEqual(lines, want) {
		t.Fatalf("counts %v, accepted %v, dev events %q (err %v); want research reviewer 2, research 4 and design 1, %q", s.Counts, s.Accepted, lines, err, want)
	}
}
