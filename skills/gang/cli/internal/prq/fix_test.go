package prq

import (
	"errors"
	"os"
	"strings"
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
	testkit.LedgerRow(t, d, "F01", testkit.CommitOn(t, repo, p.Repo.Integration, "fix: tidy a\n\nTask: F01\nFixes: T1\r", map[string]string{"a/T1.go": "package a // tidy\n"}), "review", "CLEAN")
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
	if _, _, err := FixLanding(repo, d, p, "T1", "F01"); err == nil || errors.As(err, new(*exit.Error)) || !strings.Contains(err.Error(), "origin/main..") {
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
	hook := "#!/bin/sh\nf=" + d.Run("prs.tsv") + "\nawk 'BEGIN{FS=OFS=\"\\t\"} $1==\"T1\"{$6=\"later\"} {print}' \"$f\" > \"$f.tmp\" && mv \"$f.tmp\" \"$f\"\n"
	if err := os.WriteFile(repo+"/.git/hooks/pre-push", []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Fix(repo, d, p, "T1", "F01", "")
	rows, _ := Read(d)
	want := Row{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: got.HeadSHA, State: StateOpen, OpenedAt: "later", Fixes: 1}
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
	first, err := Fix(repo, d, p, "T1", "F01", "")
	if err != nil {
		t.Fatal(err)
	}
	testkit.Answer(t, ghDir, "pr-view.json", prView("OPEN", first.HeadSHA, checkOK))
	testkit.Git(t, repo, "push", "-q", "-f", "origin", testkit.CommitOn(t, repo, "other", "chore: moved", nil)+":refs/heads/gang/demo/T1") // an unrelated tip: every later push is a non-fast-forward
	testkit.CommitOn(t, repo, p.Repo.Integration, "feat: b\n\nTask: T2", map[string]string{"b.txt": "1\n", "c.txt": "1\n"})
	refuse := func(id string, files map[string]string, code int, msg string) {
		if files != nil {
			testkit.LedgerRow(t, d, id, testkit.CommitOn(t, repo, p.Repo.Integration, "fix: x\n\nTask: "+id+"\nFixes: T1", files), "review", "CLEAN")
		}
		_, err := Fix(repo, d, p, "T1", id, "")
		wantExit(t, err, code, msg)
		if rows, _ := Read(d); len(rows) != 1 || rows[0] != first {
			t.Errorf("after %s, prs.tsv = %+v, want %+v", id, rows, first)
		}
	}
	refuse("F01", nil, exit.Refused, "F01 is already in PR #7: its replay changes nothing")
	refuse("F04", map[string]string{"b.txt": "2\n", "c.txt": "2\n"}, exit.Conflict, "F04 does not replay onto PR #7: b.txt, c.txt")
	refuse("F05", map[string]string{"k.pem": "-----BEGIN " + "EC PRIVATE KEY-----\n" + strings.Repeat("x\n", 300)}, exit.Refused, "k.pem:1:private-key") // over limits.split_lines too: the scan comes first
	refuse("F06", map[string]string{"d.txt": "d\n"}, exit.Refused, "push rejected: ")
	refuse("F09", nil, exit.Refused, "F09 is not a task commit on gang-demo above origin/main")
}

func TestFixHoldsTheSizeBudgetForTheWholePR(t *testing.T) {
	for _, c := range []struct {
		name, planReason, reason, refusal string
		task, split                       int
	}{
		{"a reason does not lift the ceiling", "", "x", "split_lines 3: the queue stops, and the checkpoint splits T1", 2, 3},
		{"the ceiling comes first", "", "", "split_lines 3: the queue stops", 2, 3},
		{"no reason", " ", " ", "task_lines 3: pass --reason <why>, and record it as a ruling", 3, 5},
		{"a reason", "", "one change", "", 3, 5},
		{"the reason of the task", "big", " ", "", 3, 5},
		{"at both limits", "", "", "", 4, 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, d, p, _ := fixFixture(t)
			p.Limits.TaskLines, p.Limits.SplitLines, p.Tasks[0].Size.Reason = c.task, c.split, c.planReason
			// The PR is T1's 1 line and the fix's 3: go.sum is not counted, and the base moved on after the fork.
			testkit.Git(t, repo, "push", "-q", "origin", testkit.CommitOn(t, repo, "other", "chore: base moves on", map[string]string{"m.txt": "p\nq\n"})+":refs/heads/main")
			testkit.LedgerRow(t, d, "F05", testkit.CommitOn(t, repo, p.Repo.Integration, "fix: x\n\nTask: F05\nFixes: T1", map[string]string{"f.txt": "1\n2\n3\n", "go.sum": "x\ny\nz\n"}), "review", "CLEAN")
			before := testkit.Git(t, repo, "ls-remote", "origin", "refs/heads/gang/demo/T1")
			_, err := Fix(repo, d, p, "T1", "F05", c.reason)
			if c.refusal != "" {
				wantExit(t, err, exit.Refused, "PR #7 would have 4 changed lines after F05, above limits."+c.refusal)
			} else if err != nil {
				t.Fatal(err)
			}
			if moved := testkit.Git(t, repo, "ls-remote", "origin", "refs/heads/gang/demo/T1") != before; moved != (c.refusal == "") {
				t.Errorf("branch moved = %v, want %v", moved, c.refusal == "")
			}
		})
	}
}

func TestFixRefusesWhenThePushLeavesNoRow(t *testing.T) {
	repo, d, p, _ := fixFixture(t)
	p.Tasks = nil // the plan lacks T1: Fix then takes no size.reason from it
	if err := os.WriteFile(repo+"/.git/hooks/pre-push", []byte("#!/bin/sh\nrm "+d.Run("prs.tsv")+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Fix(repo, d, p, "T1", "F01", "")
	head, _, _ := strings.Cut(testkit.Git(t, repo, "ls-remote", "origin", "refs/heads/gang/demo/T1"), "\t")
	if err == nil || errors.As(err, new(*exit.Error)) || !strings.HasPrefix(err.Error(), "prs.tsv has no row for T1") || !strings.Contains(err.Error(), head) {
		t.Fatalf("Fix = %v, want a plain error: prs.tsv has no row for T1, pushed head %s", err, head)
	}
	if rows, _ := Read(d); len(rows) != 0 {
		t.Errorf("prs.tsv = %+v, want no row written back", rows)
	}
}

func TestFixGoesOnWhenAPushLandsAndFails(t *testing.T) {
	repo, d, p, _ := fixFixture(t)
	hook := "#!/bin/sh\nread l s r o\ngit push -q --no-verify \"$1\" $s:$r\nexit 1\n"
	if err := os.WriteFile(repo+"/.git/hooks/pre-push", []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Fix(repo, d, p, "T1", "F01", "")
	head, _, _ := strings.Cut(testkit.Git(t, repo, "ls-remote", "origin", "refs/heads/gang/demo/T1"), "\t")
	if rows, _ := Read(d); err != nil || got.HeadSHA != head || got.Fixes != 1 || len(rows) != 1 || rows[0] != got {
		t.Fatalf("Fix = %+v, %v, prs.tsv = %+v, want a row at the landed head %s with 1 fix", got, err, rows, head)
	}
}

func TestFixReturnsThePushedHeadWhenPrsTsvIsUnreadable(t *testing.T) {
	repo, d, p, _ := fixFixture(t)
	if err := os.WriteFile(repo+"/.git/hooks/pre-push", []byte("#!/bin/sh\necho bad > "+d.Run("prs.tsv")+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Fix(repo, d, p, "T1", "F01", "")
	head, _, _ := strings.Cut(testkit.Git(t, repo, "ls-remote", "origin", "refs/heads/gang/demo/T1"), "\t")
	if err == nil || got != (Row{Branch: "gang/demo/T1", HeadSHA: head}) {
		t.Fatalf("Fix = %+v, %v, want the pushed head %s on gang/demo/T1 with the read error", got, err, head)
	}
}
