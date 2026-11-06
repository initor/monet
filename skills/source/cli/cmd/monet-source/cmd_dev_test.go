package main

import (
	"strings"
	"testing"
	"time"

	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
)

// devStep is one dev call at testNow+at and the lease, Dev, and LastBeat (-1 for none) after it.
type devStep struct {
	args, stderr, dev string
	code              int
	at, beat          time.Duration
	lease             time.Time
}

// runDev runs steps in order on one run made by init. No step prints to stdout.
func runDev(t *testing.T, steps []devStep) {
	run := newRun(t, "demo")
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK {
		t.Fatalf("init: code %d, stderr %q", code, e)
	}
	for _, st := range steps {
		out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return testNow.Add(st.at) }}, strings.Fields("dev "+st.args)...)
		if code != st.code || out != "" || (st.stderr == "") != (e == "") || !strings.Contains(e, st.stderr) {
			t.Errorf("%q: code %d, stdout %q, stderr %q, want %d with %q", st.args, code, out, e, st.code, st.stderr)
		}
		checkLease(t, run, "dev", st.lease)
		s, err := state.Load(store.Dir{Root: run})
		if err != nil || s.Dev != st.dev || (s.LastBeat == nil) != (st.beat < 0) || (s.LastBeat != nil && !s.LastBeat.Equal(testNow.Add(st.beat))) {
			t.Errorf("%q: dev %q, last beat %v, err %v; want %q at +%v", st.args, s.Dev, s.LastBeat, err, st.dev, st.beat)
		}
	}
}

// hello takes the dev lease. Another live holder is exit 4, a bad call exit 64.
func TestDevHello(t *testing.T) {
	runDev(t, []devStep{
		{"hello --holder d1", "", "d1", exit.OK, 0, 0, testNow.Add(12 * time.Hour)},
		{"hello --holder d2", "lock held: dev by d1", "d1", exit.Held, time.Hour, 0, testNow.Add(12 * time.Hour)},
		{"hello", "usage: monet-source dev hello|beat --holder <session>", "d1", exit.Usage, time.Hour, 0, testNow.Add(12 * time.Hour)},
		{"hello --holder d2 x", "usage: monet-source dev hello|beat --holder <session>", "d1", exit.Usage, time.Hour, 0, testNow.Add(12 * time.Hour)},
		{"hello --holder d2", "", "d2", exit.OK, 13 * time.Hour, 13 * time.Hour, testNow.Add(25 * time.Hour)},
	})
}

// beat renews the lease of its holder. No lease or another holder is exit 4.
func TestDevBeat(t *testing.T) {
	runDev(t, []devStep{
		{"beat --holder d1", "lock held: dev is not held by d1", "", exit.Held, 0, -1, time.Time{}},
		{"hello --holder d1", "", "d1", exit.OK, 0, 0, testNow.Add(12 * time.Hour)},
		{"beat --holder d1", "", "d1", exit.OK, time.Hour, time.Hour, testNow.Add(13 * time.Hour)},
		{"beat --holder d2", "lock held: dev is not held by d2", "d1", exit.Held, 2 * time.Hour, time.Hour, testNow.Add(13 * time.Hour)},
	})
}
