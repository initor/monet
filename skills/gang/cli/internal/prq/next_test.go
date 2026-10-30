package prq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/store"
	"gang/internal/testkit"
)

// nextTrunk cuts gang-demo, one commit per task id, from an origin/main that is ahead of the local main.
func nextTrunk(t *testing.T, ids ...string) string {
	t.Helper()
	repo := testkit.Repo(t)
	testkit.Origin(t, repo)
	testkit.CommitRef(t, repo, "refs/remotes/origin/main", "chore: merged elsewhere", map[string]string{"o.txt": "o\n"})
	testkit.Git(t, repo, "branch", "gang-demo", "refs/remotes/origin/main")
	for _, id := range ids {
		testkit.CommitOn(t, repo, "gang-demo", "feat: add "+id+"\n\nTask: "+id, map[string]string{id + ".txt": id + "\n"})
	}
	return repo
}

func wantNext(t *testing.T, repo, want string, rows ...Row) { // want "" means no landing is left
	t.Helper()
	d := store.Dir{Root: t.TempDir()}
	if err := write(d, rows); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := Next(repo, d, testkit.Plan()); err != nil || got.ID != want || ok != (want != "") {
		t.Fatalf("Next = %q, %v, %v; want %q", got.ID, ok, err, want)
	}
}

func TestNextSkipsShippedTasks(t *testing.T) {
	repo := nextTrunk(t, "T1", "T2")
	testkit.CommitRef(t, repo, "refs/remotes/origin/main", "feat: ship t1\n\nTask: T1", map[string]string{"x.txt": "x\n"})
	wantNext(t, repo, "T2")
}

func TestNextSkipsOpenAndMergedRows(t *testing.T) {
	repo := nextTrunk(t, "T1", "T2", "T3")
	merged, open := Row{Task: "T1", State: StateMerged}, Row{Task: "T2", State: StateOpen}
	wantNext(t, repo, "T3", merged, open)
	wantNext(t, repo, "", merged, open, Row{Task: "T3", State: StateMerged})
	wantNext(t, repo, "T1", Row{Task: "T1", State: StateClosed})
}

// A hand squash of T1 and F01 drops their Task lines, so only the replay shows that F01 is on the base.
func TestNextSkipsALandingAlreadyOnTheBase(t *testing.T) {
	for _, c := range []struct{ name, f01, want string }{
		{"same change", "F01\n", "T2"},
		{"replay conflict", "other\n", "F01"},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo := nextTrunk(t, "T1", "F01", "T2")
			testkit.CommitRef(t, repo, "refs/remotes/origin/main", "feat: squash t1 and f01", map[string]string{"T1.txt": "T1\n", "F01.txt": c.f01})
			wantNext(t, repo, c.want, Row{Task: "T1", State: StateMerged})
		})
	}
}

// T2 shipped after the hand squash and reverts F01's line, so F01's replay would re-apply it.
func TestNextSkipsLandingsBeforeTheLastShippedOne(t *testing.T) {
	repo := nextTrunk(t, "T1")
	testkit.CommitOn(t, repo, "gang-demo", "fix: F01\n\nTask: F01", map[string]string{"T1.txt": "T1 fixed\n"})
	testkit.CommitOn(t, repo, "gang-demo", "feat: add T2\n\nTask: T2", map[string]string{"T1.txt": "T1\n"})
	testkit.CommitRef(t, repo, "refs/remotes/origin/main", "feat: squash t1 and f01", map[string]string{"T1.txt": "T1 fixed\n"})
	testkit.CommitRef(t, repo, "refs/remotes/origin/main", "feat: t2 (#2)\n\nTask: T2", map[string]string{"T1.txt": "T1\n"})
	wantNext(t, repo, "", Row{Task: "T1", State: StateMerged})
}

func TestNextFailsWhenAReplayFails(t *testing.T) {
	repo := nextTrunk(t, "T1")
	tree := testkit.Git(t, repo, "rev-parse", "gang-demo^{tree}")
	if err := os.Remove(filepath.Join(repo, ".git/objects", tree[:2], tree[2:])); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := Next(repo, store.Dir{Root: t.TempDir()}, testkit.Plan()); err == nil {
		t.Fatalf("Next = %+v, %v, nil; want the replay error", got, ok)
	}
}

func TestNextFailsOnAStray(t *testing.T) {
	repo := nextTrunk(t, "T1")
	testkit.CommitOn(t, repo, "gang-demo", "chore: no task", map[string]string{"d.txt": "d\n"})
	testkit.CommitOn(t, repo, "gang-demo", "docs: no task either", map[string]string{"e.txt": "e\n"})
	_, _, err := Next(repo, store.Dir{Root: t.TempDir()}, testkit.Plan())
	if err == nil || !strings.Contains(err.Error(), "chore: no task") || !strings.Contains(err.Error(), "docs: no task either") {
		t.Fatalf("Next error = %v; want one that names both strays", err)
	}
}

func TestNextFailsWithoutTheBase(t *testing.T) {
	if got, ok, err := Next(testkit.Repo(t), store.Dir{Root: t.TempDir()}, testkit.Plan()); err == nil {
		t.Fatalf("Next = %+v, %v, nil; want an error", got, ok)
	}
}
