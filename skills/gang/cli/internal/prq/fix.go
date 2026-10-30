package prq

import (
	"regexp"
	"slices"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
)

// FixLanding checks that fixID can go onto the open PR of task id, and returns its row and landing. It pushes and records nothing.
func FixLanding(repo string, d store.Dir, p *plan.Plan, id, fixID string) (Row, Landing, error) {
	r, _, err := Live(d, p, id)
	if err != nil {
		return Row{}, Landing{}, err
	}
	if r.Fixes >= 2 {
		return Row{}, Landing{}, exit.New(exit.Refused, "PR #%d of %s has 2 fix commits, the most a PR takes: the queue stops for the checkpoint", r.PR, id)
	}
	landings, _, err := Landings(repo, "refs/remotes/origin/"+p.Repo.Base, "refs/heads/"+p.Repo.Integration)
	i := slices.IndexFunc(landings, func(l Landing) bool { return l.ID == fixID })
	switch {
	case err != nil:
		return Row{}, Landing{}, err
	case i < 0:
		return Row{}, Landing{}, exit.New(exit.Refused, "%s is not a task commit on %s above origin/%s", fixID, p.Repo.Integration, p.Repo.Base)
	case !regexp.MustCompile(`(?m)^Fixes: ` + regexp.QuoteMeta(id) + `$`).MatchString(landings[i].Message):
		return Row{}, Landing{}, exit.New(exit.Refused, "%s does not fix %s: its task commit has no line \"Fixes: %s\"", fixID, id, id)
	}
	if _, err := reviewed(d, p, fixID, landings[i].Tip); err != nil {
		return Row{}, Landing{}, err
	}
	return r, landings[i], nil
}
