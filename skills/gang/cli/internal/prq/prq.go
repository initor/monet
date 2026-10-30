// Package prq keeps the PR queue state in run/prs.tsv.
package prq

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

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
