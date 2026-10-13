package main

import (
	"strings"
	"testing"
	"time"

	"gang/internal/testkit"
)

// The checks themselves are tested in internal/preflight. This pins the output
// format and the exit codes.
func TestPreflightCommandReportsFailures(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "login", "acme-bot\n")
	c := &Ctx{Dir: d.Root, Now: func() time.Time { return time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC) }}

	if _, _, code := runGang(t, c, "preflight"); code != 64 {
		t.Fatalf("no --holder: exit %d, want 64", code)
	}
	out, errOut, code := runGang(t, c, "preflight", "--holder", "s1")
	if code != 1 || !strings.Contains(errOut, "checks failed") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 10 || !strings.HasPrefix(lines[0], "FAIL repo: ") || lines[1] != "PASS driver" {
		t.Fatalf("stdout =\n%s", out)
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, "PASS ") && !strings.HasPrefix(l, "FAIL ") {
			t.Errorf("bad line %q", l)
		}
	}
}
