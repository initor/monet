package main

import (
	"testing"
	"time"

	"gang/internal/testkit"
)

func TestWatchOnce(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	testkit.FakeGH(t)
	testkit.CommitOn(t, repo, p.Repo.Integration, "T1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	c := &Ctx{Dir: d.Root, Now: func() time.Time { return time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC) }}

	out, errOut, code := runGang(t, c, "watch", "--once")
	if code != 0 || out != "WAVE_READY 1\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if out, _, code = runGang(t, c, "watch", "--once"); code != 0 || out != "" {
		t.Fatalf("second pass: exit %d, stdout %q", code, out)
	}
	if _, _, code = runGang(t, c, "watch", "--interval", "0s"); code != 64 {
		t.Fatalf("zero interval: exit %d, want 64", code)
	}
}

func TestWatchDelayBacksOff(t *testing.T) {
	for fails, want := range []time.Duration{60, 120, 240, 480, 480} {
		if got := watchDelay(60, fails); got != want {
			t.Errorf("fails %d: %v, want %v", fails, got, want)
		}
	}
}
