package merge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gang/internal/gitx"
	"gang/internal/ledger"
	"gang/internal/plan"
	"gang/internal/store"
)

const integ = "refs/heads/integration"

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitx.Run(dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

type fixture struct {
	repo string
	d    store.Dir
	p    *plan.Plan
}

// newFixture makes a repo whose main and integration branches hold file.txt.
// Tasks: A1; B2 depends on A1; H3 is risk high; M4 merges after A1; C5 edits
// the same line as A1.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "config", "user.name", "Test")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "commit.gpgsign", "false")
	git(t, repo, "config", "core.excludesFile", os.DevNull)
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "file.txt")
	git(t, repo, "commit", "-q", "-m", "init")
	git(t, repo, "branch", "integration")
	task := func(id string) plan.Task {
		return plan.Task{ID: id, Title: "Task " + id, Risk: "low"}
	}
	b2, h3, m4 := task("B2"), task("H3"), task("M4")
	b2.DependsOn = []string{"A1"}
	h3.Risk = "high"
	m4.MergeAfter = []string{"A1"}
	p := &plan.Plan{Slug: "toy", Repo: plan.Repo{Base: "main", Integration: "integration"}, Tasks: []plan.Task{task("A1"), b2, h3, m4, task("C5")}}
	p.Normalize()
	return &fixture{repo: repo, d: store.Dir{Root: t.TempDir()}, p: p}
}

// commitMsg puts one commit with message msg on the task branch (made from
// integration when new) that writes file, and returns its SHA.
func (f *fixture) commitMsg(t *testing.T, branch, file, content, msg string) string {
	t.Helper()
	if git(t, f.repo, "for-each-ref", "--format=%(objectname)", "refs/heads/"+branch) == "" {
		git(t, f.repo, "switch", "-q", "-c", branch, "integration")
	} else {
		git(t, f.repo, "switch", "-q", branch)
	}
	if err := os.WriteFile(filepath.Join(f.repo, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, f.repo, "add", file)
	git(t, f.repo, "commit", "-q", "-m", msg)
	git(t, f.repo, "switch", "-q", "main")
	return git(t, f.repo, "rev-parse", "refs/heads/"+branch)
}

// commit is commitMsg with a Task: <task> trailer, or none when task is "".
func (f *fixture) commit(t *testing.T, branch, file, content, task string) string {
	t.Helper()
	msg := branch + " work"
	if task != "" {
		msg += "\n\nTask: " + task
	}
	return f.commitMsg(t, branch, file, content, msg)
}

// advance puts an empty commit on integration, standing in for another merge.
func (f *fixture) advance(t *testing.T) string {
	t.Helper()
	tip := git(t, f.repo, "rev-parse", integ)
	next := git(t, f.repo, "commit-tree", tip+"^{tree}", "-p", tip, "-m", "merge X9: elsewhere")
	git(t, f.repo, "update-ref", integ, next, tip)
	return next
}

func (f *fixture) clean(t *testing.T, id, sha string, lenses ...ledger.Lens) {
	t.Helper()
	for _, l := range lenses {
		r := ledger.Row{Task: id, SHA: sha, Lens: l, GateRan: true, MutationChecked: true, Evidence: "e.md"}
		if err := ledger.Record(f.d, r); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *fixture) merge(t *testing.T, id, sha string) Result {
	t.Helper()
	r, err := Merge(f.repo, f.d, f.p, id, sha)
	if err != nil {
		t.Fatalf("Merge(%s): %v", id, err)
	}
	return r
}

func TestMergeLandsCleanTip(t *testing.T) {
	f := newFixture(t)
	before := git(t, f.repo, "rev-parse", integ)
	a1 := f.commit(t, "gang/A1", "a.txt", "a\n", "A1")
	f.clean(t, "A1", a1, ledger.LensReview)

	r := f.merge(t, "A1", a1)
	trunk := git(t, f.repo, "rev-parse", integ)
	if r.Status != Merged || r.TrunkSHA != trunk || len(r.Files) != 0 || r.Reason != "" {
		t.Fatalf("result %+v, integration at %s", r, trunk)
	}
	if got := git(t, f.repo, "rev-list", "--parents", "-n", "1", trunk); got != trunk+" "+before+" "+a1 {
		t.Fatalf("parents line %q, want trunk then tip", got)
	}
	if got := git(t, f.repo, "log", "-1", "--format=%B", trunk); got != "merge A1: Task A1" {
		t.Fatalf("message %q", got)
	}
	if got := git(t, f.repo, "log", "-1", "--format=%B", trunk); strings.Contains(got, "\nTask: ") || strings.HasPrefix(got, "Task: ") {
		t.Fatalf("merge commit carries a Task line: %q", got)
	}
	if done, _ := gitx.TaskIDs(f.repo, integ); !reflect.DeepEqual(done, []string{"A1"}) {
		t.Fatalf("done set %v", done)
	}
	b, _ := json.Marshal(r)
	if want := `{"status":"MERGED","trunk_sha":"` + trunk + `","files":[],"reason":""}`; string(b) != want {
		t.Fatalf("json %s, want %s", b, want)
	}

	// B2 depends on A1, which is now merged.
	b2 := f.commit(t, "gang/B2", "b.txt", "b\n", "B2")
	f.clean(t, "B2", b2, ledger.LensReview)
	if r := f.merge(t, "B2", b2); r.Status != Merged {
		t.Fatalf("B2 after A1: %+v", r)
	}
}

// Each case breaks its own rule and every later rule, so the reason pins
// Contract 6's refusal order.
func TestMergeRefusalsInContractOrder(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T, f *fixture) (id, sha string)
		reason string
	}{
		{"unknown task", func(t *testing.T, f *fixture) (string, string) {
			return "Z9", strings.Repeat("0", 40)
		}, "unknown task Z9"},
		{"tip moved", func(t *testing.T, f *fixture) (string, string) {
			old := f.commit(t, "gang/B2", "b.txt", "1\n", "")
			f.commit(t, "gang/B2", "b.txt", "2\n", "")
			return "B2", old
		}, "tip moved"},
		{"no branch", func(t *testing.T, f *fixture) (string, string) {
			return "B2", strings.Repeat("0", 40)
		}, "tip moved"},
		{"no Task line", func(t *testing.T, f *fixture) (string, string) {
			return "B2", f.commit(t, "gang/B2", "b.txt", "1\n", "")
		}, "no Task line"},
		{"Task line names another task", func(t *testing.T, f *fixture) (string, string) {
			return "B2", f.commit(t, "gang/B2", "b.txt", "1\n", "A1")
		}, "no Task line"},
		{"Task line not on a whole line", func(t *testing.T, f *fixture) (string, string) {
			return "B2", f.commitMsg(t, "gang/B2", "b.txt", "1\n", "B2 work\n\nTask: B2 and more\n  Task: B2")
		}, "no Task line"},
		{"Task line missing from a SHA already in the trunk", func(t *testing.T, f *fixture) (string, string) {
			trunk := git(t, f.repo, "rev-parse", integ)
			git(t, f.repo, "branch", "gang/B2", trunk)
			return "B2", trunk
		}, "no Task line"},
		{"no ledger row", func(t *testing.T, f *fixture) (string, string) {
			return "B2", f.commit(t, "gang/B2", "b.txt", "1\n", "B2")
		}, "ledger not CLEAN"},
		{"ledger has changes", func(t *testing.T, f *fixture) (string, string) {
			sha := f.commit(t, "gang/B2", "b.txt", "1\n", "B2")
			if err := ledger.Record(f.d, ledger.Row{Task: "B2", SHA: sha, Lens: ledger.LensReview, Majors: 1, GateRan: true, MutationChecked: true, Evidence: "e"}); err != nil {
				t.Fatal(err)
			}
			return "B2", sha
		}, "ledger not CLEAN"},
		{"adversarial lens missing on a risk-high task", func(t *testing.T, f *fixture) (string, string) {
			sha := f.commit(t, "gang/H3", "h.txt", "1\n", "H3")
			f.clean(t, "H3", sha, ledger.LensReview)
			return "H3", sha
		}, "ledger not CLEAN for %s: adversarial"},
		{"depends_on not merged", func(t *testing.T, f *fixture) (string, string) {
			sha := f.commit(t, "gang/B2", "b.txt", "1\n", "B2")
			f.clean(t, "B2", sha, ledger.LensReview)
			return "B2", sha
		}, "dependency not merged: A1"},
		{"merge_after not merged", func(t *testing.T, f *fixture) (string, string) {
			sha := f.commit(t, "gang/M4", "m.txt", "1\n", "M4")
			f.clean(t, "M4", sha, ledger.LensReview)
			return "M4", sha
		}, "dependency not merged: A1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			id, sha := tc.setup(t, f)
			before := git(t, f.repo, "rev-parse", integ)
			r := f.merge(t, id, sha)
			want := tc.reason
			if strings.Contains(want, "%s") {
				want = strings.Replace(want, "%s", sha, 1)
			}
			if r.Status != Refused || !strings.HasPrefix(r.Reason, want) {
				t.Fatalf("got %+v, want REFUSED with reason starting %q", r, want)
			}
			if r.TrunkSHA != before || r.Files == nil {
				t.Fatalf("refusal must report the unchanged trunk and an empty file list: %+v", r)
			}
			if after := git(t, f.repo, "rev-parse", integ); after != before {
				t.Fatal("a refused merge moved the integration branch")
			}
		})
	}
}

// Contract 0 rule 1: demo writes the Task line above a "Testing Done"
// paragraph, where it is not a git trailer.
func TestMergeAcceptsTaskLineThatIsNotATrailer(t *testing.T) {
	f := newFixture(t)
	a1 := f.commitMsg(t, "gang/A1", "a.txt", "a\n", "A1 work\n\nTask: A1\n\nTesting Done:\n- make check")
	if got := git(t, f.repo, "log", "-1", "--format=%(trailers:key=Task,valueonly)", a1); got != "" {
		t.Fatalf("fixture is a trailer after all: %q", got)
	}
	f.clean(t, "A1", a1, ledger.LensReview)
	if r := f.merge(t, "A1", a1); r.Status != Merged {
		t.Fatalf("%+v", r)
	}
}

// Contract 0 rule 2: the tip may be a sync merge with no Task line, as long
// as the range from the integration branch holds the task commit.
func TestMergeAcceptsSyncMergeTip(t *testing.T) {
	f := newFixture(t)
	f.commit(t, "gang/A1", "a.txt", "a\n", "A1")
	f.advance(t)
	git(t, f.repo, "switch", "-q", "gang/A1")
	git(t, f.repo, "merge", "-q", "--no-ff", "-m", "sync integration into gang/A1", "integration")
	git(t, f.repo, "switch", "-q", "main")
	tip := git(t, f.repo, "rev-parse", "refs/heads/gang/A1")
	if got := git(t, f.repo, "log", "-1", "--format=%B", tip); strings.Contains(got, "Task: ") {
		t.Fatalf("sync tip carries a Task line: %q", got)
	}
	f.clean(t, "A1", tip, ledger.LensReview)
	r := f.merge(t, "A1", tip)
	if r.Status != Merged {
		t.Fatalf("%+v", r)
	}
	if got := git(t, f.repo, "rev-list", "--parents", "-n", "1", r.TrunkSHA); !strings.HasSuffix(got, " "+tip) {
		t.Fatalf("parents %q, want the sync tip second", got)
	}
}

// The Task line must be in <integration>..<sha>. A line that already
// reached the integration branch does not cover a later commit.
func TestMergeRefusesRangeWithoutTaskLine(t *testing.T) {
	f := newFixture(t)
	a1 := f.commit(t, "gang/A1", "a.txt", "a\n", "A1")
	f.clean(t, "A1", a1, ledger.LensReview)
	if r := f.merge(t, "A1", a1); r.Status != Merged {
		t.Fatalf("first merge %+v", r)
	}
	later := f.commit(t, "gang/A1", "a.txt", "a2\n", "")
	f.clean(t, "A1", later, ledger.LensReview)
	before := git(t, f.repo, "rev-parse", integ)
	r := f.merge(t, "A1", later)
	if r.Status != Refused || !strings.HasPrefix(r.Reason, "no Task line") {
		t.Fatalf("%+v, want REFUSED no Task line", r)
	}
	if git(t, f.repo, "rev-parse", integ) != before {
		t.Fatal("a refused merge moved the integration branch")
	}
}

// Contract 0 rule 16: the engine's repair task F01 is not in plan.json. It
// merges like a risk-low task with no deps on branch gang/F01.
func TestMergeRepairTaskOutsideThePlan(t *testing.T) {
	f := newFixture(t)
	if _, inPlan := f.p.Task("F01"); inPlan {
		t.Fatal("fixture plan holds F01")
	}
	f01 := f.commit(t, "gang/F01", "file.txt", "repaired\n", "F01")
	f.clean(t, "F01", f01, ledger.LensReview)
	r := f.merge(t, "F01", f01)
	if r.Status != Merged {
		t.Fatalf("%+v", r)
	}
	if done, _ := gitx.TaskIDs(f.repo, integ); !reflect.DeepEqual(done, []string{"F01"}) {
		t.Fatalf("done set %v", done)
	}
	if r := f.merge(t, "F1", f01); r.Status != Refused || !strings.HasPrefix(r.Reason, "unknown task") {
		t.Fatalf("F1: %+v, want unknown task", r)
	}
}

// A SHA already in the integration branch is ALREADY only when the branch
// also holds its Task line. A claimed tip still at a trunk commit never
// reached the trunk as A1, even with a CLEAN ledger row.
func TestMergeRefusesTrunkSHAWithoutTaskLine(t *testing.T) {
	f := newFixture(t)
	trunk := git(t, f.repo, "rev-parse", integ)
	git(t, f.repo, "branch", "gang/A1", trunk)
	f.clean(t, "A1", trunk, ledger.LensReview)
	r := f.merge(t, "A1", trunk)
	if r.Status != Refused || !strings.HasPrefix(r.Reason, "no Task line") || r.TrunkSHA != trunk {
		t.Fatalf("got %+v, want REFUSED no Task line at %s", r, trunk)
	}
}

func TestMergeAlreadyMerged(t *testing.T) {
	f := newFixture(t)
	a1 := f.commit(t, "gang/A1", "a.txt", "a\n", "A1")
	f.clean(t, "A1", a1, ledger.LensReview)
	first := f.merge(t, "A1", a1)
	second := f.merge(t, "A1", a1)
	if second.Status != Already || second.TrunkSHA != first.TrunkSHA {
		t.Fatalf("second merge %+v, first %+v", second, first)
	}
	if got := git(t, f.repo, "rev-parse", integ); got != first.TrunkSHA {
		t.Fatal("ALREADY moved the integration branch")
	}
}

func TestMergeConflictListsFiles(t *testing.T) {
	f := newFixture(t)
	a1 := f.commit(t, "gang/A1", "file.txt", "from A1\n", "A1")
	c5 := f.commit(t, "gang/C5", "file.txt", "from C5\n", "C5")
	f.clean(t, "A1", a1, ledger.LensReview)
	f.clean(t, "C5", c5, ledger.LensReview)
	trunk := f.merge(t, "A1", a1).TrunkSHA
	r := f.merge(t, "C5", c5)
	if r.Status != Conflict || !reflect.DeepEqual(r.Files, []string{"file.txt"}) || r.TrunkSHA != trunk {
		t.Fatalf("got %+v", r)
	}
	if got := git(t, f.repo, "rev-parse", integ); got != trunk {
		t.Fatal("a conflict moved the integration branch")
	}
}

// man git-merge-tree: exit status 1 with no conflicted path is still a
// conflict. The trunk splits dir/ into x/ and y/ while A1 adds dir/c, and
// git cannot tell where dir/c belongs.
func TestMergeConflictWithoutAPath(t *testing.T) {
	f := newFixture(t)
	git(t, f.repo, "switch", "-q", "integration")
	for _, d := range []string{"dir", "x", "y"} {
		if err := os.MkdirAll(filepath.Join(f.repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"dir/a", "dir/b"} {
		if err := os.WriteFile(filepath.Join(f.repo, p), []byte(p+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, f.repo, "add", "dir")
	git(t, f.repo, "commit", "-q", "-m", "add dir")
	git(t, f.repo, "switch", "-q", "main")
	a1 := f.commit(t, "gang/A1", "dir/c", "c\n", "A1")
	git(t, f.repo, "switch", "-q", "integration")
	git(t, f.repo, "mv", "dir/a", "x/a")
	git(t, f.repo, "mv", "dir/b", "y/b")
	git(t, f.repo, "commit", "-q", "-m", "split dir")
	git(t, f.repo, "switch", "-q", "main")
	f.clean(t, "A1", a1, ledger.LensReview)

	before := git(t, f.repo, "rev-parse", integ)
	r := f.merge(t, "A1", a1)
	if r.Status != Conflict || len(r.Files) == 0 || r.TrunkSHA != before {
		t.Fatalf("got %+v, want CONFLICT at %s with a non-empty files list", r, before)
	}
	if got := git(t, f.repo, "rev-parse", integ); got != before {
		t.Fatal("a conflict moved the integration branch")
	}
}

func TestMergeLostCompareAndSwap(t *testing.T) {
	f := newFixture(t)
	a1 := f.commit(t, "gang/A1", "a.txt", "a\n", "A1")
	f.clean(t, "A1", a1, ledger.LensReview)
	var moved string
	beforeUpdateRef = func() {
		tip := git(t, f.repo, "rev-parse", integ)
		moved = git(t, f.repo, "commit-tree", tip+"^{tree}", "-p", tip, "-m", "someone else")
		git(t, f.repo, "update-ref", integ, moved, tip)
	}
	t.Cleanup(func() { beforeUpdateRef = nil })
	r := f.merge(t, "A1", a1)
	if r.Status != Conflict || r.Reason != "trunk moved" || r.TrunkSHA != moved || r.Files == nil || len(r.Files) != 0 {
		t.Fatalf("got %+v, want CONFLICT trunk moved at %s", r, moved)
	}
	if got := git(t, f.repo, "rev-parse", integ); got != moved {
		t.Fatal("the merge overwrote a trunk that moved")
	}
}
