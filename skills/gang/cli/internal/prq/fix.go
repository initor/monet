package prq

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/repair"
	"gang/internal/size"
	"gang/internal/store"
	"gang/internal/wave"
)

// FixLanding checks that fixID can go onto the open PR of task id, and returns
// the open row and the fix's trunk landing. It pushes and records nothing.
func FixLanding(repo string, d store.Dir, p *plan.Plan, id, fixID string) (Row, Landing, error) {
	r, _, err := Live(d, p, id)
	if err != nil {
		return Row{}, Landing{}, err
	}
	if r.Fixes >= 2 {
		return Row{}, Landing{}, exit.New(exit.Refused, "PR #%d of %s has 2 fix commits, the most a PR takes: the queue stops for the checkpoint", r.PR, id)
	}
	landings, _, err := Landings(repo, "refs/remotes/origin/"+p.Repo.Base, "refs/heads/"+p.Repo.Integration)
	if err != nil {
		return Row{}, Landing{}, err
	}
	i := slices.IndexFunc(landings, func(l Landing) bool { return l.ID == fixID })
	switch {
	case i < 0:
		return Row{}, Landing{}, exit.New(exit.Refused, "%s is not a task commit on %s above origin/%s", fixID, p.Repo.Integration, p.Repo.Base)
	case !regexp.MustCompile(`(?m)^Fixes: ` + regexp.QuoteMeta(id) + `\r?$`).MatchString(landings[i].Message):
		return Row{}, Landing{}, exit.New(exit.Refused, "%s does not fix %s: its task commit has no line \"Fixes: %s\"", fixID, id, id)
	}
	if _, err := reviewed(d, p, fixID, landings[i].Tip); err != nil {
		return Row{}, Landing{}, err
	}
	return r, landings[i], nil
}

// Fix replays fix task fixID onto the open PR of task id as one new commit,
// pushes it as a fast-forward, and records the new head and one more fix. It
// returns the new row.
// A fix that takes the whole PR above limits.split_lines is refused, and so is one
// above limits.task_lines unless reason or the size.reason of the task says why.
func Fix(repo string, d store.Dir, p *plan.Plan, id, fixID, reason string) (Row, error) {
	r, l, err := FixLanding(repo, d, p, id, fixID)
	if err != nil {
		return Row{}, err
	}
	sha, conflicts, err := gitx.Replay(repo, l.Commit, l.Parent, r.HeadSHA, l.Message)
	switch {
	case err != nil:
		return Row{}, err
	case len(conflicts) > 0:
		return Row{}, exit.New(exit.Conflict, "%s does not replay onto PR #%d: %s", fixID, r.PR, strings.Join(conflicts, ", "))
	}
	trees, err := gitx.Run(repo, "rev-parse", sha+"^{tree}", r.HeadSHA+"^{tree}")
	if err != nil {
		return Row{}, err
	}
	if replay, head, _ := strings.Cut(trees, "\n"); replay == head {
		return Row{}, exit.New(exit.Refused, "%s is already in PR #%d: its replay changes nothing", fixID, r.PR)
	}
	if err := wave.Scan(repo, r.HeadSHA, sha); err != nil {
		return Row{}, err
	}
	base, err := gitx.Run(repo, "merge-base", "refs/remotes/origin/"+p.Repo.Base, sha)
	if err != nil {
		return Row{}, err
	}
	n, err := size.Count(repo, base, sha, size.DefaultExclude)
	if err != nil {
		return Row{}, err
	}
	if t, ok := repair.TaskFor(p, id); ok && strings.TrimSpace(reason) == "" {
		reason = t.Size.Reason
	}
	switch {
	case n > p.Limits.SplitLines:
		return Row{}, exit.New(exit.Refused, "PR #%d would have %d changed lines after %s, above limits.split_lines %d: the queue stops, and the checkpoint splits %s", r.PR, n, fixID, p.Limits.SplitLines, id)
	case n > p.Limits.TaskLines && strings.TrimSpace(reason) == "":
		return Row{}, exit.New(exit.Refused, "PR #%d would have %d changed lines after %s, above limits.task_lines %d: pass --reason <why>, and record it as a ruling", r.PR, n, fixID, p.Limits.TaskLines)
	}
	if _, err := gitx.Run(repo, "push", "--quiet", "origin", sha+":refs/heads/"+r.Branch); err != nil {
		if out, lsErr := gitx.Run(repo, "ls-remote", "origin", "refs/heads/"+r.Branch); lsErr != nil || !strings.HasPrefix(out, sha+"\t") { // a push that errors may still have landed
			return Row{}, exit.New(exit.Refused, "push rejected: %v", err)
		}
	}
	rows, err := Read(d) // again: the push took a while, and the file may have changed
	if err != nil {
		return Row{Branch: r.Branch, HeadSHA: sha}, err
	}
	i := slices.IndexFunc(rows, func(x Row) bool { return x.Task == id })
	if i < 0 {
		return Row{}, fmt.Errorf("prs.tsv has no row for %s: PR #%d is pushed at %s", id, r.PR, sha)
	}
	r = rows[i]
	r.HeadSHA, r.Fixes = sha, r.Fixes+1
	return r, Put(d, r)
}
