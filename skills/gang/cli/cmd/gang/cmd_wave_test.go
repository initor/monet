package main

import (
	"strings"
	"testing"

	"gang/internal/testkit"
)

func TestWaveOpenAndStatusCommands(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.Origin(t, repo)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	testkit.Git(t, repo, "branch", p.Repo.Integration, "main")
	testkit.MergeTask(t, repo, p.Repo.Integration, "T1", map[string]string{"a/T1.go": "package a\n"})
	tip := testkit.Git(t, repo, "rev-parse", "refs/heads/"+p.Repo.Integration)
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/12\n")
	c := &Ctx{Dir: d.Root}

	out, errOut, code := runGang(t, c, "wave", "open", "--dry-run", "1")
	if code != 0 || !strings.HasPrefix(out, "chore(gang): land wave 1 of demo\n") {
		t.Fatalf("dry run: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	out, errOut, code = runGang(t, c, "wave", "open", "1")
	if code != 0 || out != "https://github.com/acme/widget/pull/12\n" {
		t.Fatalf("open: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	out, _, code = runGang(t, c, "wave", "status")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if code != 0 || len(lines) != 2 || lines[0] != "wave\tpr\thead_sha\tstate\topened_at" || !strings.HasPrefix(lines[1], "1\t12\t"+tip+"\topen\t") {
		t.Fatalf("status: exit %d\n%s", code, out)
	}
	for _, args := range [][]string{{"wave"}, {"wave", "bogus"}, {"wave", "open"}, {"wave", "open", "x"}, {"wave", "open", "0"}, {"wave", "status", "extra"}} {
		if _, _, code := runGang(t, c, args...); code != 64 {
			t.Errorf("%v: exit %d, want 64", args, code)
		}
	}
}
