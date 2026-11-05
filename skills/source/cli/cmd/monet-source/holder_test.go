package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"source/internal/exit"
	"source/internal/lock"
	"source/internal/store"
)

// requireLease renews a lease that the holder has for 12 hours. An empty
// holder is exit 64, another holder or no lease is exit 4, and any other
// lease error passes through with no exit code.
func TestRequireLease(t *testing.T) {
	run := realTempDir(t)
	c := &Ctx{Run: run, Now: func() time.Time { return testNow }}
	if _, err := lock.Acquire(store.Dir{Root: run}.Locks(), "dev", "h1", time.Minute, testNow); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, holder string
		code         int
		msg          string
	}{
		{"dev", "", exit.Usage, "--holder <session> is required: the dev lease holder"},
		{"dev", "h2", exit.Held, "lock held: dev is not held by h2"},
		{"brain", "h1", exit.Held, "lock held: brain is not held by h1"},
	} {
		var e *exit.Error
		if err := requireLease(c, tc.name, tc.holder); !errors.As(err, &e) || e.Code != tc.code || e.Msg != tc.msg {
			t.Errorf("%s %q: err %#v, want code %d %q", tc.name, tc.holder, err, tc.code, tc.msg)
		}
	}
	checkLease(t, run, "dev", testNow.Add(time.Minute))
	var e *exit.Error
	if err := requireLease(c, "bad/name", "h1"); err == nil || errors.As(err, &e) || !strings.Contains(err.Error(), "must match") {
		t.Errorf("bad name: err %#v, want a plain lease error", err)
	}
	if err := requireLease(c, "dev", "h1"); err != nil {
		t.Fatalf("holder: %v", err)
	}
	checkLease(t, run, "dev", testNow.Add(12*time.Hour))
}
