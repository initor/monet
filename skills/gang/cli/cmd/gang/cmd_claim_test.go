package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gang/internal/store"
)

// rsWorktree adds a detached linked worktree at <repo>/.claude/worktrees/<name>.
func rsWorktree(t *testing.T, repo, name string) string {
	t.Helper()
	wt := filepath.Join(repo, ".claude", "worktrees", name)
	gitT(t, repo, "worktree", "add", "-q", "--detach", wt, "main")
	return wt
}

func TestClaimRefusesOutsideTaskWorktrees(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	outside := filepath.Join(t.TempDir(), "elsewhere")
	gitT(t, repo, "worktree", "add", "-q", "--detach", outside, "main")
	for _, cwd := range []string{repo, filepath.Join(repo, ".planning"), t.TempDir(), outside} {
		t.Chdir(cwd)
		for _, args := range [][]string{{"claim", "A1"}, {"release"}} {
			_, stderr, code := runGang(t, c, args...)
			if code != 2 || !strings.Contains(stderr, "NO-WORKTREE:") {
				t.Fatalf("%v in %s: code=%d stderr=%q", args, cwd, code, stderr)
			}
		}
	}
	if got := gitT(t, repo, "branch", "--show-current"); got != "main" {
		t.Fatalf("main checkout moved to %q", got)
	}
	if _, err := os.Stat(store.Dir{Root: dir}.Run("tasks", "A1", "claimed_at")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused claim wrote claimed_at: %v", err)
	}
}

func TestClaimCreatesBranchFromIntegration(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	integ := rsEmptyCommit(t, repo, "refs/heads/integration", "plan")
	wt := rsWorktree(t, repo, "w1")
	t.Chdir(wt)
	out, stderr, code := runGang(t, c, "claim", "A1")
	if code != 0 || out != integ+"\n" {
		t.Fatalf("claim: code=%d out=%q stderr=%q, want %s", code, out, stderr, integ)
	}
	if got := gitT(t, wt, "branch", "--show-current"); got != "gang/A1" {
		t.Fatalf("worktree on %q", got)
	}
	if out, _, code := runGang(t, c, "claim", "A1"); code != 0 || out != integ+"\n" {
		t.Fatalf("claim again in the same worktree: code=%d out=%q", code, out)
	}
	if _, _, code := runGang(t, c, "claim", "Z9"); code != 2 {
		t.Fatalf("unknown task: code=%d, want 2", code)
	}
	// A repair id outside the plan claims gang/<id> from the integration tip.
	t.Chdir(rsWorktree(t, repo, "w2"))
	if out, _, code := runGang(t, c, "claim", "F01"); code != 0 || out != integ+"\n" {
		t.Fatalf("claim F01: code=%d out=%q", code, out)
	}
	if got := gitT(t, repo, "rev-parse", "refs/heads/gang/F01"); got != integ {
		t.Fatalf("gang/F01 at %s", got)
	}
}

func TestClaimMovesBranchBetweenWorktrees(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	w1, w2 := rsWorktree(t, repo, "w1"), rsWorktree(t, repo, "w2")

	t.Chdir(w1)
	if _, _, code := runGang(t, c, "claim", "A1"); code != 0 {
		t.Fatal("claim in w1 failed")
	}
	if err := os.WriteFile(filepath.Join(w1, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, w1, "add", "a.txt")
	gitT(t, w1, "commit", "-q", "-m", "A1 work", "-m", "Task: A1")
	tip := gitT(t, w1, "rev-parse", "HEAD")

	t.Chdir(w2)
	_, stderr, code := runGang(t, c, "claim", "A1")
	if code != 4 || !strings.Contains(stderr, "w1") {
		t.Fatalf("claim while w1 holds the branch: code=%d stderr=%q", code, stderr)
	}

	t.Chdir(w1)
	if out, _, code := runGang(t, c, "release"); code != 0 || out != "" {
		t.Fatalf("release: code=%d out=%q", code, out)
	}
	if got := gitT(t, w1, "branch", "--show-current"); got != "" {
		t.Fatalf("w1 still on %q after release", got)
	}

	t.Chdir(w2)
	c.Now = func() time.Time { return rsNow.Add(time.Hour) }
	out, stderr, code := runGang(t, c, "claim", "A1")
	if code != 0 || out != tip+"\n" {
		t.Fatalf("claim in w2: code=%d out=%q stderr=%q, want %s", code, out, stderr, tip)
	}
	if got := gitT(t, w2, "branch", "--show-current"); got != "gang/A1" {
		t.Fatalf("w2 on %q", got)
	}
	// Contract 0 rule 11: claimed_at holds the first claim only.
	b, err := os.ReadFile(store.Dir{Root: dir}.Run("tasks", "A1", "claimed_at"))
	if err != nil || string(b) != rsNow.Format(time.RFC3339)+"\n" {
		t.Fatalf("claimed_at = %q (%v), want the first claim %s", b, err, rsNow.Format(time.RFC3339))
	}
}

// claim and merge agree on claimed_at: the duration is whole minutes,
// rounded up, from the first claim to the merge.
func TestClaimThenMergeRecordsDuration(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	wt := rsWorktree(t, repo, "w1")
	t.Chdir(wt)
	if _, _, code := runGang(t, c, "claim", "A1"); code != 0 {
		t.Fatal("claim failed")
	}
	if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, wt, "add", "a.txt")
	gitT(t, wt, "commit", "-q", "-m", "A1 work", "-m", "Task: A1")
	tip := gitT(t, wt, "rev-parse", "HEAD")
	if _, _, code := runGang(t, c, "ledger", "record", "A1", tip, "--lens", "review", "--blockers", "0", "--majors", "0", "--survivors", "0", "--gate-ran", "--mutation-checked", "--evidence", "e.md"); code != 0 {
		t.Fatal("ledger record failed")
	}
	c.Now = func() time.Time { return rsNow.Add(42 * time.Minute) }
	out, _, code := runGang(t, c, "merge", "A1", "--sha", tip)
	var r struct{ Status string }
	if err := json.Unmarshal([]byte(out), &r); err != nil || code != 0 || r.Status != "MERGED" {
		t.Fatalf("merge: code=%d out=%q", code, out)
	}
	b, err := os.ReadFile(store.Dir{Root: dir}.Run("durations.tsv"))
	if err != nil || string(b) != "A1\t42\n" {
		t.Fatalf("durations.tsv = %q (%v)", b, err)
	}
}

func TestClaimRefusesBranchInMainCheckout(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	gitT(t, repo, "switch", "-q", "-c", "gang/A1")
	t.Chdir(rsWorktree(t, repo, "w1"))
	if _, stderr, code := runGang(t, c, "claim", "A1"); code != 4 {
		t.Fatalf("code=%d stderr=%q, want 4", code, stderr)
	}
}
