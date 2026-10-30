package watch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gang/internal/plan"
	"gang/internal/prq"
	"gang/internal/store"
	"gang/internal/testkit"
	"gang/internal/wave"
)

var t0 = time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

type watchFixture struct {
	repo, gh string
	d        store.Dir
	p        *plan.Plan
}

func newWatchFixture(t *testing.T) watchFixture {
	t.Helper()
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	state, _ := testkit.FakeGH(t)
	return watchFixture{repo: repo, gh: state, d: d, p: p}
}

func (f watchFixture) merge(t *testing.T, id string) {
	t.Helper()
	testkit.CommitOn(t, f.repo, f.p.Repo.Integration, id+"\n\nTask: "+id, map[string]string{id + ".go": "package x\n"})
}

func (f watchFixture) openWave(t *testing.T, k, pr int) {
	t.Helper()
	body := fmt.Sprintf("wave\tpr\thead_sha\tstate\topened_at\n%d\t%d\tabc\topen\t2026-10-11T00:00:00Z\n", k, pr)
	if err := os.WriteFile(f.d.Run("waves.tsv"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f watchFixture) openPR(t *testing.T, head string) {
	t.Helper()
	if err := prq.Put(f.d, prq.Row{Task: "T1", PR: 8, Branch: "gang/demo/T1", HeadSHA: head, State: prq.StateOpen}); err != nil {
		t.Fatal(err)
	}
}

func (f watchFixture) pr(t *testing.T, state, mergeable, head string, checks ...string) {
	t.Helper()
	testkit.Answer(t, f.gh, "pr-view.json", fmt.Sprintf(`{"state":%q,"mergeable":%q,"headRefOid":%q,"statusCheckRollup":[%s]}`,
		state, mergeable, head, strings.Join(checks, ",")))
}

func check(name, conclusion string) string {
	return fmt.Sprintf(`{"__typename":"CheckRun","name":%q,"status":"COMPLETED","conclusion":%q}`, name, conclusion)
}

func poll(t *testing.T, f watchFixture, now time.Time) []string {
	t.Helper()
	lines, err := Poll(f.repo, f.d, f.p, now)
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

func want(t *testing.T, got []string, w ...string) {
	t.Helper()
	if len(got) == 0 && len(w) == 0 {
		return
	}
	if !reflect.DeepEqual(got, w) {
		t.Fatalf("events = %q, want %q", got, w)
	}
}

func TestWaveReadyPrintsOnce(t *testing.T) {
	f := newWatchFixture(t)
	want(t, poll(t, f, t0))
	f.merge(t, "T1")
	want(t, poll(t, f, t0), "WAVE_READY 1")
	want(t, poll(t, f, t0.Add(time.Minute)))
	b, _ := os.ReadFile(f.d.Run("events.tsv"))
	if !strings.HasPrefix(string(b), "key\tat\nWAVE_READY 1\t") {
		t.Fatalf("events.tsv =\n%s", b)
	}
}

func TestWaveReadyNamesHighestCompleteWave(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.merge(t, "T2")
	want(t, poll(t, f, t0), "WAVE_READY 2")
}

func TestNoWaveReadyWhileAPRIsOpen(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.merge(t, "T2")
	f.openWave(t, 1, 7)
	f.pr(t, "OPEN", "MERGEABLE", "h1", check("build", "IN_PROGRESS"))
	want(t, poll(t, f, t0))
}

// Review Focus 4: an empty check list is never green.
func TestEmptyCheckListPrintsNothing(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.openWave(t, 1, 7)
	f.pr(t, "OPEN", "MERGEABLE", "h1")
	want(t, poll(t, f, t0))
	want(t, poll(t, f, t0.Add(time.Minute)))
}

// Contract 0.8: the key carries the PR head, so a new push re-raises the event.
func TestPRGreenNeedsAPassingCheckAndRepeatsOnANewHead(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.openWave(t, 1, 7)
	f.pr(t, "OPEN", "MERGEABLE", "h1", check("build", "SUCCESS"))
	want(t, poll(t, f, t0), "PR_GREEN 1 7")
	want(t, poll(t, f, t0.Add(time.Minute)))
	f.pr(t, "OPEN", "MERGEABLE", "h2", check("build", "SUCCESS"))
	want(t, poll(t, f, t0.Add(2*time.Minute)), "PR_GREEN 1 7")
	b, _ := os.ReadFile(f.d.Run("events.tsv"))
	if !strings.Contains(string(b), "PR_GREEN 1 7 @h1\t") || !strings.Contains(string(b), "PR_GREEN 1 7 @h2\t") {
		t.Fatalf("events.tsv keys lack the head:\n%s", b)
	}
}

// Contract 0.9: without repo.gh_account, watch never runs gh, and still
// prints WAVE_READY (de-duplicated by events.tsv alone) and STALL.
func TestNoGHAccountNeverCallsGH(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	p.Repo.GHAccount = ""
	p.Limits.StallMin = 10
	d := testkit.GangDir(t, repo, p)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin) // no gh at all
	f := watchFixture{repo: repo, d: d, p: p}
	f.merge(t, "T1")
	f.openWave(t, 9, 3) // a stray open row must not make watch call gh
	f.openPR(t, "h1")   // nor must an open prs.tsv row
	want(t, poll(t, f, t0))
	if err := os.Remove(d.Run("waves.tsv")); err != nil {
		t.Fatal(err)
	}
	want(t, poll(t, f, t0.Add(time.Minute)), "WAVE_READY 1")
	want(t, poll(t, f, t0.Add(2*time.Minute)))
	want(t, poll(t, f, t0.Add(10*time.Minute)), "STALL 10")
}

func TestPRRedNamesFailingChecksAndRepeatsOnANewHead(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.openWave(t, 1, 7)
	f.pr(t, "OPEN", "MERGEABLE", "h1", check("unit", "FAILURE"), check("build", "SUCCESS"), check("lint", "TIMED_OUT"))
	want(t, poll(t, f, t0), "PR_RED 1 7 lint,unit")
	want(t, poll(t, f, t0.Add(time.Minute)))
	// A fix is pushed, CI runs again on head h2, and the same checks fail.
	f.pr(t, "OPEN", "MERGEABLE", "h2", check("unit", "FAILURE"), check("build", "SUCCESS"), check("lint", "TIMED_OUT"))
	want(t, poll(t, f, t0.Add(2*time.Minute)), "PR_RED 1 7 lint,unit")
}

// Contract 0.8 per task: the key holds the PR head, so a new head raises the event again.
func TestPRGreenAndRedNameTheTask(t *testing.T) {
	f := newWatchFixture(t)
	f.openPR(t, "h1")
	f.pr(t, "OPEN", "MERGEABLE", "h1", check("build", "SUCCESS"))
	want(t, poll(t, f, t0), "PR_GREEN T1 8")
	want(t, poll(t, f, t0.Add(time.Minute)))
	f.pr(t, "OPEN", "MERGEABLE", "h2", check("unit", "FAILURE"))
	want(t, poll(t, f, t0.Add(2*time.Minute)), "PR_RED T1 8 unit")
	f.pr(t, "OPEN", "MERGEABLE", "h3", check("unit", "FAILURE"))
	want(t, poll(t, f, t0.Add(3*time.Minute)), "PR_RED T1 8 unit")
}

// Steps are (GitHub state, row state, check conclusion). A PR that is not open raises no event.
func TestPRStateFromGitHubSettlesTheRow(t *testing.T) {
	f := newWatchFixture(t)
	f.openPR(t, "h1")
	for _, s := range [][3]string{{"CLOSED", prq.StateClosed, "FAILURE"}, {"OPEN", prq.StateOpen, "IN_PROGRESS"}, {"MERGED", prq.StateMerged, "FAILURE"}} {
		f.pr(t, s[0], "UNKNOWN", "h2", check("unit", s[2]))
		want(t, poll(t, f, t0))
		rows, err := prq.Read(f.d)
		if err != nil || len(rows) != 1 || rows[0].State != s[1] || rows[0].HeadSHA != "h1" {
			t.Fatalf("after %s: rows = %+v, err = %v", s[0], rows, err)
		}
	}
	want(t, poll(t, f, t0))
	if calls := testkit.Calls(t, f.gh); len(calls) != 3 {
		t.Fatalf("calls = %q, want one pr view per settle and none after the merge", calls)
	}
}

func TestMergedPRFreesTheNextWave(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.merge(t, "T2")
	f.openWave(t, 1, 7)
	f.pr(t, "MERGED", "UNKNOWN", "h1")
	want(t, poll(t, f, t0), "WAVE_READY 2")
	rows, err := wave.Read(f.d)
	if err != nil || rows[0].State != wave.StateMerged {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
}

// A PR closed and reopened to retrigger CI, whose poll landed in the closed
// window, is open again on the next poll. A merged row is never polled.
func TestClosedPRReopenedIsOpenAgain(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.merge(t, "T2")
	body := "wave\tpr\thead_sha\tstate\topened_at\n1\t6\tabc\tmerged\t2026-10-11T00:00:00Z\n2\t7\tdef\tclosed\t2026-10-12T00:00:00Z\n"
	if err := os.WriteFile(f.d.Run("waves.tsv"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f.pr(t, "OPEN", "MERGEABLE", "h1", check("build", "SUCCESS"))
	want(t, poll(t, f, t0), "PR_GREEN 2 7")
	rows, err := wave.Read(f.d)
	if err != nil || len(rows) != 2 || rows[0].State != wave.StateMerged || rows[1].State != wave.StateOpen {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	calls := testkit.Calls(t, f.gh)
	if len(calls) != 1 || !strings.HasPrefix(calls[0], "pr view 7 ") {
		t.Fatalf("calls = %q, want one pr view of the closed PR 7", calls)
	}
}

func TestGHErrorIsReportedAndBlocksWaveReady(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.merge(t, "T2")
	f.openWave(t, 1, 7) // no pr-view.json answer: gh fails
	lines, err := Poll(f.repo, f.d, f.p, t0)
	if err == nil || len(lines) != 0 {
		t.Fatalf("lines %q, err %v", lines, err)
	}
}

func TestStallAfterStallMinPerEpisode(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.openWave(t, 1, 7)
	f.pr(t, "OPEN", "MERGEABLE", "h1", check("build", "IN_PROGRESS"))
	report := filepath.Join(f.d.Run("tasks", "T2"), "report.md")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pin every mtime under run/tasks, so the real clock cannot leak in.
	touch := func(at time.Time) {
		for _, p := range []string{f.d.Run("tasks"), filepath.Dir(report), report} {
			if err := os.Chtimes(p, at, at); err != nil {
				t.Fatal(err)
			}
		}
	}
	touch(t0)
	limit := time.Duration(f.p.Limits.StallMin) * time.Minute

	want(t, poll(t, f, t0))
	want(t, poll(t, f, t0.Add(limit-time.Second)))
	want(t, poll(t, f, t0.Add(limit)), fmt.Sprintf("STALL %d", f.p.Limits.StallMin))
	want(t, poll(t, f, t0.Add(limit+time.Minute)))
	want(t, poll(t, f, t0.Add(2*limit)), fmt.Sprintf("STALL %d", 2*f.p.Limits.StallMin))

	// Activity starts a new episode, and its stall is reported again.
	active := t0.Add(2*limit + 10*time.Minute)
	touch(active)
	want(t, poll(t, f, active))
	want(t, poll(t, f, active.Add(limit)), fmt.Sprintf("STALL %d", f.p.Limits.StallMin))
}

func TestTrunkMoveResetsStall(t *testing.T) {
	f := newWatchFixture(t)
	f.p.Limits.StallMin = 10
	f.merge(t, "T1")
	f.openWave(t, 1, 7)
	f.pr(t, "OPEN", "MERGEABLE", "h1", check("build", "IN_PROGRESS"))
	want(t, poll(t, f, t0))
	f.merge(t, "T2")
	want(t, poll(t, f, t0.Add(10*time.Minute)))
	want(t, poll(t, f, t0.Add(20*time.Minute)), "STALL 10")
}

func TestReadyWave(t *testing.T) {
	p := testkit.Plan()
	cases := []struct {
		name string
		rows []wave.Row
		done map[string]bool
		k    int
		ok   bool
	}{
		{"nothing done", nil, map[string]bool{}, 0, false},
		{"wave 1 done", nil, map[string]bool{"T1": true}, 1, true},
		{"both done", nil, map[string]bool{"T1": true, "T2": true}, 2, true},
		{"wave 1 recorded", []wave.Row{{Wave: 1, State: wave.StateMerged}}, map[string]bool{"T1": true}, 0, false},
		{"wave 2 after a recorded wave 1", []wave.Row{{Wave: 1, State: wave.StateMerged}}, map[string]bool{"T1": true, "T2": true}, 2, true},
		{"T2 done out of order", nil, map[string]bool{"T2": true}, 0, false},
	}
	for _, c := range cases {
		k, ok := readyWave(p, c.rows, c.done)
		if k != c.k || ok != c.ok {
			t.Errorf("%s: got (%d, %v), want (%d, %v)", c.name, k, ok, c.k, c.ok)
		}
	}
}

// A gh error on a task PR comes back beside the events and does not block WAVE_READY.
func TestTaskPRGHErrorComesBesideTheEvents(t *testing.T) {
	f := newWatchFixture(t)
	f.merge(t, "T1")
	f.openPR(t, "h1") // no pr-view.json answer: gh fails
	lines, err := Poll(f.repo, f.d, f.p, t0)
	if err == nil || !reflect.DeepEqual(lines, []string{"WAVE_READY 1"}) {
		t.Fatalf("lines %q, err %v", lines, err)
	}
}

func TestPRNextNamesTheNextTaskOnce(t *testing.T) {
	f := newWatchFixture(t)
	f.p.Repo.GHAccount = "" // PR_NEXT reads only git and prs.tsv
	testkit.Origin(t, f.repo)
	f.merge(t, "T1")
	want(t, poll(t, f, t0), "WAVE_READY 1", "PR_NEXT T1")
	want(t, poll(t, f, t0.Add(time.Minute)))
	f.p.Repo.GHAccount = "acme-bot"
	f.openPR(t, "h1")
	f.pr(t, "OPEN", "MERGEABLE", "h1", check("build", "IN_PROGRESS"))
	testkit.CommitOn(t, f.repo, f.p.Repo.Integration, "feat: add T2\n\nTask: T2", map[string]string{"t2.go": "package x\n"})
	want(t, poll(t, f, t0.Add(2*time.Minute)), "WAVE_READY 2")
	f.pr(t, "MERGED", "UNKNOWN", "h1")
	want(t, poll(t, f, t0.Add(3*time.Minute)), "PR_NEXT T2")
	testkit.CommitRef(t, f.repo, "refs/remotes/origin/main", "feat: ship T2\n\nTask: T2", map[string]string{"t2": "x\n"})
	want(t, poll(t, f, t0.Add(4*time.Minute)))
}

func TestPRNextWaitsOnAStrayAndAGHError(t *testing.T) {
	f := newWatchFixture(t)
	testkit.Origin(t, f.repo)
	f.merge(t, "T1")
	testkit.CommitOn(t, f.repo, f.p.Repo.Integration, "chore: stray", map[string]string{"s.go": "package x\n"})
	f.openPR(t, "h1")
	f.pr(t, "CLOSED", "UNKNOWN", "h1")
	lines, err := Poll(f.repo, f.d, f.p, t0) // the row closes, then PR_NEXT meets the stray
	if err == nil || !reflect.DeepEqual(lines, []string{"WAVE_READY 1"}) {
		t.Fatalf("lines %q, err %v", lines, err)
	}
	testkit.Answer(t, f.gh, "pr-view.json", "") // gh fails: PR_NEXT must not ask for the stray again
	if lines, err = Poll(f.repo, f.d, f.p, t0.Add(time.Minute)); err == nil || len(lines) != 0 || strings.Contains(err.Error(), "stray") {
		t.Fatalf("lines %q, err %v", lines, err)
	}
}

func TestPRRedAfterTheGraceWithNoChecks(t *testing.T) {
	f := newWatchFixture(t)
	f.p.Limits.ChecksGraceMin = 3
	t.Setenv("GIT_COMMITTER_DATE", t0.Format(time.RFC3339))
	head := testkit.CommitOn(t, f.repo, "pr", "T1: work\n\nTask: T1", map[string]string{"a.txt": "a\n"})
	f.openPR(t, head)
	f.pr(t, "OPEN", "MERGEABLE", head)
	want(t, poll(t, f, t0.Add(2*time.Minute)))
	f.openPR(t, "h1") // git cannot read this head
	want(t, poll(t, f, t0.Add(3*time.Minute)))
	f.openPR(t, head)
	f.pr(t, "OPEN", "MERGEABLE", head, check("build", "IN_PROGRESS"))
	want(t, poll(t, f, t0.Add(3*time.Minute)))
	f.pr(t, "OPEN", "MERGEABLE", head)
	f.p.Repo.Checks = plan.ChecksGate
	want(t, poll(t, f, t0.Add(3*time.Minute)))
	f.p.Repo.Checks = plan.ChecksGitHub
	want(t, poll(t, f, t0.Add(3*time.Minute)), "PR_RED T1 8 no-checks")
	want(t, poll(t, f, t0.Add(4*time.Minute)))
}
