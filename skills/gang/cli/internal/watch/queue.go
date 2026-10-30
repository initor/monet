package watch

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gang/internal/gh"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/prq"
	"gang/internal/store"
)

var ghStates = map[string]string{"OPEN": prq.StateOpen, "MERGED": prq.StateMerged, "CLOSED": prq.StateClosed}

// queue is the PR queue part of one watch pass. With repo.gh_account set, it
// sets each prs.tsv row that is not merged to the state GitHub reports, with
// prq.SetState, and emits PR_GREEN or PR_RED for an open PR, red too for no
// checks past the grace. With no gh error or open row, it emits PR_NEXT if
// origin/<base> exists. It returns gh and prq.Next errors as soft, the rest hard.
func queue(repo string, d store.Dir, p *plan.Plan, now time.Time, emit func(line, key string, since time.Time)) (soft, hard error) {
	rows, err := prq.Read(d)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.State == prq.StateMerged || p.Repo.GHAccount == "" {
			continue
		}
		ps, err := gh.PRState(p.Repo.Remote, r.PR)
		if err != nil {
			soft = errors.Join(soft, err)
			continue
		}
		if next, ok := ghStates[ps.State]; ok && next != r.State {
			if err := prq.SetState(d, r.Task, next); err != nil {
				return soft, err
			}
		}
		green, red, failed := gh.Classify(ps)
		if len(ps.Checks) == 0 && p.Repo.Checks == plan.ChecksGitHub && ps.HeadSHA == r.HeadSHA {
			ct, _ := gitx.Run(repo, "show", "-s", "--format=%cI", "--end-of-options", r.HeadSHA) // gang's own head: CI could start at its commit time. No time if unreadable.
			if at, err := time.Parse(time.RFC3339, ct); err == nil && now.Sub(at) >= time.Duration(p.Limits.ChecksGraceMin)*time.Minute {
				red, failed = true, []string{"no-checks"}
			}
		}
		line := fmt.Sprintf("PR_GREEN %s %d", r.Task, r.PR)
		if red {
			line = fmt.Sprintf("PR_RED %s %d %s", r.Task, r.PR, strings.Join(failed, ","))
		}
		if ps.State == "OPEN" && (green || red) {
			emit(line, line+" @"+ps.HeadSHA, time.Time{})
		}
	}
	rows, err = prq.Read(d)
	if _, open := prq.Current(rows); open || soft != nil || err != nil {
		return soft, err
	}
	if _, err := gitx.RevParse(repo, "refs/remotes/origin/"+p.Repo.Base); errors.Is(err, gitx.ErrNotFound) {
		return nil, nil
	}
	l, ok, err := prq.Next(repo, d, p)
	if ok {
		emit("PR_NEXT "+l.ID, "PR_NEXT "+l.ID, time.Time{})
	}
	return err, nil
}
