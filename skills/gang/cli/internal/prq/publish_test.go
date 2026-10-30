package prq

import (
	"errors"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
	"gang/internal/testkit"
)

func queueFixture(t *testing.T) (repo string, d store.Dir, p *plan.Plan) {
	t.Helper()
	repo, p = testkit.Repo(t), testkit.Plan()
	testkit.Origin(t, repo)
	d = testkit.GangDir(t, repo, p)
	testkit.Git(t, repo, "branch", p.Repo.Integration, "main")
	testkit.MergeTask(t, repo, p.Repo.Integration, "T1", map[string]string{"a/T1.go": "package a\n"})
	return repo, d, p
}

func wantExit(t *testing.T, err error, code int, msg string) {
	t.Helper()
	var e *exit.Error
	if !errors.As(err, &e) || e.Code != code || !strings.Contains(e.Msg, msg) {
		t.Fatalf("err = %v, want exit %d containing %q", err, code, msg)
	}
}

// shipOnOrigin lands a commit on origin/main that only a fetch can see: the
// push moves the local tracking ref, so the ref goes back to where it was.
func shipOnOrigin(t *testing.T, repo, msg string, files map[string]string) {
	t.Helper()
	old := testkit.Git(t, repo, "rev-parse", "refs/remotes/origin/main")
	testkit.CommitOn(t, repo, "ship", msg, files)
	testkit.Git(t, repo, "push", "-q", "origin", "ship:main")
	testkit.Git(t, repo, "update-ref", "refs/remotes/origin/main", old)
}

func TestPrepareChecksTheQueue(t *testing.T) {
	for _, c := range []struct {
		name, id, refusal string // refusal "" means Prepare succeeds
		rows              []Row
	}{
		{"the next task", "T1", "", nil},
		{"a PR is open", "T2", "PR #7 for T1 is open", []Row{{Task: "T1", PR: 7, State: StateOpen}}},
		{"another task is next", "T2", "the next task to ship is T1, not T2", nil},
		{"nothing left", "T1", "nothing left to ship", []Row{{Task: "T1", PR: 7, State: StateMerged}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, d, p := queueFixture(t)
			for _, r := range c.rows {
				if err := Put(d, r); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Prepare(repo, d, p, c.id)
			if c.refusal != "" {
				wantExit(t, err, exit.Refused, c.refusal)
			} else if err != nil || got.ID != "T1" || got.Subject != "T1: work" || got.Branch != "gang/demo/T1" {
				t.Fatalf(`Prepare = %+v, %v, want T1, "T1: work", gang/demo/T1`, got, err)
			}
		})
	}
}

func TestPrepareReturnsAStrayError(t *testing.T) {
	repo, d, p := queueFixture(t)
	testkit.CommitOn(t, repo, p.Repo.Integration, "chore: no task", nil)
	if _, err := Prepare(repo, d, p, "T1"); err == nil || !strings.Contains(err.Error(), "chore: no task") {
		t.Fatalf("Prepare error = %v, want one that names the stray", err)
	}
}

// T2 is cut beside T1, as in a 0.4.0 wave. T1 ships on origin behind the local ref, so only a fetch skips it.
func TestPrepareReplaysOnlyTheTask(t *testing.T) {
	repo, d, p := queueFixture(t)
	tip := testkit.CommitOn(t, repo, "gang/T2", "T2: work\n\nTask: T2", map[string]string{"b/T2.go": "package b\n"})
	testkit.GangMerge(t, repo, p.Repo.Integration, "T2", "Build T2", tip)
	shipOnOrigin(t, repo, "feat: ship t1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n", "x.txt": "x\n"})
	c, err := Prepare(repo, d, p, "T2")
	if err != nil {
		t.Fatal(err)
	}
	shipped := testkit.Git(t, repo, "rev-parse", "ship")
	for name, v := range map[string][2]string{
		"base":    {c.Base, shipped},
		"parents": {testkit.Git(t, repo, "rev-list", "--parents", "-n", "1", c.SHA), c.SHA + " " + shipped},
		"tree":    {testkit.Git(t, repo, "ls-tree", "-r", "--name-only", c.SHA), "README.md\na/T1.go\nb/T2.go\nx.txt"},
		"message": {testkit.Git(t, repo, "log", "-1", "--format=%B", c.SHA), c.Message},
	} {
		if v[0] != v[1] {
			t.Errorf("%s = %q, want %q", name, v[0], v[1])
		}
	}
}

func TestPrepareRefusesWhatCannotShip(t *testing.T) {
	for _, c := range []struct {
		name, id string
		code     int
		msg      string
		setup    func(t *testing.T, repo string, d store.Dir)
	}{
		{"already on the base", "T1", exit.Refused, "T1 is already on origin/main", func(t *testing.T, repo string, _ store.Dir) {
			shipOnOrigin(t, repo, "chore: same a", map[string]string{"a/T1.go": "package a\n"})
		}},
		{"conflict", "T1", exit.Conflict, "T1 does not replay onto origin/main: a/T1.go", func(t *testing.T, repo string, _ store.Dir) {
			shipOnOrigin(t, repo, "chore: other a", map[string]string{"a/T1.go": "package other\n"})
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, d, p := queueFixture(t)
			c.setup(t, repo, d)
			_, err := Prepare(repo, d, p, c.id)
			wantExit(t, err, c.code, c.msg)
		})
	}
}

func TestPrepareStopsWhenTheFetchFails(t *testing.T) {
	repo, d, p := queueFixture(t)
	testkit.Git(t, repo, "remote", "set-url", "origin", t.TempDir()+"/gone.git")
	if _, err := Prepare(repo, d, p, "T1"); err == nil || !strings.HasPrefix(err.Error(), "fetch origin: ") || errors.As(err, new(*exit.Error)) {
		t.Fatalf("Prepare error = %v, want one that starts with %q", err, "fetch origin: ")
	}
}
