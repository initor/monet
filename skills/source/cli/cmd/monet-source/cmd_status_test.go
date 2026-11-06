package main

import (
	"strings"
	"testing"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/item"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

// After init on a local clock with nanoseconds, status prints 8 lines in UTC (no round and nothing waiting) and takes
// no lease. At the brain expiry the lease prints free. An argument, a failed write, or a bad file is an error.
func TestStatusAfterInit(t *testing.T) {
	run := newRun(t, "demo")
	if _, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return testNow.Add(5).In(time.FixedZone("PST", -8*3600)) }}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK {
		t.Fatalf("init: code %d, stderr %q", code, e)
	}
	want := "run demo at " + run + "\nphase kickoff\nround none\nwaiting none\ncharter not sealed\nlease brain h1 until 2026-11-02T00:00:00Z\nlease dev free\nevents 0, last none\n"
	if out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return testNow.Add(time.Hour) }}, "status"); code != exit.OK || out != want || e != "" || listRun(t, store.Dir{Root: run}.Locks()) != "brain.json" {
		t.Fatalf("code %d, stderr %q, stdout %q, want %q, locks %q", code, e, out, want, listRun(t, store.Dir{Root: run}.Locks()))
	}
	checkLease(t, run, "brain", testNow.Add(12*time.Hour+5))
	if out, _, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return testNow.Add(12*time.Hour + 5) }}, "status"); code != exit.OK || !strings.Contains(out, "\nlease brain free\nlease dev free\n") {
		t.Errorf("expired: code %d, stdout %q", code, out)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "status", "x"); code != exit.Usage || e != "monet-source: usage: monet-source status\n" {
		t.Errorf("arg: code %d, stderr %q", code, e)
	}
	if err := cmdStatus(&Ctx{Run: run, Stdout: &opWriter{failAt: 1}, Now: time.Now}, nil); err == nil || err.Error() != "reader gone" {
		t.Errorf("failed write: err %v", err)
	}
	for _, f := range []string{"events.tsv", "locks/dev.json", "state.json"} { // each file is read before the ones already bad
		err := store.AppendLine(store.Dir{Root: run}.Path(f), "bad")
		if out, e, code := runSource(t, &Ctx{Run: run}, "status"); err != nil || code != exit.User || out != "" || !strings.Contains(e, f) {
			t.Errorf("bad %s: append %v, code %d, stdout %q, stderr %q", f, err, code, out, e)
		}
	}
}

// After 3 events and a phase, status prints at most 8 lines, the phase, and
// the event count with the last line.
func TestStatusAtMostEightLines(t *testing.T) {
	run := newRun(t, "demo")
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK {
		t.Fatalf("init: code %d, stderr %q", code, e)
	}
	for _, line := range []string{"PHASE design", "WALL w1", "ITEM i3 ready"} {
		events.Emit(store.Dir{Root: run}, line, testNow) // a failed emit fails the events line below
	}
	state.Update(store.Dir{Root: run}, func(s *state.State) error { s.Phase = "design"; return nil }) // a failure fails the phase line
	if out, e, code := runSource(t, &Ctx{Run: run}, "status"); code != exit.OK || e != "" || strings.Count(out, "\n") > 8 || !strings.Contains(out, "\nphase design\n") || !strings.HasSuffix(out, "\nevents 3, last ITEM i3 ready\n") {
		t.Fatalf("code %d, stderr %q, stdout %q", code, e, out)
	}
}

// After the phase line, status prints the newest round with its state, its own phase and version (or final), and
// the counts of the round phase sorted by persona (or counts none), then the items and walls that wait (or
// none), so it prints 8 lines with the events line last. A bad round.json or a bad wall file is an error.
func TestStatusRoundAndWaiting(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	check := func(step, want string) {
		t.Helper()
		out, e, code := runSource(t, &Ctx{Run: run}, "status")
		lines := strings.Split(out, "\n")
		if code != exit.OK || e != "" || len(lines) != 9 || strings.Join(lines[1:4], "\n") != want || !strings.HasPrefix(lines[7], "events ") || lines[8] != "" {
			t.Fatalf("%s: code %d, stderr %q, stdout %q; want lines 2 to 4 %q", step, code, e, out, want)
		}
	}
	submitFile(t, run, "a.md", "one\n")
	check("no round", "phase research\nround none\nwaiting none")
	openRound(t, run)
	check("open", "phase research\nround 1 open on research v1, counts none\nwaiting none")
	writeVerdict(t, run, 1, "reviewer", "accept")
	closeRound(t, run, 1, "ACCEPT")
	check("accept", "phase research\nround 1 ACCEPT on research v1, counts none\nwaiting none")
	submitFile(t, run, "a.md", "two\n")
	openRound(t, run)
	writeVerdict(t, run, 2, "reviewer", "block", round.Finding{ID: "F1", Severity: "blocker"})
	closeRound(t, run, 2, "FIX")
	if err := item.Save(d, "D1", item.Item{ID: "D1", Phase: "research", Persona: "reviewer", Round: 2, Status: "draft"}); err != nil {
		t.Fatal(err)
	}
	check("draft", "phase research\nround 2 FIX on research v2, counts reviewer=1\nwaiting 1 items, 0 walls: D1")
	if _, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(`{"question": "q", "default": "d", "blocks": "design"}`)}, "wall", "open", "--holder", "h1"); code != exit.OK {
		t.Fatalf("wall open: code %d, stderr %q", code, e)
	}
	check("fix", "phase research\nround 2 FIX on research v2, counts reviewer=1\nwaiting 1 items, 1 walls: D1 W1")
	r, err := round.Load(run, 2)
	if r.Final = true; err != nil || round.Save(run, r) != nil {
		t.Fatalf("mark round 2 final: load err %v, or the save failed", err)
	}
	if err := state.Update(d, func(s *state.State) error {
		s.Phase, s.Counts["design"] = "design", map[string]int{"reader": 5}
		s.Counts["research"]["writer"], s.Counts["research"]["critic"], s.Counts["research"]["author"] = 3, 2, 1
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	check("final", "phase design\nround 2 FIX on final, counts author=1,critic=2,reviewer=1,writer=3\nwaiting 1 items, 1 walls: D1 W1")
	for _, f := range []string{"items/W1.json", "rounds/2/round.json"} { // the round is read before the walls
		err := store.AppendLine(d.Path(f), "bad")
		if out, e, code := runSource(t, &Ctx{Run: run}, "status"); err != nil || code != exit.User || out != "" || !strings.Contains(e, f) {
			t.Errorf("bad %s: append %v, code %d, stdout %q, stderr %q", f, err, code, out, e)
		}
	}
	run = newLoopRun(t) // check reads this run: round 1 is on v2, and phase next moves the run past its phase
	submitFile(t, run, "a.md", "one\n")
	submitFile(t, run, "a.md", "two\n")
	openRound(t, run)
	writeVerdict(t, run, 1, "reviewer", "accept")
	closeRound(t, run, 1, "ACCEPT")
	if _, e, code := runSource(t, &Ctx{Run: run}, "phase", "next", "--holder", "h1"); code != exit.OK {
		t.Fatalf("phase next: code %d, stderr %q", code, e)
	}
	check("phase next", "phase design\nround 1 ACCEPT on research v2, counts none\nwaiting none")
}
