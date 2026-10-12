package wave

import (
	"fmt"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/testkit"
)

func prView(state, head string, checks ...string) string {
	return fmt.Sprintf(`{"state":%q,"mergeable":"MERGEABLE","headRefOid":%q,"statusCheckRollup":[%s]}`, state, head, strings.Join(checks, ","))
}

const checkOK = `{"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"SUCCESS"}`

// openWave1 opens wave 1 for a fixture where T1 took a fix round: its first
// commit was reviewed CHANGES, and gang merge landed the fix, which is CLEAN
// only when cleanFix is set. It returns the fixture, the fake gh dir, and the
// integration tip.
func openWave1(t *testing.T, cleanFix bool) (waveFixture, string, string) {
	t.Helper()
	repo := testkit.Repo(t)
	origin := testkit.Origin(t, repo)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	f := waveFixture{repo: repo, origin: origin, d: d, p: p}
	state, _ := testkit.FakeGH(t)
	testkit.Git(t, repo, "branch", "gang-demo", "main")
	testkit.Git(t, repo, "branch", "gang/T1", "main")
	first := testkit.CommitOn(t, repo, "gang/T1", "T1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	fix := testkit.CommitOn(t, repo, "gang/T1", "T1 fix\n\nTask: T1", map[string]string{"a/T1.go": "package a // fixed\n"})
	testkit.GangMerge(t, repo, "gang-demo", "T1", "Build T1", fix)
	testkit.LedgerRow(t, d, "T1", first, "review", "CHANGES")
	if cleanFix {
		testkit.LedgerRow(t, d, "T1", fix, "review", "CLEAN")
	}
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/7\n")
	if _, err := Open(repo, d, p, 1, false); err != nil {
		t.Fatal(err)
	}
	tip := testkit.Git(t, repo, "rev-parse", "refs/heads/gang-demo")
	testkit.Answer(t, state, "pr-view.json", prView("OPEN", tip, checkOK))
	testkit.Answer(t, state, "merge-commit", "1111111111111111111111111111111111111111\n")
	testkit.Answer(t, state, "pr-merge", "")
	return f, state, tip
}

func merged(t *testing.T, state string) bool {
	for _, c := range testkit.Calls(t, state) {
		if strings.HasPrefix(c, "pr merge") {
			return true
		}
	}
	return false
}

func TestMergeChecksTheLandedTipThenMerges(t *testing.T) {
	f, state, tip := openWave1(t, true)
	sha, err := Merge(f.repo, f.d, f.p, 1)
	if err != nil {
		t.Fatal(err)
	}
	if sha != "1111111111111111111111111111111111111111" {
		t.Fatalf("sha = %q", sha)
	}
	want := "pr merge 7 --repo github.com/acme/widget --merge --match-head-commit " + tip
	found := false
	for _, c := range testkit.Calls(t, state) {
		found = found || c == want
	}
	if !found {
		t.Fatalf("no %q in %q", want, testkit.Calls(t, state))
	}
	rows, _ := Read(f.d)
	if rows[0].State != StateMerged {
		t.Fatalf("row = %+v", rows[0])
	}
	_, err = Merge(f.repo, f.d, f.p, 1)
	wantExit(t, err, exit.Refused, "wave 1 has no open PR")
}

// The red wave PR flow pushes a fix while the PR is open. The push records the
// pushed tip as the row's head, so wave merge accepts it, and a head that
// matches neither tip is still refused.
func TestMergeAfterAPushToTheOpenWave(t *testing.T) {
	f, state, tip1 := openWave1(t, true)
	fix := testkit.MergeTask(t, f.repo, "gang-demo", "T2", map[string]string{"b/T2.go": "package b\n"})
	testkit.LedgerRow(t, f.d, "T2", fix, "review", "CLEAN")
	if _, err := Push(f.repo, f.d, f.p, true); err != nil {
		t.Fatal(err)
	}
	if rows, _ := Read(f.d); rows[0].HeadSHA != tip1 {
		t.Fatalf("a dry-run push moved the row head to %s", rows[0].HeadSHA)
	}
	tip2, err := Push(f.repo, f.d, f.p, false)
	if err != nil {
		t.Fatal(err)
	}
	if tip2 == tip1 || tip2 != testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang-demo") {
		t.Fatalf("pushed %s, want the new integration tip", tip2)
	}
	rows, err := Read(f.d)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].HeadSHA != tip2 || rows[0].State != StateOpen {
		t.Fatalf("rows = %+v, want wave 1 open at %s", rows, tip2)
	}

	testkit.Answer(t, state, "pr-view.json", prView("OPEN", "3333333333333333333333333333333333333333", checkOK))
	_, err = Merge(f.repo, f.d, f.p, 1)
	wantExit(t, err, exit.Refused, "PR #7 head moved")

	testkit.Answer(t, state, "pr-view.json", prView("OPEN", tip2, checkOK))
	if _, err := Merge(f.repo, f.d, f.p, 1); err != nil {
		t.Fatal(err)
	}
	want := "pr merge 7 --repo github.com/acme/widget --merge --match-head-commit " + tip2
	found := false
	for _, c := range testkit.Calls(t, state) {
		found = found || c == want
	}
	if !found {
		t.Fatalf("no %q in %q", want, testkit.Calls(t, state))
	}
}

// Contract 0.2: a task tip can be a sync merge commit with no Task line. The
// ledger row is for that tip, and the wave still merges.
func TestMergeAcceptsASyncMergeTip(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.Origin(t, repo)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	state, _ := testkit.FakeGH(t)
	testkit.Git(t, repo, "branch", "gang-demo", "main")
	testkit.Git(t, repo, "branch", "gang/T2", "main")
	t2 := testkit.CommitOn(t, repo, "gang/T2", "T2\n\nTask: T2", map[string]string{"b/T2.go": "package b\n"})
	t1 := testkit.MergeTask(t, repo, "gang-demo", "T1", map[string]string{"a/T1.go": "package a\n"})
	trunk := testkit.Git(t, repo, "rev-parse", "refs/heads/gang-demo")
	tree := testkit.Git(t, repo, "merge-tree", "--write-tree", t2, trunk)
	sync := testkit.Git(t, repo, "commit-tree", tree, "-p", t2, "-p", trunk, "-m", "sync gang-demo into gang/T2")
	testkit.GangMerge(t, repo, "gang-demo", "T2", "Build T2", sync)
	testkit.LedgerRow(t, d, "T1", t1, "review", "CLEAN")
	testkit.LedgerRow(t, d, "T2", sync, "review", "CLEAN")
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/9\n")
	if _, err := Open(repo, d, p, 2, false); err != nil {
		t.Fatal(err)
	}
	tip := testkit.Git(t, repo, "rev-parse", "refs/heads/gang-demo")
	testkit.Answer(t, state, "pr-view.json", prView("OPEN", tip, checkOK))
	testkit.Answer(t, state, "merge-commit", "2222222222222222222222222222222222222222\n")
	testkit.Answer(t, state, "pr-merge", "")
	if _, err := Merge(repo, d, p, 2); err != nil {
		t.Fatal(err)
	}
}

func TestMergeRefusals(t *testing.T) {
	t.Run("ledger not clean", func(t *testing.T) {
		f, state, _ := openWave1(t, false)
		fix := testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang/T1")
		_, err := Merge(f.repo, f.d, f.p, 1)
		wantExit(t, err, exit.Refused, "T1 "+fix[:7]+" NOT-VERIFIED review")
		if merged(t, state) {
			t.Fatal("merged a wave with an unverified task")
		}
	})
	t.Run("high risk needs adversarial", func(t *testing.T) {
		f, state, _ := openWave1(t, true)
		f.p.Tasks[0].Risk = "high"
		_, err := Merge(f.repo, f.d, f.p, 1)
		wantExit(t, err, exit.Refused, "NOT-VERIFIED adversarial")
		if merged(t, state) {
			t.Fatal("merged without the adversarial lens")
		}
	})
	t.Run("a commit gang merge did not land", func(t *testing.T) {
		f, state, _ := openWave1(t, true)
		testkit.CommitOn(t, f.repo, "gang-demo", "hand edit", map[string]string{"h.txt": "h\n"})
		rows, _ := Read(f.d)
		rows[0].HeadSHA = testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang-demo")
		if err := write(f.d, rows); err != nil {
			t.Fatal(err)
		}
		testkit.Answer(t, state, "pr-view.json", prView("OPEN", rows[0].HeadSHA, checkOK))
		_, err := Merge(f.repo, f.d, f.p, 1)
		wantExit(t, err, exit.Refused, "not landed by gang merge: ")
	})
	// A hand-made merge with the gang title and a CLEAN second parent, whose
	// tree adds a file that git merge-tree of its parents does not produce.
	t.Run("a gang merge title on a tree gang merge did not write", func(t *testing.T) {
		f, state, _ := openWave1(t, true)
		trunk := testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang-demo")
		testkit.Git(t, f.repo, "branch", "gang/T2", trunk)
		t2 := testkit.CommitOn(t, f.repo, "gang/T2", "T2\n\nTask: T2", map[string]string{"b/T2.go": "package b\n"})
		testkit.LedgerRow(t, f.d, "T2", t2, "review", "CLEAN")
		testkit.Git(t, f.repo, "branch", "forge", t2)
		forged := testkit.CommitOn(t, f.repo, "forge", "forge", map[string]string{"b/unreviewed.go": "package b\n"})
		tree := testkit.Git(t, f.repo, "rev-parse", forged+"^{tree}")
		merge := testkit.Git(t, f.repo, "commit-tree", tree, "-p", trunk, "-p", t2, "-m", "merge T2: Build T2")
		testkit.Git(t, f.repo, "update-ref", "refs/heads/gang-demo", merge)
		rows, _ := Read(f.d)
		rows[0].HeadSHA = merge
		if err := write(f.d, rows); err != nil {
			t.Fatal(err)
		}
		testkit.Answer(t, state, "pr-view.json", prView("OPEN", merge, checkOK))
		_, err := Merge(f.repo, f.d, f.p, 1)
		wantExit(t, err, exit.Refused, "not landed by gang merge: "+merge[:7]+" merge T2: Build T2")
		if merged(t, state) {
			t.Fatal("merged a wave with a forged gang merge")
		}
	})
	t.Run("not green", func(t *testing.T) {
		f, state, tip := openWave1(t, true)
		testkit.Answer(t, state, "pr-view.json", prView("OPEN", tip, `{"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"FAILURE"}`))
		_, err := Merge(f.repo, f.d, f.p, 1)
		wantExit(t, err, exit.Refused, "PR #7 is not green")
	})
	t.Run("zero checks", func(t *testing.T) {
		f, state, tip := openWave1(t, true)
		testkit.Answer(t, state, "pr-view.json", prView("OPEN", tip))
		_, err := Merge(f.repo, f.d, f.p, 1)
		wantExit(t, err, exit.Refused, "PR #7 is not green")
	})
	t.Run("head moved", func(t *testing.T) {
		f, state, _ := openWave1(t, true)
		testkit.Answer(t, state, "pr-view.json", prView("OPEN", "2222222222222222222222222222222222222222", checkOK))
		_, err := Merge(f.repo, f.d, f.p, 1)
		wantExit(t, err, exit.Refused, "PR #7 head moved")
	})
	t.Run("no row", func(t *testing.T) {
		f, _, _ := openWave1(t, true)
		_, err := Merge(f.repo, f.d, f.p, 2)
		wantExit(t, err, exit.Refused, "wave 2 has no open PR")
	})
}

// A push while the wave PR is closed moves that PR's head too, so a reopened
// PR merges at the pushed tip.
func TestMergeAfterAClosedPRIsReopened(t *testing.T) {
	f, state, _ := openWave1(t, true)
	if err := SetState(f.d, 1, StateClosed); err != nil {
		t.Fatal(err)
	}
	fix := testkit.MergeTask(t, f.repo, "gang-demo", "T2", map[string]string{"b/T2.go": "package b\n"})
	testkit.LedgerRow(t, f.d, "T2", fix, "review", "CLEAN")
	tip2, err := Push(f.repo, f.d, f.p, false)
	if err != nil {
		t.Fatal(err)
	}
	testkit.Answer(t, state, "pr-view.json", prView("OPEN", tip2, checkOK))
	if _, err := Refresh(f.d, f.p); err != nil {
		t.Fatal(err)
	}
	if _, err := Merge(f.repo, f.d, f.p, 1); err != nil {
		t.Fatalf("wave merge after the reopen: %v", err)
	}
}
