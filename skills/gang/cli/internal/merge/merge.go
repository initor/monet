// Package merge lands one reviewed task tip on the integration branch
// without a checkout: merge-tree, commit-tree, then a compare-and-swap
// update-ref. It trusts only git and the ledger.
package merge

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gang/internal/gitx"
	"gang/internal/ledger"
	"gang/internal/plan"
	"gang/internal/repair"
	"gang/internal/size"
	"gang/internal/store"
)

type Status string

const (
	Merged   Status = "MERGED"
	Already  Status = "ALREADY"
	Conflict Status = "CONFLICT"
	Refused  Status = "REFUSED"
)

// Result is Contract 6.
type Result struct {
	Status   Status   `json:"status"`
	TrunkSHA string   `json:"trunk_sha"`
	Files    []string `json:"files"`
	Reason   string   `json:"reason"`
}

// beforeUpdateRef runs just before the compare-and-swap. Tests use it to
// move the integration branch under the merge.
var beforeUpdateRef func()

// taskLine is Contract 0 rule 1: a whole line, not necessarily a trailer.
var taskLine = regexp.MustCompile(`^Task: ([A-Z]+[0-9]+)$`)

// Merge refuses, in Contract 6 order: unknown task (repair.TaskFor), tip differs from sha, no
// commit in <integration>..<sha> with the line "Task: <id>" (Contract 0
// rule 2), ledger not CLEAN for sha, a depends_on or merge_after task not
// merged. Then it reports ALREADY or CONFLICT. A merge that changes more
// lines on the trunk than size.Check allows is refused last, and any other
// merge is MERGED. A sha that is already in the integration branch has an
// empty range, so its Task line must be in the integration branch's history
// instead. The returned error is for git or file failures only.
func Merge(repo string, d store.Dir, p *plan.Plan, id, sha string) (Result, error) {
	ref := "refs/heads/" + p.Repo.Integration
	trunk, err := gitx.RevParse(repo, ref)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", ref, err)
	}
	trunk = strings.TrimSpace(trunk)
	refuse := func(format string, args ...any) (Result, error) {
		return Result{Status: Refused, TrunkSHA: trunk, Files: []string{}, Reason: fmt.Sprintf(format, args...)}, nil
	}

	t, ok := repair.TaskFor(p, id)
	if !ok {
		return refuse("unknown task %s", id)
	}
	branch := plan.BranchFor(t)
	tip, err := gitx.RevParse(repo, "refs/heads/"+branch)
	if err != nil {
		return refuse("tip moved: branch %s does not exist, expected %s", branch, sha)
	}
	if tip = strings.TrimSpace(tip); tip != sha {
		return refuse("tip moved: %s is at %s, not %s", branch, tip, sha)
	}

	done, err := gitx.TaskIDs(repo, trunk)
	if err != nil {
		return Result{}, err
	}
	merged := map[string]bool{}
	for _, doneID := range done {
		merged[doneID] = true
	}

	already, err := gitx.IsAncestor(repo, sha, trunk)
	if err != nil {
		return Result{}, err
	}
	if already {
		if !merged[id] {
			return refuse("no Task line \"Task: %s\" in %s, which already contains %s", id, trunk, sha)
		}
	} else {
		msgs, err := gitx.Run(repo, "log", "--format=%B%x00", trunk+".."+sha)
		if err != nil {
			return Result{}, err
		}
		if !hasTaskLine(msgs, id) {
			return refuse("no Task line \"Task: %s\" in %s..%s", id, trunk, sha)
		}
	}

	clean, missing, err := ledger.Check(d, id, sha, ledger.RequiredLenses(t))
	if err != nil {
		return Result{}, err
	}
	if !clean {
		names := make([]string, len(missing))
		for i, l := range missing {
			names[i] = string(l)
		}
		return refuse("ledger not CLEAN for %s: %s", sha, strings.Join(names, ","))
	}

	for _, dep := range append(append([]string{}, t.DependsOn...), t.MergeAfter...) {
		if !merged[dep] {
			return refuse("dependency not merged: %s", dep)
		}
	}

	if already {
		return Result{Status: Already, TrunkSHA: trunk, Files: []string{}}, nil
	}

	tree, conflicts, err := gitx.MergeTree(repo, trunk, sha)
	if err != nil {
		return Result{}, err
	}
	if len(conflicts) > 0 {
		return Result{Status: Conflict, TrunkSHA: trunk, Files: conflicts, Reason: "merge conflict"}, nil
	}
	n, err := size.Count(repo, trunk, strings.TrimSpace(tree), size.DefaultExclude)
	if err != nil {
		return Result{}, err
	}
	if why := size.Check(p.Limits, t.Size.Reason, n); why != "" {
		return refuse("%s", why)
	}
	commit, err := gitx.CommitTree(repo, strings.TrimSpace(tree), []string{trunk, sha}, fmt.Sprintf("merge %s: %s", id, t.Title))
	if err != nil {
		return Result{}, err
	}
	commit = strings.TrimSpace(commit)
	if beforeUpdateRef != nil {
		beforeUpdateRef()
	}
	if err := gitx.UpdateRefCAS(repo, ref, commit, trunk); err != nil {
		if !errors.Is(err, gitx.ErrCASLost) {
			return Result{}, err
		}
		now, rerr := gitx.RevParse(repo, ref)
		if rerr != nil {
			return Result{}, rerr
		}
		return Result{Status: Conflict, TrunkSHA: strings.TrimSpace(now), Files: []string{}, Reason: "trunk moved"}, nil
	}
	return Result{Status: Merged, TrunkSHA: commit, Files: []string{}}, nil
}

// hasTaskLine scans NUL-separated messages line by line, the way
// gitx.TaskIDs does, for the line "Task: <id>".
func hasTaskLine(msgs, id string) bool {
	for _, msg := range strings.Split(msgs, "\x00") {
		for _, line := range strings.Split(msg, "\n") {
			if m := taskLine.FindStringSubmatch(strings.TrimSuffix(line, "\r")); m != nil && m[1] == id {
				return true
			}
		}
	}
	return false
}
