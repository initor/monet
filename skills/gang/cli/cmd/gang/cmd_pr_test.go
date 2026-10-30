package main

import (
	"strings"
	"testing"

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
	lease(t, c, "me")
	step(c, 4, "not held by other", "T1", "--holder", "other")
	step(c, 2, "not T2", "--holder", "me", "T2")
	step(c, 0, "git -C "+repo+" push origin ", "--holder", "me", "--dry-run", "T1")
	testkit.Answer(t, ghDir, "pr-create", "https://github.com/acme/widget/pull/7\n")
	if out, errOut, code := runGang(t, c, "pr", "open", "T1", "--holder", "me"); code != 0 || out != "https://github.com/acme/widget/pull/7\n" {
		t.Fatalf("holder: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}
