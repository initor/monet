package main

import (
	"strings"
	"testing"
	"time"

	"source/internal/exit"
	"source/internal/lock"
	"source/internal/store"
)

// wantLockUsage is the lock usage line, typed out so that a change to
// lockUsage fails a test.
const wantLockUsage = "usage: monet-source lock acquire|renew|release <name> --holder <h> [--ttl 12h]"

// checkLease fails unless lease name in run expires at want, or is absent
// when want is zero.
func checkLease(t *testing.T, run, name string, want time.Time) {
	t.Helper()
	l, ok, err := lock.Read(store.Dir{Root: run}.Locks(), name)
	if err != nil || ok != !want.IsZero() || !l.Expires.Equal(want) {
		t.Errorf("lease %s: %+v, ok %v, err %v, want expiry %v", name, l, ok, err, want)
	}
}

// The rows run in order on one run. Another holder is exit 4 and a bad call
// is exit 64. acquire and renew default to leaseTTL, and --ttl overrides it.
// renew of a free lease is exit 4 and takes no lease.
func TestLockCommand(t *testing.T) {
	run := realTempDir(t)
	later := testNow.Add(time.Hour)
	for _, tc := range []struct {
		now     time.Time
		args    []string
		code    int
		stderr  string
		expires time.Time
	}{
		{testNow, []string{"acquire", "brain", "--holder", "h1"}, exit.OK, "", testNow.Add(12 * time.Hour)},
		{testNow, []string{"acquire", "brain", "--holder", "h2"}, exit.Held, "lock held: brain by h1 until", testNow.Add(12 * time.Hour)},
		{later, []string{"renew", "brain", "--holder", "h1"}, exit.OK, "", later.Add(12 * time.Hour)},
		{later, []string{"renew", "--ttl", "1h", "--holder", "h1", "brain"}, exit.OK, "", later.Add(time.Hour)},
		{testNow, []string{"release", "brain", "--holder", "h2"}, exit.Held, "lock held: brain by h1", later.Add(time.Hour)},
		{testNow, []string{"release", "brain", "--holder", "h1"}, exit.OK, "", time.Time{}},
		{testNow, []string{"renew", "brain", "--holder", "h1"}, exit.Held, "lock held: brain is not held by h1", time.Time{}},
		{testNow, []string{"acquire", "brain", "--holder", "h1", "--ttl", "1h"}, exit.OK, "", testNow.Add(time.Hour)},
		{testNow, []string{"release", "brain", "--holder", "h1"}, exit.OK, "", time.Time{}},
		{testNow, []string{"steal", "brain"}, exit.Usage, `lock: unknown subcommand "steal": want acquire, release, renew`, time.Time{}},
		{testNow, []string{"acquire", "brain"}, exit.Usage, wantLockUsage, time.Time{}},
		{testNow, []string{"acquire", "--holder", "h1"}, exit.Usage, wantLockUsage, time.Time{}},
		{testNow, []string{"acquire", "brain", "dev", "--holder", "h1"}, exit.Usage, wantLockUsage, time.Time{}},
	} {
		now := tc.now
		_, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return now }}, append([]string{"lock"}, tc.args...)...)
		if code != tc.code || (tc.stderr == "") != (e == "") || !strings.Contains(e, tc.stderr) {
			t.Errorf("%q: code %d, stderr %q, want %d with %q", tc.args, code, e, tc.code, tc.stderr)
		}
		checkLease(t, run, "brain", tc.expires)
	}
}
