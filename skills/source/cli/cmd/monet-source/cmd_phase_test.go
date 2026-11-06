package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

// acceptRound opens round n on the newest version, writes an accept for each persona, and closes it as ACCEPT.
func acceptRound(t *testing.T, run string, n int, personas ...string) {
	t.Helper()
	openRound(t, run)
	for _, p := range personas {
		writeVerdict(t, run, n, p, "accept")
	}
	closeRound(t, run, n, "ACCEPT")
}

// phaseFiles returns state.json, events.tsv, and log.tsv, the files that phase next writes.
func phaseFiles(t *testing.T, run string) string {
	t.Helper()
	d := store.Dir{Root: run}
	return readOr(t, d.State()) + "-- events.tsv\n" + readOr(t, d.Events()) + "-- log.tsv\n" + readOr(t, d.Log())
}

// An ACCEPT on research v1 moves the run to design: phase next renews brain, drops only the research counts,
// prints nothing, and records PHASE 1 design for both roles after the state mutex. An ACCEPT on design v1
// moves it to acceptance with PHASE 2 acceptance, and another phase next gives 2 and writes nothing.
func TestPhaseNext(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	submitFile(t, run, "a.md", "one\n")
	acceptRound(t, run, 1, "reviewer")
	if err := state.Update(d, func(s *state.State) error {
		s.Counts = map[string]map[string]int{"research": {"reviewer": 1}, "design": {"reviewer": 1}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	later, mutex, under := testNow.Add(time.Hour), d.Path("locks", "state.mutex"), false
	at := func() time.Time { _, err := os.Stat(mutex); under = err == nil; return later }
	out, e, code := runSource(t, &Ctx{Run: run, Now: at}, "phase", "next", "--holder", "h1")
	s, err := state.Load(d)
	if code != exit.OK || out != "" || e != "" || err != nil || s.Phase != "design" || s.Moves != 1 || under ||
		!reflect.DeepEqual(s.Counts, map[string]map[string]int{"design": {"reviewer": 1}}) || !strings.HasSuffix(readOr(t, d.Log()), "\n2026-11-01T13:00:00Z\tPHASE 1 design\n") {
		t.Fatalf("to design: code %d, stdout %q, stderr %q, phase %q, moves %d, counts %v (err %v), last Now under state.mutex %v, log %q",
			code, out, e, s.Phase, s.Moves, s.Counts, err, under, readOr(t, d.Log()))
	}
	checkLease(t, run, "brain", later.Add(leaseTTL))
	submitFile(t, run, "a.md", "two\n")
	acceptRound(t, run, 2, "reviewer", "reader")
	out, e, code = runSource(t, &Ctx{Run: run}, "phase", "next", "--holder", "h1")
	s, err = state.Load(d)
	brain, _, berr := events.For(d, "brain", 0)
	dev, _, derr := events.For(d, "dev", 0)
	if code != exit.OK || out != "" || e != "" || err != nil || s.Phase != phaseAcceptance || s.Moves != 2 || len(s.Counts) != 0 || berr != nil || derr != nil ||
		!reflect.DeepEqual(brain, []string{"VERSION research v1", "PHASE 1 design", "VERSION design v1", "PHASE 2 acceptance"}) ||
		!reflect.DeepEqual(dev, []string{"FINDINGS 1 ACCEPT", "PHASE 1 design", "FINDINGS 2 ACCEPT", "PHASE 2 acceptance"}) {
		t.Fatalf("to acceptance: code %d, stdout %q, stderr %q, phase %q, moves %d, counts %v (err %v), brain %q, dev %q (errs %v, %v)",
			code, out, e, s.Phase, s.Moves, s.Counts, err, brain, dev, berr, derr)
	}
	before := phaseFiles(t, run)
	if _, e, code = runSource(t, &Ctx{Run: run}, "phase", "next", "--holder", "h1"); code != exit.Refused ||
		e != "monet-source: phase next: phase acceptance is not a charter phase\n" || phaseFiles(t, run) != before {
		t.Fatalf("from acceptance: code %d, stderr %q, wrote %v; want 2 and no write", code, e, phaseFiles(t, run) != before)
	}
}

// Each row runs on a fresh run and writes nothing. A v2 after an ACCEPT on v1, a FIX round, and no version
// have no ACCEPT on the newest version. An open round, delivered, and kickoff (no phase yet) give 2 as well,
// another holder gives 4, and an argument gives 64. These last five rows hold an ACCEPT on research v1.
func TestPhaseNextNeedsAcceptOnNewest(t *testing.T) {
	accepted := func(run string) { submitFile(t, run, "a.md", "one\n"); acceptRound(t, run, 1, "reviewer") }
	phase := func(p string) func(string) {
		return func(run string) {
			accepted(run)
			if err := state.Update(store.Dir{Root: run}, func(s *state.State) error { s.Phase = p; return nil }); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		name  string
		setup func(run string)
		args  []string
		code  int
		e     string // stderr after "monet-source: "
	}{
		{"v2", func(run string) { accepted(run); submitFile(t, run, "a.md", "two\n") }, nil, exit.Refused, "phase next: v2 of research has no ACCEPT"},
		{"FIX", func(run string) {
			submitFile(t, run, "a.md", "one\n")
			openRound(t, run)
			writeVerdict(t, run, 1, "reviewer", "block", round.Finding{ID: "F1", Severity: "blocker"})
			closeRound(t, run, 1, "FIX")
		}, nil, exit.Refused, "phase next: v1 of research has no ACCEPT"},
		{"no version", func(string) {}, nil, exit.Refused, "phase next: v0 of research has no ACCEPT"},
		{"open round", func(run string) {
			accepted(run)
			if err := state.Update(store.Dir{Root: run}, func(s *state.State) error { s.RoundOpen = true; return nil }); err != nil {
				t.Fatal(err)
			}
		}, nil, exit.Refused, "phase next: round 1 is open"},
		{"delivered", phase(phaseDelivered), nil, exit.Refused, "phase next: phase delivered is not a charter phase"},
		{"kickoff", phase(""), nil, exit.Refused, "phase next: phase kickoff is not a charter phase"},
		{"other holder", accepted, []string{"--holder", "h2"}, exit.Held, "lock held: brain is not held by h2"},
		{"usage", accepted, []string{"x"}, exit.Usage, "usage: monet-source phase next --holder <session>"},
	} {
		run := newLoopRun(t)
		tc.setup(run)
		before := phaseFiles(t, run)
		out, e, code := runSource(t, &Ctx{Run: run}, append([]string{"phase", "next", "--holder", "h1"}, tc.args...)...)
		if code != tc.code || out != "" || e != "monet-source: "+tc.e+"\n" || phaseFiles(t, run) != before {
			t.Errorf("%s: code %d, stdout %q, stderr %q, wrote %v; want %d, %q", tc.name, code, out, e, phaseFiles(t, run) != before, tc.code, tc.e)
		}
	}
}

// Each row runs on a fresh run in design after one move, optionally set to acceptance or delivered. A charter
// phase before the current one, or any charter phase from acceptance or delivered, gives 0: brain renews, the
// run moves with PHASE 2 <phase>, and decisions.tsv gets one row by brain. Every other row writes nothing.
func TestPhaseBack(t *testing.T) {
	const reason, usage = "the claims need a source", "usage: monet-source phase back <phase> --reason <reason> --holder <session>"
	for _, tc := range []struct {
		from string // the phase set before phase back, or "" to stay in design
		args []string
		code int
		e    string // stderr after "monet-source: ", or the phase after a move
	}{
		{"", []string{"research", "--reason", reason}, exit.OK, "research"},
		{"", []string{"design", "--reason", reason}, exit.Refused, "phase back: design is not before design"},
		{"", []string{"implementation", "--reason", reason}, exit.Refused, "phase back: implementation is not before design"},
		{phaseAcceptance, []string{"design", "--reason", reason}, exit.OK, "design"},
		{phaseDelivered, []string{"design", "--reason", reason}, exit.OK, "design"},
		{"", []string{"research"}, exit.Usage, usage},
		{"", []string{"--reason", reason}, exit.Usage, usage},
		{"", []string{"research", "--reason", reason, "--holder", "h2"}, exit.Held, "lock held: brain is not held by h2"},
	} {
		run := newLoopRun(t)
		d := store.Dir{Root: run}
		submitFile(t, run, "a.md", "one\n")
		acceptRound(t, run, 1, "reviewer")
		if _, e, code := runSource(t, &Ctx{Run: run}, "phase", "next", "--holder", "h1"); code != exit.OK {
			t.Fatalf("phase next: code %d, stderr %q", code, e)
		}
		if err := state.Update(d, func(s *state.State) error { s.Phase = firstSet(tc.from, s.Phase); return nil }); err != nil {
			t.Fatal(err)
		}
		files := func() string { return phaseFiles(t, run) + "-- decisions.tsv\n" + readOr(t, d.Path("decisions.tsv")) }
		before, later := files(), testNow.Add(time.Hour)
		out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return later }}, append([]string{"phase", "back", "--holder", "h1"}, tc.args...)...)
		if tc.code != exit.OK {
			if code != tc.code || out != "" || e != "monet-source: "+tc.e+"\n" || files() != before {
				t.Errorf("%v from %q: code %d, stdout %q, stderr %q, wrote %v; want %d, %q", tc.args, tc.from, code, out, e, files() != before, tc.code, tc.e)
			}
			continue
		}
		s, err := state.Load(d)
		rows, rerr := store.ReadTSV(d.Path("decisions.tsv"), decisionHeader)
		want := [][]string{{"2026-11-01T13:00:00Z", "brain", "send back to " + tc.e, reason}}
		if code != exit.OK || out != "" || e != "" || err != nil || s.Phase != tc.e || s.Moves != 2 || rerr != nil || !reflect.DeepEqual(rows, want) ||
			!strings.HasSuffix(readOr(t, d.Log()), "\n2026-11-01T13:00:00Z\tPHASE 2 "+tc.e+"\n") {
			t.Errorf("%v from %q: code %d, stdout %q, stderr %q, phase %q, moves %d (err %v), decisions %q (err %v), log %q",
				tc.args, tc.from, code, out, e, s.Phase, s.Moves, err, rows, rerr, readOr(t, d.Log()))
		}
		checkLease(t, run, "brain", later.Add(leaseTTL))
	}
}
