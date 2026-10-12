package wave

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/testkit"
)

const openPR = `{"state":"OPEN","mergeable":"MERGEABLE","headRefOid":"x","statusCheckRollup":[]}`

func TestOpenPushesCreatesAndRecords(t *testing.T) {
	f := newWaveFixture(t)
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/7\n")
	tip := testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang-demo")
	t1 := testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang/T1")
	testkit.LedgerRow(t, f.d, "T1", t1, "review", "CLEAN")

	url, err := Open(f.repo, f.d, f.p, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://github.com/acme/widget/pull/7" {
		t.Fatalf("url = %q", url)
	}
	if remoteRef(t, f.origin, "gang-demo") != tip {
		t.Fatal("Open did not push first")
	}
	calls := testkit.Calls(t, state)
	want := "pr create --repo github.com/acme/widget --base main --head gang-demo --title chore(gang): land wave 1 of demo --body-file "
	if len(calls) != 1 || !strings.HasPrefix(calls[0], want) {
		t.Fatalf("calls = %q", calls)
	}
	body, err := os.ReadFile(filepath.Join(state, "body.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "| T1 | Build T1 | 1 | R1 | CLEAN |\n") || !strings.Contains(string(body), "1 tasks in origin/main..gang-demo at "+tip[:7]) {
		t.Fatalf("body =\n%s", body)
	}
	rows, err := Read(f.d)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Wave != 1 || rows[0].PR != 7 || rows[0].HeadSHA != tip || rows[0].State != StateOpen || rows[0].OpenedAt == "" {
		t.Fatalf("rows = %+v", rows)
	}
	if left, _ := filepath.Glob(f.d.Run("wave-body-*.md")); len(left) != 0 {
		t.Fatalf("body file left behind: %v", left)
	}
}

func TestOpenUsesTheWaveTitleTemplate(t *testing.T) {
	f := newWaveFixture(t)
	testkit.FakeGH(t)
	f.p.Repo.WaveTitle = "feat({slug}): wave {k}"
	got, err := Open(f.repo, f.d, f.p, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "feat(demo): wave 1\n\n") {
		t.Fatalf("got %q", got)
	}
}

func TestOpenPreconditions(t *testing.T) {
	f := newWaveFixture(t)
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/7\n")
	testkit.Answer(t, state, "pr-view.json", openPR)

	_, err := Open(f.repo, f.d, f.p, 2, false)
	wantExit(t, err, exit.Refused, "wave 2 is not ready, not merged: T2")
	_, err = Open(f.repo, f.d, f.p, 5, false)
	wantExit(t, err, exit.Refused, "wave 5 has no tasks")

	if _, err := Open(f.repo, f.d, f.p, 1, false); err != nil {
		t.Fatal(err)
	}
	testkit.MergeTask(t, f.repo, "gang-demo", "T2", map[string]string{"b/T2.go": "package b\n"})
	_, err = Open(f.repo, f.d, f.p, 2, false)
	wantExit(t, err, exit.Refused, "wave 1 PR #7 is still open")

	// The owner merges PR 7 on GitHub: Refresh settles the row and wave 2 opens.
	testkit.Answer(t, state, "pr-view.json", `{"state":"MERGED","mergeable":"UNKNOWN","headRefOid":"x","statusCheckRollup":[]}`)
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/8\n")
	if _, err := Open(f.repo, f.d, f.p, 2, false); err != nil {
		t.Fatal(err)
	}
	rows, _ := Read(f.d)
	if len(rows) != 2 || rows[0].State != StateMerged || rows[1].PR != 8 || rows[1].State != StateOpen {
		t.Fatalf("rows = %+v", rows)
	}
	testkit.Answer(t, state, "pr-view.json", `{"state":"CLOSED","mergeable":"UNKNOWN","headRefOid":"x","statusCheckRollup":[]}`)
	_, err = Open(f.repo, f.d, f.p, 1, false)
	wantExit(t, err, exit.Refused, "wave 1 already has PR #7 (merged)")
}

const closedPR = `{"state":"CLOSED","mergeable":"UNKNOWN","headRefOid":"x","statusCheckRollup":[]}`

// A PR closed and reopened to retrigger CI, whose poll landed in the closed
// window: Refresh sets the row back to open. It never polls a merged row.
func TestRefreshReopensAClosedRow(t *testing.T) {
	f := newWaveFixture(t)
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "pr-view.json", openPR)
	rows := []Row{
		{Wave: 1, PR: 6, HeadSHA: "abc", State: StateMerged, OpenedAt: "2026-10-11T00:00:00Z"},
		{Wave: 2, PR: 7, HeadSHA: "def", State: StateClosed, OpenedAt: "2026-10-12T00:00:00Z"},
	}
	if err := write(f.d, rows); err != nil {
		t.Fatal(err)
	}
	got, err := Refresh(f.d, f.p)
	if err != nil {
		t.Fatal(err)
	}
	rows[1].State = StateOpen
	if len(got) != 2 || got[0] != rows[0] || got[1] != rows[1] {
		t.Fatalf("rows = %+v, want %+v", got, rows)
	}
	calls := testkit.Calls(t, state)
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "pr view 7 ") {
		t.Fatalf("calls = %q, want one pr view of the closed PR 7", calls)
	}

	// Reopened and merged between two polls: the closed row is merged.
	rows[1].State = StateClosed
	if err := write(f.d, rows); err != nil {
		t.Fatal(err)
	}
	testkit.Answer(t, state, "pr-view.json", `{"state":"MERGED","mergeable":"UNKNOWN","headRefOid":"x","statusCheckRollup":[]}`)
	if got, err = Refresh(f.d, f.p); err != nil || got[1].State != StateMerged {
		t.Fatalf("rows = %+v, err = %v, want wave 2 merged", got, err)
	}
}

// A closed wave PR does not wedge the run: wave open k replaces a closed row
// for wave k. A closed row for a later wave is still refused.
func TestOpenReplacesAClosedRowForTheSameWave(t *testing.T) {
	f := newWaveFixture(t)
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/7\n")
	if _, err := Open(f.repo, f.d, f.p, 1, false); err != nil {
		t.Fatal(err)
	}
	testkit.Answer(t, state, "pr-view.json", closedPR)
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/8\n")
	url, err := Open(f.repo, f.d, f.p, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://github.com/acme/widget/pull/8" {
		t.Fatalf("url = %q", url)
	}
	tip := testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang-demo")
	rows, err := Read(f.d)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Wave != 1 || rows[0].PR != 8 || rows[0].HeadSHA != tip || rows[0].State != StateOpen {
		t.Fatalf("rows = %+v, want one open wave 1 row for PR 8", rows)
	}

	if err := write(f.d, []Row{{Wave: 2, PR: 9, HeadSHA: tip, State: StateClosed, OpenedAt: "2026-10-12T00:00:00Z"}}); err != nil {
		t.Fatal(err)
	}
	_, err = Open(f.repo, f.d, f.p, 1, false)
	wantExit(t, err, exit.Refused, "wave 2 already has PR #9 (closed)")
}

func TestOpenDryRunRecordsNothing(t *testing.T) {
	f := newWaveFixture(t)
	state, _ := testkit.FakeGH(t)
	got, err := Open(f.repo, f.d, f.p, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "chore(gang): land wave 1 of demo\n\nWave 1 of gang run `demo`") || !strings.Contains(got, "| T1 | Build T1 | 1 | R1 | NOT-VERIFIED review |") {
		t.Fatalf("dry run =\n%s", got)
	}
	if calls := testkit.Calls(t, state); len(calls) != 0 {
		t.Fatalf("dry run called gh: %q", calls)
	}
	if _, err := os.Stat(f.d.Run("waves.tsv")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote waves.tsv")
	}
	if remoteRef(t, f.origin, "gang-demo") != "" {
		t.Fatal("dry run pushed")
	}
}

// Contract 0.9: with no GitHub account, only a dry run is allowed.
func TestOpenWithoutGHAccountNeedsDryRun(t *testing.T) {
	f := newWaveFixture(t)
	f.p.Repo.GHAccount = ""
	_, err := Open(f.repo, f.d, f.p, 1, false)
	wantExit(t, err, exit.Refused, "works only with --dry-run")
	if remoteRef(t, f.origin, "gang-demo") != "" {
		t.Fatal("a refused open pushed")
	}
	if _, err := Open(f.repo, f.d, f.p, 1, true); err != nil {
		t.Fatal(err)
	}
}

// The body reports the tip each gang merge landed: a fixed task's newest
// commit, the adversarial lens for high risk, a repair task outside the plan,
// and a commit that gang merge did not land.
func TestOpenBodyVerdicts(t *testing.T) {
	f := newWaveFixture(t)
	testkit.FakeGH(t)
	f.p.Tasks[1].Risk = "high"
	f.p.Tasks[1].Title = "Pipe | title"
	testkit.Git(t, f.repo, "update-ref", "refs/heads/gang/T2", "refs/heads/gang-demo")
	first := testkit.CommitOn(t, f.repo, "gang/T2", "T2\n\nTask: T2", map[string]string{"b/T2.go": "package b\n"})
	fix := testkit.CommitOn(t, f.repo, "gang/T2", "T2 fix\n\nTask: T2", map[string]string{"b/T2.go": "package b // fixed\n"})
	testkit.GangMerge(t, f.repo, "gang-demo", "T2", "Pipe | title", fix)
	f01 := testkit.MergeTask(t, f.repo, "gang-demo", "F01", map[string]string{"r.txt": "r\n"})
	testkit.CommitOn(t, f.repo, "gang-demo", "hand edit", map[string]string{"h.txt": "h\n"})
	t1 := testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang/T1")
	testkit.LedgerRow(t, f.d, "T1", t1, "review", "CLEAN")
	testkit.LedgerRow(t, f.d, "T2", first, "review", "CLEAN")
	testkit.LedgerRow(t, f.d, "T2", first, "adversarial", "CLEAN")
	testkit.LedgerRow(t, f.d, "T2", fix, "review", "CLEAN")
	testkit.LedgerRow(t, f.d, "F01", f01, "review", "CLEAN")

	got, err := Open(f.repo, f.d, f.p, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"3 tasks in origin/main..gang-demo",
		"| - | CLEAN |\n| T1 | Build T1 | 1 | R1 | CLEAN |\n| T2 | Pipe \\| title | 2 | R2 | NOT-VERIFIED adversarial |\n",
		"| F01 | ",
		"Commits that gang merge did not land (gang wave merge refuses these):\n\n- ",
		" hand edit\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("body lacks %q:\n%s", want, got)
		}
	}
}

func TestOpenRefusesWhenBaseHasEverything(t *testing.T) {
	f := newWaveFixture(t)
	testkit.FakeGH(t)
	testkit.Git(t, f.repo, "push", "-q", "origin", "gang-demo:main")
	_, err := Open(f.repo, f.d, f.p, 1, false)
	wantExit(t, err, exit.Refused, "nothing to open")
}
