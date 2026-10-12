package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gang/internal/store"
)

// rsTaskCommit commits file=content on branch in repo (creating the branch
// from integration when absent) with a Task: <task> trailer, then returns
// to main. It returns the new tip.
func rsTaskCommit(t *testing.T, repo, branch, task, file, content string) string {
	t.Helper()
	if gitT(t, repo, "for-each-ref", "--format=%(objectname)", "refs/heads/"+branch) == "" {
		gitT(t, repo, "switch", "-q", "-c", branch, "integration")
	} else {
		gitT(t, repo, "switch", "-q", branch)
	}
	if err := os.WriteFile(filepath.Join(repo, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", file)
	gitT(t, repo, "commit", "-q", "-m", task+" work", "-m", "Task: "+task)
	gitT(t, repo, "switch", "-q", "main")
	return gitT(t, repo, "rev-parse", "refs/heads/"+branch)
}

type mergeJSON struct {
	Status   string   `json:"status"`
	TrunkSHA string   `json:"trunk_sha"`
	Files    []string `json:"files"`
	Reason   string   `json:"reason"`
}

func runMerge(t *testing.T, c *Ctx, id, sha string) (mergeJSON, int) {
	t.Helper()
	out, stderr, code := runGang(t, c, "merge", id, "--sha", sha)
	var r mergeJSON
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("merge %s printed %q (stderr %q): %v", id, out, stderr, err)
	}
	return r, code
}

func TestMergeCommandExitCodes(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1"), rsTask("C5")))
	c := rsCtx(dir, repo)
	a1 := rsTaskCommit(t, repo, "gang/A1", "A1", "README.md", "from A1\n")
	c5 := rsTaskCommit(t, repo, "gang/C5", "C5", "README.md", "from C5\n")

	// Review Focus 1: a SHA that is not the branch tip, or a tip with no
	// clean ledger row, is refused with exit 2.
	integ := gitT(t, repo, "rev-parse", "refs/heads/integration")
	r, code := runMerge(t, c, "A1", integ)
	if code != 2 || r.Status != "REFUSED" || !strings.HasPrefix(r.Reason, "tip moved") {
		t.Fatalf("stale sha: code=%d %+v", code, r)
	}
	r, code = runMerge(t, c, "A1", a1)
	if code != 2 || r.Status != "REFUSED" || !strings.HasPrefix(r.Reason, "ledger not CLEAN") {
		t.Fatalf("no ledger row: code=%d %+v", code, r)
	}

	for _, id := range []string{"A1", "C5"} {
		sha := map[string]string{"A1": a1, "C5": c5}[id]
		if _, _, code := runGang(t, c, "ledger", "record", id, sha, "--lens", "review", "--blockers", "0", "--majors", "0", "--survivors", "0", "--gate-ran", "--mutation-checked", "--evidence", "e.md"); code != 0 {
			t.Fatalf("ledger record %s: %d", id, code)
		}
	}
	r, code = runMerge(t, c, "A1", a1)
	if code != 0 || r.Status != "MERGED" || r.TrunkSHA != gitT(t, repo, "rev-parse", "refs/heads/integration") {
		t.Fatalf("merge A1: code=%d %+v", code, r)
	}
	r, code = runMerge(t, c, "A1", a1)
	if code != 0 || r.Status != "ALREADY" {
		t.Fatalf("merge A1 again: code=%d %+v", code, r)
	}
	r, code = runMerge(t, c, "C5", c5)
	if code != 3 || r.Status != "CONFLICT" || len(r.Files) != 1 || r.Files[0] != "README.md" {
		t.Fatalf("merge C5: code=%d %+v", code, r)
	}
}

// Contract 0 rule 11: after MERGED, merge appends "<id>\t<minutes>" to
// run/durations.tsv, whole minutes rounded up from claimed_at. A task
// without claimed_at gets no line, and ALREADY adds nothing.
func TestMergeCommandRecordsDuration(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1"), rsTask("B2")))
	d := store.Dir{Root: dir}
	c := rsCtx(dir, repo)
	claimed := filepath.Join(d.Run("tasks", "A1"), "claimed_at")
	if err := os.MkdirAll(filepath.Dir(claimed), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claimed, []byte(rsNow.Format(time.RFC3339)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a1 := rsTaskCommit(t, repo, "gang/A1", "A1", "a.txt", "a\n")
	b2 := rsTaskCommit(t, repo, "gang/B2", "B2", "b.txt", "b\n")
	for id, sha := range map[string]string{"A1": a1, "B2": b2} {
		if _, _, code := runGang(t, c, "ledger", "record", id, sha, "--lens", "review", "--blockers", "0", "--majors", "0", "--survivors", "0", "--gate-ran", "--mutation-checked", "--evidence", "e.md"); code != 0 {
			t.Fatalf("ledger record %s: %d", id, code)
		}
	}
	c.Now = func() time.Time { return rsNow.Add(90*time.Minute + 30*time.Second) }
	for _, step := range []struct{ id, sha, status string }{{"A1", a1, "MERGED"}, {"A1", a1, "ALREADY"}, {"B2", b2, "MERGED"}} {
		if r, code := runMerge(t, c, step.id, step.sha); code != 0 || r.Status != step.status {
			t.Fatalf("merge %s: code=%d %+v", step.id, code, r)
		}
	}
	b, err := os.ReadFile(d.Run("durations.tsv"))
	if err != nil || string(b) != "A1\t91\n" {
		t.Fatalf("durations.tsv = %q (%v), want one A1 line of 91 minutes", b, err)
	}
}

func TestMergeCommandUsage(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	for _, args := range [][]string{{"merge", "A1"}, {"merge", "--sha", "abc"}, {"merge", "A1", "B2", "--sha", "abc"}} {
		if _, _, code := runGang(t, c, args...); code != 64 {
			t.Errorf("%v: code=%d, want 64", args, code)
		}
	}
}
