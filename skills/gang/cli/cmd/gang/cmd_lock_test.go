package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gang/internal/store"
)

// Review Focus 5: a live holder is refused with exit 4, and only expiry
// lets another holder take over.
func TestLockCommandDriverLease(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	at := func(d time.Duration) { c.Now = func() time.Time { return rsNow.Add(d) } }
	steps := []struct {
		at   time.Duration
		args []string
		code int
	}{
		{0, []string{"lock", "acquire", "driver", "--holder", "s1"}, 0},
		{time.Minute, []string{"lock", "acquire", "driver", "--holder", "s2"}, 4},
		{time.Minute, []string{"lock", "renew", "driver", "--holder", "s2"}, 4},
		{time.Minute, []string{"lock", "release", "driver", "--holder", "s2"}, 4},
		{time.Minute, []string{"lock", "renew", "driver", "--holder", "s1"}, 0},
		{5*time.Minute + 59*time.Second, []string{"lock", "acquire", "driver", "--holder", "s2"}, 4},
		{6 * time.Minute, []string{"lock", "acquire", "driver", "--holder", "s2", "--ttl", "1m"}, 0},
		{6 * time.Minute, []string{"lock", "acquire", "driver", "--holder", "s1"}, 4},
		{7 * time.Minute, []string{"lock", "acquire", "driver", "--holder", "s1"}, 0},
		{7 * time.Minute, []string{"lock", "release", "driver", "--holder", "s1"}, 0},
	}
	for i, s := range steps {
		at(s.at)
		if out, stderr, code := runGang(t, c, s.args...); code != s.code || out != "" {
			t.Fatalf("step %d %v at +%s: code=%d out=%q stderr=%q, want %d", i, s.args, s.at, code, out, stderr, s.code)
		}
	}
	lease := filepath.Join(store.Dir{Root: dir}.Run("locks"), "driver.json")
	if _, err := os.Stat(lease); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lease left after release: %v", err)
	}
}

func TestLockCommandUsage(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	for _, args := range [][]string{
		{"lock"},
		{"lock", "steal", "driver", "--holder", "s1"},
		{"lock", "acquire", "driver"},
		{"lock", "acquire", "--holder", "s1"},
		{"lock", "acquire", "driver", "--holder", "s1", "--ttl", "soon"},
	} {
		if _, _, code := runGang(t, c, args...); code != 64 {
			t.Errorf("%v: code=%d, want 64", args, code)
		}
	}
	if _, _, code := runGang(t, c, "lock", "acquire", "../x", "--holder", "s1"); code != 1 {
		t.Errorf("bad lock name: code=%d, want 1", code)
	}
}
