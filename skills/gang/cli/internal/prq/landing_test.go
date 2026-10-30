package prq

import (
	"slices"
	"testing"

	"gang/internal/testkit"
)

func wantLandings(t *testing.T, repo string, want []Landing, wantStray ...string) {
	t.Helper()
	got, stray, err := Landings(repo, "main", "gang-demo")
	if err != nil || !slices.Equal(got, want) || !slices.Equal(stray, wantStray) {
		t.Fatalf("Landings = %+v, %q, %v; want %+v, %q", got, stray, err, want, wantStray)
	}
}

func TestLandingsReadsGangMerges(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.Git(t, repo, "branch", "gang-demo", "main")
	base := testkit.Git(t, repo, "rev-parse", "main")
	t1 := testkit.MergeTask(t, repo, "gang-demo", "T1", map[string]string{"a.txt": "a\n"})
	m1 := testkit.Git(t, repo, "rev-parse", "gang-demo")
	t2 := testkit.MergeTask(t, repo, "gang-demo", "T2", map[string]string{"b.txt": "b\n"})
	m2 := testkit.Git(t, repo, "rev-parse", "gang-demo")
	wantLandings(t, repo, []Landing{
		{"T1", m1, base, t1, "T1: work", "T1: work\n\nTask: T1\n\nTesting Done: unit tests"},
		{"T2", m2, m1, t2, "T2: work", "T2: work\n\nTask: T2\n\nTesting Done: unit tests"},
	})
}

// The task commit is the oldest in Parent..Tip with the merge's own id as a whole Task
// line. An older commit of that id on main lies before Parent, so it is out of range.
func TestLandingsTakesTheOldestTaskCommit(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.CommitOn(t, repo, "main", "DOC12: early\n\nTask: DOC12", map[string]string{"b.txt": "b\n"})
	testkit.Git(t, repo, "branch", "gang-demo", "main")
	base := testkit.Git(t, repo, "rev-parse", "main")
	testkit.CommitOn(t, repo, "gang/DOC12", "DOC12: other\n\nTask: T9", map[string]string{"c.txt": "c\n"})
	testkit.CommitOn(t, repo, "gang/DOC12", "DOC12: work\n\nTask: DOC12", map[string]string{"d.txt": "d\n"})
	tip := testkit.CommitOn(t, repo, "gang/DOC12", "DOC12: fix\n\nTask: DOC12", map[string]string{"e.txt": "e\n"})
	merge := testkit.GangMerge(t, repo, "gang-demo", "DOC12", "Build DOC12", tip)
	wantLandings(t, repo, []Landing{{"DOC12", merge, base, tip, "DOC12: work", "DOC12: work\n\nTask: DOC12"}})
}

func TestLandingsReadsOneParentTaskCommits(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.Git(t, repo, "branch", "gang-demo", "main")
	base := testkit.Git(t, repo, "rev-parse", "main")
	c := testkit.CommitOn(t, repo, "gang-demo", "feat: add c\n\nTask: DOC12\r\nTask: T4", map[string]string{"c.txt": "c\n"})
	wantLandings(t, repo, []Landing{{"DOC12", c, base, c, "feat: add c", "feat: add c\n\nTask: DOC12\r\nTask: T4"}})
}

func TestLandingsFailsOnAMissingBase(t *testing.T) {
	repo := testkit.Repo(t)
	if got, stray, err := Landings(repo, "nope", "main"); err == nil {
		t.Fatalf("Landings = %+v, %q, nil; want an error", got, stray)
	}
}

// Strays: near-miss Task lines, and merges with no Task line, a wrong tree, or a lowercase id.
func TestLandingsReportsStrays(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.Git(t, repo, "branch", "gang-demo", "main")
	loose := testkit.CommitOn(t, repo, "gang-demo", "chore: no task\n\nsee Task: T9\nTask: T9 soon", map[string]string{"d.txt": "d\n"})
	side := testkit.CommitOn(t, repo, "side", "side: work", map[string]string{"e.txt": "e\n"})
	bare := testkit.GangMerge(t, repo, "gang-demo", "T4", "Build T4", side)
	tip := testkit.CommitOn(t, repo, "gang/T5", "T5: work\n\nTask: T5", map[string]string{"f.txt": "f\n"})
	bad := testkit.Git(t, repo, "commit-tree", tip+"^{tree}", "-p", bare, "-p", tip, "-m", "merge T5: Build T5\n\nTask: T5")
	testkit.Git(t, repo, "update-ref", "refs/heads/gang-demo", bad)
	t6 := testkit.CommitOn(t, repo, "gang/T6", "T6: work\n\nTask: T6", map[string]string{"g.txt": "g\n"})
	plain := testkit.GangMerge(t, repo, "gang-demo", "t6", "remerge T6: lowercase id", t6)
	wantLandings(t, repo, nil, loose[:7]+" chore: no task", bare[:7]+" merge T4: Build T4", bad[:7]+" merge T5: Build T5", plain[:7]+" merge t6: remerge T6: lowercase id")
}
