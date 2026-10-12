package main

import (
	"strings"
	"testing"
	"time"

	"gang/internal/plan"
)

// Helpers for plan 02 command tests, on top of plan 01's runGang,
// newTestRepo, newTestGangDir, and gitT. The rs prefix keeps them apart.

var rsNow = time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

func rsCtx(dir, repo string) *Ctx {
	return &Ctx{Dir: dir, Repo: repo, Stdin: strings.NewReader(""), Now: func() time.Time { return rsNow }}
}

func rsTask(id string) plan.Task {
	return plan.Task{ID: id, Title: "Task " + id, Goal: "Do " + id + ".", Risk: "low", Wave: 1}
}

func rsPlan(tasks ...plan.Task) *plan.Plan {
	p := &plan.Plan{
		Version: 1,
		Slug:    "toy",
		Repo:    plan.Repo{Remote: "github.com/example/toy", Base: "main", Integration: "integration", GHAccount: "example"},
		Tasks:   tasks,
	}
	p.Normalize()
	return p
}

// rsSetup makes a repo, the gang dir for p, and the integration branch at main.
func rsSetup(t *testing.T, p *plan.Plan) (repo, dir string) {
	t.Helper()
	repo = newTestRepo(t)
	dir = newTestGangDir(t, repo, p)
	ref := "refs/heads/" + p.Repo.Integration
	if gitT(t, repo, "for-each-ref", "--format=%(objectname)", ref) == "" {
		gitT(t, repo, "update-ref", ref, gitT(t, repo, "rev-parse", "refs/heads/main"))
	}
	return repo, dir
}
