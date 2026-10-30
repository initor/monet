package prq

import (
	"fmt"
	"strings"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/store"
)

// Prepared is the next trunk task, checked for gang pr open.
type Prepared struct {
	Landing
	Branch string // gang/<slug>/<id>
	Base   string // the origin/<base> commit that the replay starts from
	SHA    string // the replayed commit: one parent, Base
}

// Prepare checks that task id is the one to ship now: no PR is open, and Next
// returns id once origin is fetched. It replays that task alone onto the
// fetched origin/<base>. It pushes nothing and records nothing.
func Prepare(repo string, d store.Dir, p *plan.Plan, id string) (Prepared, error) {
	rows, err := Read(d)
	if err != nil {
		return Prepared{}, err
	}
	if r, ok := Current(rows); ok {
		return Prepared{}, exit.New(exit.Refused, "PR #%d for %s is open: one PR at a time", r.PR, r.Task)
	}
	if _, err := gitx.Run(repo, "fetch", "--quiet", "origin"); err != nil {
		return Prepared{}, fmt.Errorf("fetch origin: %w", err)
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
	c := Prepared{Landing: l, Branch: "gang/" + p.Slug + "/" + id}
	if c.Base, err = gitx.RevParse(repo, "refs/remotes/origin/"+p.Repo.Base); err != nil {
		return Prepared{}, err
	}
	sha, conflicts, err := gitx.Replay(repo, l.Commit, l.Parent, c.Base, l.Message)
	switch {
	case err != nil:
		return Prepared{}, err
	case len(conflicts) > 0:
		return Prepared{}, exit.New(exit.Conflict, "%s does not replay onto origin/%s: %s", id, p.Repo.Base, strings.Join(conflicts, ", "))
	}
	trees, err := gitx.Run(repo, "rev-parse", sha+"^{tree}", c.Base+"^{tree}")
	if err != nil {
		return Prepared{}, err
	}
	if replay, base, _ := strings.Cut(trees, "\n"); replay == base {
		return Prepared{}, exit.New(exit.Refused, "%s is already on origin/%s: its replay changes nothing", id, p.Repo.Base)
	}
	c.SHA = sha
	return c, nil
}
