package prq

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/store"
)

// Next returns the first landing that the queue has yet to ship: its task has no
// "Task: <id>" line on origin/<base>, no open or merged row, no shipped landing
// after it, and a replay onto origin/<base> that changes something. A stray is an error.
func Next(repo string, d store.Dir, p *plan.Plan) (Landing, bool, error) {
	base := "refs/remotes/origin/" + p.Repo.Base
	landings, stray, err := Landings(repo, base, "refs/heads/"+p.Repo.Integration)
	if err != nil {
		return Landing{}, false, err
	}
	if len(stray) > 0 {
		return Landing{}, false, fmt.Errorf("trunk has commits that are not task landings, so replays would leave their changes out: %s", strings.Join(stray, ", "))
	}
	shipped, err1 := gitx.TaskIDs(repo, base)
	rows, err2 := Read(d)
	if err := errors.Join(err1, err2); err != nil {
		return Landing{}, false, err
	}
	tip, err := gitx.RevParse(repo, base)
	if err != nil {
		return Landing{}, false, err
	}
	last := -1 // the queue ships in trunk order, so each landing up to the last shipped one is shipped
	for i, l := range landings {
		queued := func(r Row) bool { return r.Task == l.ID && (r.State == StateOpen || r.State == StateMerged) }
		if slices.Contains(shipped, l.ID) || slices.ContainsFunc(rows, queued) {
			last = i
		}
	}
	for _, l := range landings[last+1:] {
		// A hand squash drops the Task lines, so a replay that changes nothing is shipped too.
		_, _, same, _, err := replayOnto(repo, l, tip)
		if err != nil {
			return Landing{}, false, err
		}
		if !same {
			return l, true, nil
		}
	}
	return Landing{}, false, nil
}
