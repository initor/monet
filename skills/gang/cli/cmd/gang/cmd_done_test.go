package main

import (
	"testing"
)

// rsEmptyCommit adds an empty commit with msg on top of ref without a checkout.
func rsEmptyCommit(t *testing.T, repo, ref, msg string) string {
	t.Helper()
	tip := gitT(t, repo, "rev-parse", ref)
	sha := gitT(t, repo, "commit-tree", tip+"^{tree}", "-p", tip, "-m", msg)
	gitT(t, repo, "update-ref", ref, sha, tip)
	return sha
}

// Contract 0 rule 1: a task is done when a commit reachable from the
// integration branch holds the whole line "Task: <id>", trailer or not.
func TestDoneListsTaskLinesOnIntegration(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1"), rsTask("B2"), rsTask("C3"), rsTask("D4")))
	c := rsCtx(dir, repo)
	if out, _, code := runGang(t, c, "done"); code != 0 || out != "" {
		t.Fatalf("fresh integration: code=%d out=%q", code, out)
	}
	rsEmptyCommit(t, repo, "refs/heads/integration", "b2 work\n\nTask: B2\n\nTesting Done: make check")
	rsEmptyCommit(t, repo, "refs/heads/integration", "a1 work\n\nTask: A1")
	rsEmptyCommit(t, repo, "refs/heads/integration", "merge B2: Task B2")
	rsEmptyCommit(t, repo, "refs/heads/integration", "d4 notes\n\nTask: D4 (later)\n  Task: D4\nSubtask: D4")
	rsEmptyCommit(t, repo, "refs/heads/main", "c3 on main only\n\nTask: C3")
	out, _, code := runGang(t, c, "done")
	if code != 0 || out != "A1\nB2\n" {
		t.Fatalf("code=%d out=%q, want A1 and B2 sorted, without C3 or D4", code, out)
	}
}

func TestDoneWithoutIntegrationBranch(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	gitT(t, repo, "update-ref", "-d", "refs/heads/integration")
	out, _, code := runGang(t, rsCtx(dir, repo), "done")
	if code != 0 || out != "" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}
