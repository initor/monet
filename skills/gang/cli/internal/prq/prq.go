// Package prq keeps the PR queue state in run/prs.tsv.
package prq

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gang/internal/exit"
	"gang/internal/gh"
	"gang/internal/plan"
	"gang/internal/store"
)

// Row states in prs.tsv.
const StateOpen, StateMerged, StateClosed = "open", "merged", "closed"

var header = []string{"task", "pr", "branch", "head_sha", "state", "opened_at", "fixes"}

// Row is one prs.tsv line, the PR that publishes one task. Fixes counts the
// fix commits that gang pr fix has added to it, at most 2.
type Row struct {
	Task                             string
	PR                               int
	Branch, HeadSHA, State, OpenedAt string
	Fixes                            int
}

// Read returns the prs.tsv rows, or none when the file does not exist.
func Read(d store.Dir) ([]Row, error) {
	recs, err := store.ReadTSV(d.Run("prs.tsv"), header)
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(recs))
	for _, r := range recs {
		pr, err1 := strconv.Atoi(r[1])
		fixes, err2 := strconv.Atoi(r[6])
		if err := errors.Join(err1, err2); err != nil {
			return nil, fmt.Errorf("prs.tsv: %w", err)
		}
		rows = append(rows, Row{Task: r[0], PR: pr, Branch: r[2], HeadSHA: r[3], State: r[4], OpenedAt: r[5], Fixes: fixes})
	}
	return rows, nil
}

// write rewrites prs.tsv atomically.
func write(d store.Dir, rows []Row) error {
	var b strings.Builder
	b.WriteString(strings.Join(header, "\t") + "\n")
	for _, r := range rows {
		cells := []string{r.Task, strconv.Itoa(r.PR), r.Branch, r.HeadSHA, r.State, r.OpenedAt, strconv.Itoa(r.Fixes)}
		for i := range cells {
			cells[i] = store.SanitizeCell(cells[i])
		}
		b.WriteString(strings.Join(cells, "\t") + "\n")
	}
	return store.WriteFileAtomic(d.Run("prs.tsv"), []byte(b.String()), 0o644)
}

// Put replaces the row of r.Task, or appends r when the task has none.
func Put(d store.Dir, r Row) error {
	rows, err := Read(d)
	if err != nil {
		return err
	}
	for i := range rows {
		if rows[i].Task == r.Task {
			rows[i] = r
			return write(d, rows)
		}
	}
	return write(d, append(rows, r))
}

// Current returns the first open row, if there is one.
func Current(rows []Row) (Row, bool) {
	for _, r := range rows {
		if r.State == StateOpen {
			return r, true
		}
	}
	return Row{}, false
}

// SetState sets the state of task's row. It reads prs.tsv right before it
// writes, so every other field stays as the file holds it now, never as a
// caller read it before a slow step such as a gh call.
func SetState(d store.Dir, task, state string) error {
	rows, err := Read(d)
	if err != nil {
		return err
	}
	for i := range rows {
		if rows[i].Task == task {
			rows[i].State = state
			return write(d, rows)
		}
	}
	return fmt.Errorf("prs.tsv has no row for %s", task)
}

// Live returns the open row of task id and the state of its PR on GitHub.
// Every outward step on an open PR starts here.
func Live(d store.Dir, p *plan.Plan, id string) (Row, gh.State, error) {
	if p.Repo.GHAccount == "" {
		return Row{}, gh.State{}, exit.New(exit.Refused, "repo.gh_account is empty: gang makes no GitHub call for %s", id)
	}
	rows, err := Read(d)
	if err != nil {
		return Row{}, gh.State{}, err
	}
	r, ok := Current(rows)
	if !ok || r.Task != id {
		return Row{}, gh.State{}, exit.New(exit.Refused, "%s has no open PR", id)
	}
	st, err := gh.PRState(p.Repo.Remote, r.PR)
	switch {
	case err != nil:
		return Row{}, gh.State{}, err
	case st.State != "OPEN":
		return Row{}, gh.State{}, exit.New(exit.Refused, "PR #%d of %s is %s on GitHub", r.PR, id, st.State)
	case st.HeadSHA != r.HeadSHA:
		return Row{}, gh.State{}, exit.New(exit.Refused, "PR #%d head moved: %.7s, recorded %.7s", r.PR, st.HeadSHA, r.HeadSHA)
	}
	return r, st, nil
}

// Merge squash-merges the open PR of task id with --match-head-commit, and
// marks its row merged. It returns the squash subject.
func Merge(repo string, d store.Dir, p *plan.Plan, id string) (string, error) {
	r, st, err := Live(d, p, id)
	if err != nil {
		return "", err
	}
	if green, _, failed := gh.Classify(st); !green {
		if len(failed) > 0 {
			return "", exit.New(exit.Refused, "PR #%d is not green, failed: %s", r.PR, strings.Join(failed, ","))
		}
		return "", exit.New(exit.Refused, "PR #%d is not green", r.PR)
	}
	cs, err := commits(repo, "refs/remotes/origin/"+p.Repo.Base+".."+r.HeadSHA)
	if err != nil {
		return "", err
	}
	if len(cs) == 0 {
		return "", fmt.Errorf("PR #%d has no commit above origin/%s", r.PR, p.Repo.Base)
	}
	// Landings reads the first Task line of the squash message, which is the
	// first one of the oldest commit, so that line must name the task.
	if m := taskLine.FindStringSubmatch(cs[0].msg); m == nil || m[1] != id {
		return "", exit.New(exit.Refused, "PR #%d: the oldest commit above origin/%s is not the %s task commit", r.PR, p.Repo.Base, id)
	}
	subject, body := fmt.Sprintf("%s (#%d)", cs[0].subject, r.PR), ""
	for _, c := range cs {
		if m := taskLine.FindStringSubmatch(c.msg); m != nil {
			body += "Task: " + m[1] + "\n"
		}
	}
	file := d.Run("merge-body-" + id + ".md")
	if err := store.WriteFileAtomic(file, []byte(body), 0o644); err != nil {
		return "", err
	}
	defer os.Remove(file)
	args := append([]string{"pr", "merge", strconv.Itoa(r.PR)}, gh.RepoArgs(p.Repo.Remote)...)
	if _, err := gh.Run(append(args, "--squash", "--match-head-commit", r.HeadSHA, "--subject", subject, "--body-file", file)...); err != nil {
		return "", err
	}
	if err := SetState(d, id, StateMerged); err != nil {
		return "", err
	}
	return subject, nil
}
