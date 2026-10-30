package main

import (
	"strings"
	"testing"

	"gang/internal/prq"
	"gang/internal/store"
	"gang/internal/testkit"
)

// origin/main holds a commit that the local ref has not seen, and the PR branch sits on it:
// only a fetch makes T1 the oldest commit above the ref.
func TestPRMergeNeedsTheDriverLease(t *testing.T) {
	c, repo, ghDir := prRun(t, nil)
	old0 := testkit.Git(t, repo, "rev-parse", "refs/remotes/origin/main")
	testkit.CommitOn(t, repo, "pr", "other: moved\n\nTask: T9", map[string]string{"z.go": "package z\n"})
	testkit.Git(t, repo, "push", "-q", "origin", "pr:main")
	testkit.Git(t, repo, "update-ref", "refs/remotes/origin/main", old0)
	head := testkit.CommitOn(t, repo, "pr", "T1: work\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	if err := prq.Put(store.Dir{Root: c.Dir}, prq.Row{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: head, State: prq.StateOpen}); err != nil {
		t.Fatal(err)
	}
	testkit.Answer(t, ghDir, "pr-view.json", `{"state":"OPEN","mergeable":"MERGEABLE","headRefOid":"`+head+`","statusCheckRollup":[{"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"SUCCESS"}]}`)
	testkit.Answer(t, ghDir, "pr-merge", "")
	step := func(code int, want string, args ...string) {
		t.Helper()
		if out, errOut, got := runGang(t, c, append([]string{"pr", "merge", "T1"}, args...)...); got != code || !strings.Contains(out+errOut, want) || len(testkit.Calls(t, ghDir)) != 0 {
			t.Fatalf("pr merge %v: exit %d, stdout %q, stderr %q, gh calls %q, want exit %d, %q, and no gh calls", args, got, out, errOut, testkit.Calls(t, ghDir), code, want)
		}
	}
	step(64, "")
	lease(t, c, "other")
	step(4, "not held by me", "--holder", "me")
	if got := testkit.Git(t, repo, "rev-parse", "refs/remotes/origin/main"); got != old0 {
		t.Fatalf("a refused run fetched origin: origin/main %s, want %s", got, old0)
	}
	_, _, noPlan := runGang(t, &Ctx{Dir: c.Dir + "/run"}, "pr", "merge", "T1", "--holder", "me") // the lease check comes before any plan read
	if out, _, code := runGang(t, c, "pr", "merge", "T2", "--holder", "other"); noPlan != 4 || code != 2 || out != "" {
		t.Fatalf("no plan: exit %d, want 4. T2 has no open PR: exit %d, stdout %q, want exit 2 and none", noPlan, code, out)
	}
	url := testkit.Git(t, repo, "remote", "get-url", "origin")
	testkit.Git(t, repo, "remote", "set-url", "origin", repo+"/missing.git")
	if _, errOut, code := runGang(t, c, "pr", "merge", "T1", "--holder", "other"); code != 1 || !strings.Contains(errOut, "fetch origin: ") || len(testkit.Calls(t, ghDir)) != 0 {
		t.Fatalf("fetch failure: exit %d, stderr %q, gh calls %q, want exit 1, fetch origin, and no gh call", code, errOut, testkit.Calls(t, ghDir))
	}
	testkit.Git(t, repo, "remote", "set-url", "origin", url)
	if out, errOut, code := runGang(t, c, "pr", "merge", "T1", "--holder", "other"); code != 0 || out != "T1: work (#7)\n" || !strings.HasPrefix(testkit.Calls(t, ghDir)[1], "pr merge 7 ") {
		t.Fatalf("holder: exit %d, stdout %q, stderr %q, gh calls %q", code, out, errOut, testkit.Calls(t, ghDir))
	}
}
