package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"gang/internal/plan"
	"gang/internal/testkit"
)

func prRun(t *testing.T, edit func(p *plan.Plan)) (c *Ctx, repo, ghDir string) {
	repo, p := testkit.Repo(t), testkit.Plan()
	if edit != nil {
		edit(p)
	}
	testkit.Origin(t, repo)
	d := testkit.GangDir(t, repo, p)
	testkit.Git(t, repo, "branch", p.Repo.Integration, "main")
	testkit.LedgerRow(t, d, "T1", testkit.MergeTask(t, repo, p.Repo.Integration, "T1", map[string]string{"a/T1.go": "package a\n"}), "review", "CLEAN")
	ghDir, _ = testkit.FakeGH(t)
	return &Ctx{Dir: d.Root}, repo, ghDir
}

func lease(t *testing.T, c *Ctx, holder string) {
	t.Helper()
	if _, errOut, code := runGang(t, c, "lock", "acquire", "driver", "--holder", holder, "--ttl", "12h"); code != 0 {
		t.Fatalf("lease for %s: exit %d, stderr %q", holder, code, errOut)
	}
}

func TestPROpenNeedsTheDriverLease(t *testing.T) {
	c, repo, ghDir := prRun(t, nil)
	step := func(cc *Ctx, code int, want string, args ...string) {
		t.Helper()
		if out, errOut, got := runGang(t, cc, append([]string{"pr", "open"}, args...)...); got != code || !strings.Contains(out+errOut, want) || len(testkit.Calls(t, ghDir)) != 0 {
			t.Fatalf("pr open %v: exit %d, stdout %q, stderr %q, gh calls %q, want exit %d, %q, and no gh calls", args, got, out, errOut, testkit.Calls(t, ghDir), code, want)
		}
	}
	step(c, 64, "pr open: want 1 argument(s), got 0")
	step(c, 64, "--holder <session> is required: the driver lease holder", "T1")
	step(&Ctx{Dir: c.Dir + "/run"}, 4, "not held by me", "T1", "--holder", "me") // no plan here: the lease check comes first
	lease(t, &Ctx{Dir: c.Dir + "/run"}, "me")
	step(&Ctx{Dir: c.Dir + "/run"}, 1, "plan", "T1", "--holder", "me")
	lock := c.Dir + "/run/locks/driver.json"
	if err := os.MkdirAll(lock, 0o755); err != nil {
		t.Fatal(err)
	}
	step(c, 1, "is a directory", "T1", "--holder", "me") // a lease that cannot be read stops the command
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	lease(t, c, "me")
	step(c, 4, "not held by other", "T1", "--holder", "other")
	step(c, 2, "not T2", "--holder", "me", "T2")
	step(c, 0, "git -C "+repo+" push origin ", "--holder", "me", "--dry-run", "T1")
	testkit.Answer(t, ghDir, "pr-create", "https://github.com/acme/widget/pull/7\n")
	later := &Ctx{Dir: c.Dir, Now: func() time.Time { return time.Date(2026, 10, 11, 13, 0, 0, 0, time.UTC) }}
	if out, errOut, code := runGang(t, later, "pr", "open", "T1", "--holder", "me"); code != 0 || out != "https://github.com/acme/widget/pull/7\n" {
		t.Fatalf("holder: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	for file, want := range map[string]string{"locks/driver.json": "2026-10-12T01:00:00Z", "prs.tsv": "\t2026-10-11T13:00:00Z\t"} {
		if b, err := os.ReadFile(c.Dir + "/run/" + file); err != nil || !strings.Contains(string(b), want) {
			t.Fatalf("%s: %q, %v, want %q", file, b, err, want)
		}
	}
}
