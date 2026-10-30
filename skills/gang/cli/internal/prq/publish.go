package prq

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/ledger"
	"gang/internal/plan"
	"gang/internal/repair"
	"gang/internal/size"
	"gang/internal/store"
	"gang/internal/wave"
)

// Prepared is the next trunk task, checked for gang pr open.
type Prepared struct {
	Landing
	Branch string // gang/<slug>/<id>
	Base   string // the origin/<base> commit that the replay starts from
	SHA    string // the PR head: the replay, or the equal branch that pr open pushed

	Task *plan.Task // the plan task, or the synthetic repair task of an F<nn> id
}

// Prepare checks that task id is the one to ship now: no PR is open, Next
// returns id once origin is fetched, id is in the plan or an F<nn> repair id,
// and its tip is ledger CLEAN. It replays that task alone onto the fetched
// origin/<base>, scans the PR head for secrets, and pushes and records nothing.
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
	if c.Task, err = reviewed(d, p, id, l.Tip); err != nil {
		return Prepared{}, err
	}
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
	replay, base, _ := strings.Cut(trees, "\n")
	if replay == base {
		return Prepared{}, exit.New(exit.Refused, "%s is already on origin/%s: its replay changes nothing", id, p.Repo.Base)
	}
	c.SHA = sha
	if err := c.reuse(repo, replay); err != nil {
		return Prepared{}, err
	}
	if err := wave.Scan(repo, c.Base, c.SHA); err != nil {
		return Prepared{}, err
	}
	return c, nil
}

// reuse sets c.SHA to origin/<c.Branch> when that branch exists and holds the replay's tree on c.Base, and refuses any other tip.
func (c *Prepared) reuse(repo, tree string) error {
	tip, err := gitx.RevParse(repo, "refs/remotes/origin/"+c.Branch)
	if errors.Is(err, gitx.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	got, err := gitx.Run(repo, "rev-parse", tip+"^{tree}", tip+"^@")
	switch {
	case err != nil:
		return err
	case got != tree+"\n"+c.Base:
		return exit.New(exit.Refused, "origin/%s exists at %s with another change or base: gang never moves it", c.Branch, tip[:7])
	}
	c.SHA = tip
	return nil
}

// reviewed returns the task of id, and refuses an id that is neither in the
// plan nor an F<nn> repair id, and a tip that is not ledger CLEAN.
func reviewed(d store.Dir, p *plan.Plan, id, tip string) (*plan.Task, error) {
	t, ok := repair.TaskFor(p, id)
	if !ok {
		return nil, exit.New(exit.Refused, "%s is not a task of the plan", id)
	}
	clean, missing, err := ledger.Check(d, id, tip, ledger.RequiredLenses(t))
	switch {
	case err != nil:
		return nil, err
	case !clean:
		return nil, exit.New(exit.Refused, "%s tip %s is not ledger CLEAN: missing %v", id, tip[:7], missing)
	}
	return t, nil
}

// Body is the PR text of c: the task and its goal, its requirement ids, the
// lenses that reviewed its tip CLEAN, and the changed lines of the replay.
func Body(repo string, p *plan.Plan, c Prepared) (string, error) {
	n, err := size.Count(repo, c.Base, c.SHA, size.DefaultExclude)
	if err != nil {
		return "", err
	}
	reqs := cmp.Or(strings.Join(c.Task.Requirements, ", "), "-")
	return fmt.Sprintf("Task %s of gang run `%s`: %s\n\n- Requirements: %s\n- Review: CLEAN %v at %s\n- Changed lines: %d\n\nOpened by `gang pr open`.\n",
		c.ID, p.Slug, c.Task.Goal, reqs, ledger.RequiredLenses(c.Task), c.Tip[:7], n), nil
}
