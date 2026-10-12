package main

import (
	"os"
	"path/filepath"
	"testing"

	"gang/internal/testkit"
)

func TestWorktreesListsOnlyCleanMergedTaskWorktrees(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	t1 := testkit.CommitOn(t, repo, p.Repo.Integration, "T1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	testkit.CommitOn(t, repo, "gang/T2", "T2 wip", map[string]string{"b/T2.go": "package b\n"})
	wt := func(name string) string { return filepath.Join(repo, ".claude", "worktrees", name) }

	testkit.Git(t, repo, "worktree", "add", "-q", "--detach", wt("merged"), t1)
	testkit.Git(t, repo, "worktree", "add", "-q", "--detach", wt("dirty"), t1)
	if err := os.WriteFile(filepath.Join(wt("dirty"), "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	testkit.Git(t, repo, "worktree", "add", "-q", wt("ahead"), "gang/T2")
	testkit.Git(t, repo, "worktree", "add", "-q", "--detach", wt("locked"), t1)
	testkit.Git(t, repo, "worktree", "lock", wt("locked"))
	testkit.Git(t, repo, "worktree", "add", "-q", "--detach", filepath.Join(t.TempDir(), "elsewhere"), t1)

	c := &Ctx{Dir: d.Root}
	out, errOut, code := runGang(t, c, "worktrees")
	if want := "git -C " + repo + " worktree remove " + wt("merged") + "\n"; code != 0 || out != want {
		t.Fatalf("exit %d\nstdout %q\nwant   %q\nstderr %q", code, out, want, errOut)
	}
	if _, err := os.Stat(wt("merged")); err != nil {
		t.Fatal("gang worktrees removed a worktree")
	}
}

func TestWorktreesWithNoWorktreeDir(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	out, _, code := runGang(t, &Ctx{Dir: d.Root}, "worktrees")
	if code != 0 || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
}

func TestShellQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/repo/.claude/worktrees/a": "/repo/.claude/worktrees/a",
		"/my repo/x":                "'/my repo/x'",
		"it's":                      `'it'\''s'`,
	} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
