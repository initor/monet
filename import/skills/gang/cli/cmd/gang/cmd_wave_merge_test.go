package main

import (
	"strings"
	"testing"

	"gang/internal/testkit"
)

func TestWaveMergeCommand(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.Origin(t, repo)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	testkit.Git(t, repo, "branch", p.Repo.Integration, "main")
	t1 := testkit.MergeTask(t, repo, p.Repo.Integration, "T1", map[string]string{"a/T1.go": "package a\n"})
	tip := testkit.Git(t, repo, "rev-parse", "refs/heads/"+p.Repo.Integration)
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/3\n")
	c := &Ctx{Dir: d.Root}
	if _, errOut, code := runGang(t, c, "wave", "open", "1"); code != 0 {
		t.Fatalf("open: %s", errOut)
	}
	testkit.Answer(t, state, "pr-view.json", `{"state":"OPEN","mergeable":"MERGEABLE","headRefOid":"`+tip+`","statusCheckRollup":[{"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"SUCCESS"}]}`)
	testkit.Answer(t, state, "merge-commit", "abababababababababababababababababababab\n")
	testkit.Answer(t, state, "pr-merge", "")

	_, errOut, code := runGang(t, c, "wave", "merge", "1")
	if code != 2 || !strings.Contains(errOut, "NOT-VERIFIED review") {
		t.Fatalf("unreviewed: exit %d, stderr %q", code, errOut)
	}
	testkit.LedgerRow(t, d, "T1", t1, "review", "CLEAN")
	out, errOut, code := runGang(t, c, "wave", "merge", "1")
	if code != 0 || out != "abababababababababababababababababababab\n" {
		t.Fatalf("merge: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}
