// Package wave pushes the integration branch and opens and merges wave PRs.
package wave

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/secrets"
	"gang/internal/store"
)

// Row states in waves.tsv.
const (
	StateOpen   = "open"
	StateMerged = "merged"
	StateClosed = "closed"
)

var header = []string{"wave", "pr", "head_sha", "state", "opened_at"}

// Row is one waves.tsv line (Contract 4).
type Row struct {
	Wave, PR       int
	HeadSHA, State string
	OpenedAt       string
}

func tsvPath(d store.Dir) string { return d.Run("waves.tsv") }

// Read returns the waves.tsv rows, or none when the file does not exist.
func Read(d store.Dir) ([]Row, error) {
	recs, err := store.ReadTSV(tsvPath(d), header)
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(recs))
	for _, r := range recs {
		k, err1 := strconv.Atoi(r[0])
		pr, err2 := strconv.Atoi(r[1])
		if err := errors.Join(err1, err2); err != nil {
			return nil, fmt.Errorf("waves.tsv: %w", err)
		}
		rows = append(rows, Row{Wave: k, PR: pr, HeadSHA: r[2], State: r[3], OpenedAt: r[4]})
	}
	return rows, nil
}

// write rewrites waves.tsv atomically.
func write(d store.Dir, rows []Row) error {
	var b strings.Builder
	b.WriteString(strings.Join(header, "\t") + "\n")
	for _, r := range rows {
		cells := []string{strconv.Itoa(r.Wave), strconv.Itoa(r.PR), r.HeadSHA, r.State, r.OpenedAt}
		for i := range cells {
			cells[i] = store.SanitizeCell(cells[i])
		}
		b.WriteString(strings.Join(cells, "\t") + "\n")
	}
	return store.WriteFileAtomic(tsvPath(d), []byte(b.String()), 0o644)
}

// SetState sets the state of wave k's row.
func SetState(d store.Dir, k int, state string) error {
	rows, err := Read(d)
	if err != nil {
		return err
	}
	for i := range rows {
		if rows[i].Wave == k {
			rows[i].State = state
			return write(d, rows)
		}
	}
	return exit.New(exit.Refused, "wave %d has no row in waves.tsv", k)
}

// Done returns the plan task ids with a "Task: <id>" line reachable from the
// integration branch (Contract 0.1). A missing branch means nothing is done.
func Done(repo string, p *plan.Plan) (map[string]bool, error) {
	ids, err := gitx.TaskIDs(repo, "refs/heads/"+p.Repo.Integration)
	if err != nil {
		return nil, err
	}
	done := map[string]bool{}
	for _, id := range ids {
		if _, ok := p.Task(id); ok {
			done[id] = true
		}
	}
	return done, nil
}

// Push fast-forwards origin/<integration> to the local tip after a secret scan
// of every commit that would reach the remote. It never forces. A push moves
// the open wave PR's head, so it records the pushed tip as that row's head_sha.
func Push(repo string, d store.Dir, p *plan.Plan, dryRun bool) (string, error) {
	integ := p.Repo.Integration
	tip, err := gitx.RevParse(repo, "refs/heads/"+integ)
	if errors.Is(err, gitx.ErrNotFound) {
		return "", exit.New(exit.Refused, "integration branch %s not found", integ)
	}
	if err != nil {
		return "", err
	}
	if _, err := gitx.Run(repo, "fetch", "--quiet", "origin"); err != nil {
		return "", fmt.Errorf("fetch origin: %w", err)
	}
	from := "refs/remotes/origin/" + integ
	remote, err := gitx.RevParse(repo, from)
	switch {
	case err == nil:
		ok, err := gitx.IsAncestor(repo, remote, tip)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", exit.New(exit.Refused, "origin/%s is not an ancestor of %s: not a fast-forward", integ, short(tip))
		}
	case errors.Is(err, gitx.ErrNotFound):
		from = "refs/remotes/origin/" + p.Repo.Base
		if _, err := gitx.RevParse(repo, from); err != nil {
			return "", exit.New(exit.Refused, "origin/%s not found", p.Repo.Base)
		}
	default:
		return "", err
	}
	if err := scan(repo, from, tip); err != nil {
		return "", err
	}
	if dryRun {
		return fmt.Sprintf("git -C %s push origin %s:refs/heads/%s", repo, tip, integ), nil
	}
	if _, err := gitx.Run(repo, "push", "--quiet", "origin", tip+":refs/heads/"+integ); err != nil {
		return "", exit.New(exit.Refused, "push rejected: %v", err)
	}
	if err := setPRHeads(d, tip); err != nil {
		return "", err
	}
	return tip, nil
}

// setPRHeads sets head_sha to tip on every row that is not merged. Each
// wave PR's head is the integration branch, so a closed PR that is reopened
// shows the pushed tip too. With no such row it changes nothing.
func setPRHeads(d store.Dir, tip string) error {
	rows, err := Read(d)
	if err != nil {
		return err
	}
	changed := false
	for i := range rows {
		if rows[i].State != StateMerged && rows[i].HeadSHA != tip {
			rows[i].HeadSHA, changed = tip, true
		}
	}
	if !changed {
		return nil
	}
	return write(d, rows)
}

// scan checks each commit's patch and message in from..tip, so a secret added
// and then deleted inside the range is still caught. A merge shows its diff
// against its first parent, which covers lines added while resolving it.
// Binary and -diff files are diffed as text, with no textconv filter.
func scan(repo, from, tip string) error {
	patch, err := gitx.Run(repo, "log", "-p", "--text", "--no-textconv", "--unified=0", "--no-color", "--no-ext-diff",
		"--no-renames", "--diff-merges=first-parent", "--format=", from+".."+tip)
	if err != nil {
		return err
	}
	msgs, err := gitx.Run(repo, "log", "--format=%x00%H%n%B", from+".."+tip)
	if err != nil {
		return err
	}
	found := append(secrets.ScanDiff(patch), secrets.ScanDiff(messageDiff(msgs))...)
	if len(found) == 0 {
		return nil
	}
	lines := make([]string, len(found))
	for i, f := range found {
		lines[i] = fmt.Sprintf("%s:%d:%s", f.File, f.Line, f.Rule)
	}
	return exit.New(exit.Refused, "secret scan of %s..%s found %d:\n%s", strings.TrimPrefix(from, "refs/remotes/"), short(tip), len(found), strings.Join(lines, "\n"))
}

// messageDiff turns git log --format=%x00%H%n%B output into a diff that adds
// each message as the lines of a file named "commit <sha7> message".
func messageDiff(log string) string {
	var b strings.Builder
	for _, rec := range strings.Split(log, "\x00") {
		sha, msg, ok := strings.Cut(rec, "\n")
		if !ok {
			continue
		}
		lines := strings.Split(strings.TrimRight(msg, "\n"), "\n")
		fmt.Fprintf(&b, "+++ commit %s message\n@@ -0,0 +1,%d @@\n", short(sha), len(lines))
		for _, l := range lines {
			b.WriteString("+" + l + "\n")
		}
	}
	return b.String()
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
