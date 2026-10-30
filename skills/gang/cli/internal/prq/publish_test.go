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
