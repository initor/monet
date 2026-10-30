package prq

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
	"gang/internal/testkit"
)

func TestPutReplacesTheTaskRow(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	open := Row{Task: "T1", Branch: "gang/demo/T1", HeadSHA: "abc1234", State: StateOpen, OpenedAt: "2026-10-29T10:00:00Z"}
	other := Row{Task: "T2", Branch: "a\tb", State: StateClosed}
	merged := open
	merged.PR, merged.State, merged.Fixes = 7, StateMerged, 2
	for _, r := range []Row{open, other, merged} {
		if err := Put(d, r); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(d.Run("prs.tsv"))
	want := "task\tpr\tbranch\thead_sha\tstate\topened_at\tfixes\n" +
		"T1\t7\tgang/demo/T1\tabc1234\tmerged\t2026-10-29T10:00:00Z\t2\n" +
		"T2\t0\ta b\t\tclosed\t\t0\n"
	if err != nil || string(raw) != want {
		t.Fatalf("prs.tsv = %q, want %q", raw, want)
	}
	rows, err := Read(d)
	if err != nil || !slices.Equal(rows, []Row{merged, {Task: "T2", Branch: "a b", State: StateClosed}}) {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
}

func TestReadWithoutAFile(t *testing.T) {
	rows, err := Read(store.Dir{Root: t.TempDir()})
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
}

func TestCurrentFindsTheOpenRow(t *testing.T) {
	merged := Row{Task: "T1", State: StateMerged}
	closed := Row{Task: "T3", State: StateClosed}
	open := Row{Task: "T2", State: StateOpen}
	if got, ok := Current([]Row{merged, open}); !ok || got != open {
		t.Fatalf("Current = %+v, %v; want %+v, true", got, ok, open)
	}
	if got, ok := Current([]Row{merged, closed}); ok {
		t.Fatalf("Current = %+v, true; want no open row", got)
	}
}

func prView(state, head string, checks ...string) string {
	return fmt.Sprintf(`{"state":%q,"mergeable":"MERGEABLE","headRefOid":%q,"statusCheckRollup":[%s]}`, state, head, strings.Join(checks, ","))
}

func TestSetStateKeepsANewerHead(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	for i, head := range []string{"h1", "h2"} {
		if err := Put(d, Row{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: head, State: StateOpen, OpenedAt: "2026-10-29T10:00:00Z", Fixes: i}); err != nil {
			t.Fatal(err)
		}
	}
	err := SetState(d, "T1", StateMerged)
	rows, readErr := Read(d)
	if want := []Row{{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: "h2", State: StateMerged, OpenedAt: "2026-10-29T10:00:00Z", Fixes: 1}}; err != nil || readErr != nil || !slices.Equal(rows, want) {
		t.Fatalf("rows = %+v, errs = %v, %v, want %+v", rows, err, readErr, want)
	}
	if err := SetState(d, "T9", StateMerged); err == nil || err.Error() != "prs.tsv has no row for T9" || errors.As(err, new(*exit.Error)) {
		t.Fatalf("SetState of a missing row = %v", err)
	}
}

func TestLiveRefusesUnlessTheRecordedPRIsOpen(t *testing.T) {
	d, p := store.Dir{Root: t.TempDir()}, testkit.Plan()
	state, _ := testkit.FakeGH(t)
	if err := write(d, []Row{{Task: "T0", PR: 6, State: StateMerged}, {Task: "T1", PR: 7, HeadSHA: "h1", State: StateOpen}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ id, view, refusal string }{
		{"T2", prView("OPEN", "h1"), "T2 has no open PR"},
		{"T1", prView("CLOSED", "h1"), "PR #7 of T1 is CLOSED on GitHub"},
		{"T1", prView("OPEN", "0123456789"), "PR #7 head moved: 0123456, recorded h1"},
	} {
		testkit.Answer(t, state, "pr-view.json", c.view)
		_, _, err := Live(d, p, c.id)
		wantExit(t, err, exit.Refused, c.refusal)
	}
	testkit.Answer(t, state, "pr-view.json", prView("OPEN", "h1"))
	row, st, err := Live(d, p, "T1")
	if err != nil || row.PR != 7 || st.HeadSHA != "h1" {
		t.Fatalf("Live = %+v, %+v, %v, want PR 7 at h1", row, st, err)
	}
	p.Repo.GHAccount = ""
	_, _, err = Live(d, p, "T1")
	wantExit(t, err, exit.Refused, "repo.gh_account is empty: gang makes no GitHub call for T1")
	if got := testkit.Calls(t, state); len(got) != 3 || !strings.HasPrefix(got[2], "pr view 7 --repo github.com/acme/widget ") {
		t.Fatalf("gh calls = %q, want one for each Live that reached GitHub, the last for PR 7 of github.com/acme/widget", got)
	}
}

const checkOK = `{"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"SUCCESS"}`

// openPR leaves T1 with PR 7 open on a branch of two commits, and a fake gh that shows it green at its head.
func openPR(t *testing.T) (repo string, d store.Dir, p *plan.Plan, ghDir, head string) {
	t.Helper()
	repo, d, p = queueFixture(t)
	testkit.Git(t, repo, "branch", "pr", "refs/remotes/origin/main")
	testkit.CommitOn(t, repo, "pr", "T1: work\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	head = testkit.CommitOn(t, repo, "pr", "fix: tidy a\n\nTask: F01\nTask: T9\nFixes: T1", map[string]string{"a/T1.go": "package a // tidy\n"})
	if err := Put(d, Row{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: head, State: StateOpen}); err != nil {
		t.Fatal(err)
	}
	ghDir, _ = testkit.FakeGH(t)
	testkit.Answer(t, ghDir, "pr-view.json", prView("OPEN", head, checkOK))
	testkit.Answer(t, ghDir, "pr-merge", "")
	return repo, d, p, ghDir, head
}

func TestMergeSquashesWithTheTaskLines(t *testing.T) {
	repo, d, p, ghDir, head := openPR(t)
	subject, err := Merge(repo, d, p, "T1")
	if err != nil || subject != "T1: work (#7)" {
		t.Fatalf("Merge = %q, %v, want %q", subject, err, "T1: work (#7)")
	}
	calls := testkit.Calls(t, ghDir)
	if want := "pr merge 7 --repo github.com/acme/widget --squash --match-head-commit " + head + " --subject T1: work (#7) --body-file " + d.Run("merge-body-T1.md"); calls[len(calls)-1] != want {
		t.Errorf("last gh call = %q, want %q", calls[len(calls)-1], want)
	}
	if body, err := os.ReadFile(ghDir + "/body.md"); err != nil || string(body) != "Task: T1\nTask: F01\n" {
		t.Errorf("body = %q, %v, want the first Task line of each commit", body, err)
	}
	rows, readErr := Read(d)
	_, statErr := os.Stat(d.Run("merge-body-T1.md"))
	if want := []Row{{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: head, State: StateMerged}}; readErr != nil || !slices.Equal(rows, want) || !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("rows = %+v, %v, merge-body-T1.md stat = %v, want %+v and no merge-body file", rows, readErr, statErr, want)
	}
}

func TestMergeRefusesUntilGreen(t *testing.T) {
	for _, c := range []struct{ check, refusal string }{
		{`{"__typename":"CheckRun","name":"unit","status":"COMPLETED","conclusion":"FAILURE"},{"__typename":"CheckRun","name":"lint","status":"COMPLETED","conclusion":"FAILURE"}`, "PR #7 is not green, failed: lint,unit"},
		{"", "PR #7 is not green"},
		{`{"__typename":"CheckRun","name":"build","status":"IN_PROGRESS"}`, "PR #7 is not green"},
	} {
		repo, d, p, ghDir, head := openPR(t)
		testkit.Answer(t, ghDir, "pr-view.json", prView("OPEN", head, c.check))
		_, err := Merge(repo, d, p, "T1")
		wantExit(t, err, exit.Refused, c.refusal)
		rows, _ := Read(d)
		if calls := testkit.Calls(t, ghDir); err.Error() != c.refusal || len(calls) != 1 || rows[0].State != StateOpen {
			t.Fatalf("err = %q, gh calls = %q, rows = %+v, want %q, only the pr view, an open row", err, calls, rows, c.refusal)
		}
	}
}

func TestMergeKeepsTheRowOpenWhenGHFails(t *testing.T) {
	repo, d, p, ghDir, _ := openPR(t)
	if err := os.Remove(ghDir + "/pr-merge"); err != nil {
		t.Fatal(err)
	}
	_, err := Merge(repo, d, p, "T1")
	rows, _ := Read(d)
	if err == nil || errors.As(err, new(*exit.Error)) || !strings.HasPrefix(err.Error(), "gh pr merge 7 ") || !strings.HasSuffix(err.Error(), "/pr-merge: No such file or directory") || rows[0].State != StateOpen {
		t.Fatalf("err = %v, rows = %+v, want the plain gh error and an open row", err, rows)
	}
}

func TestMergeFailsOnAPRWithNoCommitAboveTheBase(t *testing.T) {
	repo, d, p, ghDir, _ := openPR(t)
	base := testkit.Git(t, repo, "rev-parse", "refs/remotes/origin/main")
	if err := Put(d, Row{Task: "T1", PR: 7, HeadSHA: base, State: StateOpen}); err != nil {
		t.Fatal(err)
	}
	testkit.Answer(t, ghDir, "pr-view.json", prView("OPEN", base, checkOK))
	if _, err := Merge(repo, d, p, "T1"); err == nil || errors.As(err, new(*exit.Error)) || err.Error() != "PR #7 has no commit above origin/main" {
		t.Fatalf("Merge error = %v, want the plain error: no commit above origin/main", err)
	}
}

func TestMergeRefusesUnlessTheOldestCommitIsTheTask(t *testing.T) {
	const refusal = "PR #7: the oldest commit above origin/main is not the T1 task commit"
	// The last case is a local origin/main older than the PR base: another task's squash sits below T1.
	for _, msgs := range [][]string{{"T1: work\n\nTask: T9\nTask: T1"}, {"T1: work"}, {"T2: ship (#6)\n\nTask: T2", "T1: work\n\nTask: T1"}} {
		repo, d, p, ghDir, _ := openPR(t)
		testkit.Git(t, repo, "branch", "stale", "refs/remotes/origin/main")
		head := ""
		for _, msg := range msgs {
			head = testkit.CommitOn(t, repo, "stale", msg, map[string]string{"stale.txt": msg})
		}
		if err := Put(d, Row{Task: "T1", PR: 7, HeadSHA: head, State: StateOpen}); err != nil {
			t.Fatal(err)
		}
		testkit.Answer(t, ghDir, "pr-view.json", prView("OPEN", head, checkOK))
		_, err := Merge(repo, d, p, "T1")
		wantExit(t, err, exit.Refused, refusal)
		rows, _ := Read(d)
		if calls := testkit.Calls(t, ghDir); err.Error() != refusal || len(calls) != 1 || rows[0].State != StateOpen {
			t.Fatalf("commit messages %q: err = %q, gh calls = %q, rows = %+v, want %q, only the pr view, an open row", msgs, err, calls, rows, refusal)
		}
	}
}
