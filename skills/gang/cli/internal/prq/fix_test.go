package prq

import (
	"errors"
	"testing"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
	"gang/internal/testkit"
)

func fixFixture(t *testing.T) (repo string, d store.Dir, p *plan.Plan, ghDir string) {
	t.Helper()
	repo, d, p = queueFixture(t)
	testkit.Git(t, repo, "branch", "pr", "refs/remotes/origin/main")
	head := testkit.CommitOn(t, repo, "pr", "T1: work\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	testkit.Git(t, repo, "push", "-q", "origin", "pr:refs/heads/gang/demo/T1")
	if err := Put(d, Row{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: head, State: StateOpen}); err != nil {
		t.Fatal(err)
	}
	testkit.LedgerRow(t, d, "F01", testkit.CommitOn(t, repo, p.Repo.Integration, "fix: tidy a\n\nTask: F01\nFixes: T1", map[string]string{"a/T1.go": "package a // tidy\n"}), "review", "CLEAN")
	ghDir, _ = testkit.FakeGH(t)
	testkit.Answer(t, ghDir, "pr-view.json", prView("OPEN", head, checkOK))
	return repo, d, p, ghDir
}

func TestFixLandingFindsTheReviewedFix(t *testing.T) {
	repo, d, p, _ := fixFixture(t)
	testkit.CommitOn(t, repo, p.Repo.Integration, "chore: no task", nil)
	r, l, err := FixLanding(repo, d, p, "T1", "F01")
	if err != nil || r.PR != 7 || r.Fixes != 0 || l.ID != "F01" || l.Subject != "fix: tidy a" {
		t.Fatalf("FixLanding = %+v, %+v, %v, want PR 7 with no fix and F01 %q", r, l, err, "fix: tidy a")
	}
	testkit.Git(t, repo, "update-ref", "-d", "refs/remotes/origin/main")
	if _, _, err := FixLanding(repo, d, p, "T1", "F01"); err == nil || errors.As(err, new(*exit.Error)) {
		t.Fatalf("FixLanding without origin/main = %v, want a plain git error", err)
	}
}

func TestFixLandingRefusals(t *testing.T) {
	for _, c := range []struct {
		name, fix, refusal string
		edit               func(*Row)
	}{
		{"two fixes", "F09", "PR #7 of T1 has 2 fix commits, the most a PR takes: the queue stops for the checkpoint", func(r *Row) { r.Fixes = 2 }},
		{"more than two fixes", "F09", "PR #7 of T1 has 2 fix commits", func(r *Row) { r.Fixes = 3 }},
		{"head moved", "F09", ", recorded 0123456", func(r *Row) { r.HeadSHA = "0123456789" }},
		{"one fix, F01 already shipped", "F01", "F01 is not a task commit on gang-demo above origin/main", func(r *Row) { r.Fixes = 1 }},
		{"fixes another task", "F02", `F02 does not fix T1: its task commit has no line "Fixes: T1"`, nil},
		{"reviewed at the merge, not the tip", "F03", "F03 tip ", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, d, p, _ := fixFixture(t)
			testkit.Git(t, repo, "push", "-q", "origin", p.Repo.Integration+":main")
			testkit.CommitOn(t, repo, p.Repo.Integration, "fix: b\n\nTask: F02\nFixes: T10\nNot Fixes: T1", map[string]string{"b.txt": "b\n"})
			tip := testkit.CommitOn(t, repo, "gang/F03", "fix: c\n\nTask: F03\nFixes: T1", map[string]string{"c.txt": "c\n"})
			testkit.LedgerRow(t, d, "F03", testkit.GangMerge(t, repo, p.Repo.Integration, "F03", "Build F03", tip), "review", "CLEAN")
			if c.edit != nil {
				rows, _ := Read(d)
				c.edit(&rows[0])
				if err := Put(d, rows[0]); err != nil {
					t.Fatal(err)
				}
			}
			_, _, err := FixLanding(repo, d, p, "T1", c.fix)
			wantExit(t, err, exit.Refused, c.refusal)
		})
	}
}

func TestFixPushesOneFastForwardCommit(t *testing.T) {
	repo, d, p, _ := fixFixture(t)
	got, err := Fix(repo, d, p, "T1", "F01")
	rows, _ := Read(d)
	want := Row{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: got.HeadSHA, State: StateOpen, Fixes: 1}
	if err != nil || got != want || len(rows) != 1 || rows[0] != want {
		t.Fatalf("Fix = %+v, %v, prs.tsv = %+v, want %+v", got, err, rows, want)
	}
	for _, c := range [][2]string{
		{testkit.Git(t, repo, "log", "-1", "--format=%P %B", got.HeadSHA), testkit.Git(t, repo, "rev-parse", "refs/heads/pr") + " fix: tidy a\n\nTask: F01\nFixes: T1"},
		{testkit.Git(t, repo, "ls-remote", "origin", "refs/heads/gang/demo/T1"), got.HeadSHA + "\trefs/heads/gang/demo/T1"},
		{testkit.Git(t, repo, "show", got.HeadSHA+":a/T1.go"), "package a // tidy"},
	} {
		if c[0] != c[1] {
			t.Errorf("got %q, want %q", c[0], c[1])
		}
	}
}

func TestFixRefusesAReplayThatCannotShip(t *testing.T) {
	repo, d, p, ghDir := fixFixture(t)
	first, err := Fix(repo, d, p, "T1", "F01")
	if err != nil {
		t.Fatal(err)
	}
	testkit.Answer(t, ghDir, "pr-view.json", prView("OPEN", first.HeadSHA, checkOK))
	testkit.Git(t, repo, "push", "-q", "-f", "origin", testkit.CommitOn(t, repo, "other", "chore: moved", nil)+":refs/heads/gang/demo/T1")
	testkit.CommitOn(t, repo, p.Repo.Integration, "feat: b\n\nTask: T2", map[string]string{"b.txt": "1\n", "c.txt": "1\n"})
	refuse := func(id string, files map[string]string, code int, msg string) {
		if files != nil {
			testkit.LedgerRow(t, d, id, testkit.CommitOn(t, repo, p.Repo.Integration, "fix: x\n\nTask: "+id+"\nFixes: T1", files), "review", "CLEAN")
		}
		_, err := Fix(repo, d, p, "T1", id)
		wantExit(t, err, code, msg)
		if rows, _ := Read(d); len(rows) != 1 || rows[0] != first {
			t.Errorf("after %s, prs.tsv = %+v, want %+v", id, rows, first)
		}
	}
	refuse("F01", nil, exit.Refused, "F01 is already in PR #7: its replay changes nothing")
	refuse("F04", map[string]string{"b.txt": "2\n", "c.txt": "2\n"}, exit.Conflict, "F04 does not replay onto PR #7: b.txt, c.txt")
	refuse("F05", map[string]string{"k.pem": "-----BEGIN " + "EC PRIVATE KEY-----\n"}, exit.Refused, "k.pem:1:private-key")
	refuse("F06", map[string]string{"d.txt": "d\n"}, exit.Refused, "push rejected: ")
	refuse("F09", nil, exit.Refused, "F09 is not a task commit on gang-demo above origin/main")
}
