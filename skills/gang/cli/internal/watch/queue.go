package watch

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gang/internal/gh"
	"gang/internal/plan"
	"gang/internal/prq"
	"gang/internal/store"
)

var ghStates = map[string]string{"OPEN": prq.StateOpen, "MERGED": prq.StateMerged, "CLOSED": prq.StateClosed}

// queue is the PR queue part of one watch pass. With repo.gh_account set, it
// sets each prs.tsv row that is not merged to the state GitHub reports, with
// prq.SetState, and emits PR_GREEN or PR_RED for an open PR. It returns the gh
// errors as soft, and the errors that end the pass as hard.
func queue(repo string, d store.Dir, p *plan.Plan, now time.Time, emit func(line, key string, since time.Time)) (soft, hard error) {
	if p.Repo.GHAccount == "" {
		return nil, nil
	}
	rows, err := prq.Read(d)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.State == prq.StateMerged {
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
		line := fmt.Sprintf("PR_GREEN %s %d", r.Task, r.PR)
		if red {
			line = fmt.Sprintf("PR_RED %s %d %s", r.Task, r.PR, strings.Join(failed, ","))
		}
		if ps.State == "OPEN" && (green || red) {
			emit(line, line+" @"+ps.HeadSHA, time.Time{})
		}
	}
	return soft, nil
}
