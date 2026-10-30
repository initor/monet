package prq

import (
	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
)

// Prepared is the next trunk task, checked for gang pr open.
type Prepared struct {
	Landing
	Branch string // gang/<slug>/<id>
}

// Prepare checks that task id is the one to ship now: no PR is open, and
// Next returns id. It pushes nothing and records nothing.
func Prepare(repo string, d store.Dir, p *plan.Plan, id string) (Prepared, error) {
	rows, err := Read(d)
	if err != nil {
		return Prepared{}, err
	}
	if r, ok := Current(rows); ok {
		return Prepared{}, exit.New(exit.Refused, "PR #%d for %s is open: one PR at a time", r.PR, r.Task)
	}
	l, ok, err := Next(repo, d, p)
	switch {
	case err != nil:
		return Prepared{}, err
	case !ok:
		return Prepared{}, exit.New(exit.Refused, "nothing left to ship: each trunk task is on origin/%s or has a PR", p.Repo.Base)
	case l.ID != id:
		return Prepared{}, exit.New(exit.Refused, "the next task to ship is %s, not %s", l.ID, id)
	}
	return Prepared{Landing: l, Branch: "gang/" + p.Slug + "/" + id}, nil
}
