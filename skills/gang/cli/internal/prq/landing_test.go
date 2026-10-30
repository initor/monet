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

func TestLandingsReadsOneParentTaskCommits(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.Git(t, repo, "branch", "gang-demo", "main")
	base := testkit.Git(t, repo, "rev-parse", "main")
	c := testkit.CommitOn(t, repo, "gang-demo", "feat: add c\n\nTask: T3\nTask: T4", map[string]string{"c.txt": "c\n"})
	wantLandings(t, repo, []Landing{{"T3", c, base, c, "feat: add c", "feat: add c\n\nTask: T3\nTask: T4"}})
}

func TestLandingsFailsOnAMissingBase(t *testing.T) {
	repo := testkit.Repo(t)
	if got, stray, err := Landings(repo, "nope", "main"); err == nil {
		t.Fatalf("Landings = %+v, %q, nil; want an error", got, stray)
	}
}

// Strays: near-miss Task lines, and a merge commit with a Task line of its own.
func TestLandingsReportsStrays(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.Git(t, repo, "branch", "gang-demo", "main")
	loose := testkit.CommitOn(t, repo, "gang-demo", "chore: no task\n\nsee Task: T9\nTask: T9 soon", map[string]string{"d.txt": "d\n"})
	side := testkit.CommitOn(t, repo, "side", "side: work", map[string]string{"e.txt": "e\n"})
	merge := testkit.Git(t, repo, "commit-tree", side+"^{tree}", "-p", loose, "-p", side, "-m", "Merge side\n\nTask: T5")
	testkit.Git(t, repo, "update-ref", "refs/heads/gang-demo", merge)
	wantLandings(t, repo, nil, loose[:7]+" chore: no task", merge[:7]+" Merge side")
}
