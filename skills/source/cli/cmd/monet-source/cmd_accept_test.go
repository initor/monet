package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"source/internal/charter"
	"source/internal/exit"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

// newAcceptanceRun is a newLoopRun with an ACCEPT round on v1 of research and of design, each
// followed by phase next, so the run is in acceptance after round 2.
func newAcceptanceRun(t *testing.T) string {
	t.Helper()
	run := newLoopRun(t)
	for i, personas := range [][]string{{"reviewer"}, {"reviewer", "reader"}} {
		submitFile(t, run, "a.md", "one\n")
		acceptRound(t, run, i+1, personas...)
		if _, e, code := runSource(t, &Ctx{Run: run}, "phase", "next", "--holder", "h1"); code != exit.OK {
			t.Fatalf("phase next %d: code %d, stderr %q", i+1, code, e)
		}
	}
	return run
}

// accept opens final round 3 on design with the blocking reviewer only, at max effort, on the accepted
// v1 of research and of design, and leaves Checked as it was. In design, a second accept, another
// holder, and an argument each give a refusal that writes nothing.
func TestAccept(t *testing.T) {
	run := newAcceptanceRun(t)
	d := store.Dir{Root: run}
	before, err := state.Load(d)
	if err != nil || before.Phase != phaseAcceptance {
		t.Fatalf("newAcceptanceRun: phase %q, err %v", before.Phase, err)
	}
	out, e, code := runSource(t, &Ctx{Run: run}, "accept", "--holder", "h1")
	brief, verdict := round.Path(run, 3, "briefs", "3-reviewer.md"), round.Path(run, 3, "verdicts", "3-reviewer.json")
	if want := "check 3-reviewer reviewer max " + brief + " " + verdict + "\n"; code != exit.OK || out != want || e != "" {
		t.Fatalf("accept: code %d, stdout %q, stderr %q; want 0 and %q", code, out, e, want)
	}
	versions := []string{filepath.Join(run, "artifacts", "research", "v1"), filepath.Join(run, "artifacts", "design", "v1")}
	text := readOr(t, brief)
	if want := round.Brief(round.BriefInput{Check: "3-reviewer", Persona: "reviewer", Effort: "max", Charter: readOr(t, d.CharterMD()),
		PersonaBrief: readOr(t, filepath.Join(d.Personas(), "reviewer.md")), Verdict: verdict, Sources: []string{"skills/source/cli"}, Versions: versions}); text != want {
		t.Fatalf("brief\n%s\nwant\n%s", text, want)
	}
	r, err := round.Load(run, 3)
	wantR := round.Round{N: 3, Phase: "design", Final: true, Version: 1, Versions: versions, Checks: []round.Check{{ID: "3-reviewer", Persona: "reviewer",
		Weight: "blocking", Effort: "max", Brief: brief, BriefHash: charter.Hash([]byte(text)), Verdict: verdict}}}
	if err != nil || !reflect.DeepEqual(r, wantR) {
		t.Fatalf("round.json %+v, err %v; want %+v", r, err, wantR)
	}
	s, err := state.Load(d)
	if err != nil || s.Round != 3 || !s.RoundOpen || s.Phase != phaseAcceptance || !reflect.DeepEqual(s.Checked, before.Checked) {
		t.Fatalf("state round %d, open %v, phase %q, checked %v (err %v); want 3, true, acceptance, %v", s.Round, s.RoundOpen, s.Phase, s.Checked, err, before.Checked)
	}
	newer := newAcceptanceRun(t) // Accepted, not Versions (1 for both phases), names each folder
	if err := state.Update(store.Dir{Root: newer}, func(s *state.State) error {
		s.Accepted, s.Checked = map[string]int{"research": 2, "design": 3}, map[string]int{"research": 1, "design": 3}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	runSource(t, &Ctx{Run: newer}, "accept", "--holder", "h1")
	r, err = round.Load(newer, 3)
	if want := []string{filepath.Join(newer, "artifacts", "research", "v2"), filepath.Join(newer, "artifacts", "design", "v3")}; err != nil || r.Version != 3 || !reflect.DeepEqual(r.Versions, want) {
		t.Fatalf("accepted research v2 and design v3: version %d, versions %v (err %v); want 3 and %v", r.Version, r.Versions, err, want)
	}
	picked := newAcceptanceRun(t) // a blocking persona with no phases, picked in design, gets a final check
	writeCharter(t, picked, `"phases": ["design"]}]`, `"phases": ["design"]}, {"id": "skim", "description": "Skims the page.", "weight": "blocking", "sources": [], "phases": []}]`)
	setBrief(t, picked, "skim", "# skim\nSkim the page.\n")
	if err := state.Update(store.Dir{Root: picked}, func(s *state.State) error { s.Picks = map[string][]string{"design": {"skim"}}; return nil }); err != nil {
		t.Fatal(err)
	}
	line := func(id string) string {
		return "check 3-" + id + " " + id + " max " + round.Path(picked, 3, "briefs", "3-"+id+".md") + " " + round.Path(picked, 3, "verdicts", "3-"+id+".json") + "\n"
	}
	out, e, code = runSource(t, &Ctx{Run: picked}, "accept", "--holder", "h1")
	if r, err = round.Load(picked, 3); code != exit.OK || out != line("reviewer")+line("skim") || err != nil || len(r.Checks) != 2 || r.Checks[1].Persona != "skim" {
		t.Fatalf("picked skim: code %d, stdout %q, stderr %q, checks %+v (err %v)", code, out, e, r.Checks, err)
	}
	design := newLoopRun(t)
	if err := state.Update(store.Dir{Root: design}, func(s *state.State) error { s.Phase = "design"; return nil }); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		run  string
		args []string
		code int
		e    string // stderr after "monet-source: "
	}{
		{design, nil, exit.Refused, "accept: phase design is not acceptance"},
		{run, nil, exit.Refused, "accept: round 3 is open"},
		{run, []string{"--holder", "h2"}, exit.Held, "lock held: brain is not held by h2"},
		{run, []string{"x"}, exit.Usage, "usage: monet-source accept --holder <session>"},
	} {
		sd := store.Dir{Root: tc.run}
		st, files := readOr(t, sd.State()), roundFiles(t, tc.run)
		out, e, code := runSource(t, &Ctx{Run: tc.run}, append([]string{"accept", "--holder", "h1"}, tc.args...)...)
		if code != tc.code || out != "" || e != "monet-source: "+tc.e+"\n" || readOr(t, sd.State()) != st || roundFiles(t, tc.run) != files {
			t.Errorf("%q: code %d, stdout %q, stderr %q, wrote %v; want %d, %q", tc.e, code, out, e, readOr(t, sd.State()) != st || roundFiles(t, tc.run) != files, tc.code, tc.e)
		}
	}
}
