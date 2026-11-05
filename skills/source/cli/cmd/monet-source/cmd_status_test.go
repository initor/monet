package main

import (
	"strings"
	"testing"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
)

// After init on a local clock with nanoseconds, status prints the 6 lines of plan A in UTC and takes no
// lease. At the brain expiry the lease prints free. An argument, a failed write, or a bad file is an error.
func TestStatusAfterInit(t *testing.T) {
	run := newRun(t, "demo")
	if _, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return testNow.Add(5).In(time.FixedZone("PST", -8*3600)) }}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK {
		t.Fatalf("init: code %d, stderr %q", code, e)
	}
	want := "run demo at " + run + "\nphase kickoff\ncharter not sealed\nlease brain h1 until 2026-11-02T00:00:00Z\nlease dev free\nevents 0, last none\n"
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
