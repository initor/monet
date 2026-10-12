package wave

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gang/internal/exit"
	"gang/internal/gh"
	"gang/internal/gitx"
	"gang/internal/ledger"
	"gang/internal/plan"
	"gang/internal/repair"
	"gang/internal/store"
)

var (
	prURLRe     = regexp.MustCompile(`https?://\S+/pull/(\d+)`)
	gangMergeRe = regexp.MustCompile(`^merge ([A-Z]+[0-9]+): `)
)

// ghStates maps a GitHub PR state to a waves.tsv row state.
var ghStates = map[string]string{"OPEN": StateOpen, "MERGED": StateMerged, "CLOSED": StateClosed}

// Settle sets a row that is not merged to the state GitHub reports, and
// reports whether the state changed. A PR merged by hand frees the next wave,
// and a closed PR that is reopened (to retrigger CI) is open again.
func Settle(d store.Dir, r Row, st gh.State) (bool, error) {
	next, ok := ghStates[st.State]
	if r.State == StateMerged || !ok || next == r.State {
		return false, nil
	}
	return true, SetState(d, r.Wave, next)
}

// Refresh settles every open or closed row against GitHub, then returns the
// rows. It never polls a merged row.
func Refresh(d store.Dir, p *plan.Plan) ([]Row, error) {
	rows, err := Read(d)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.State == StateMerged {
			continue
		}
		st, err := gh.PRState(p.Repo.Remote, r.PR)
		if err != nil {
			return nil, err
		}
		if _, err := Settle(d, r, st); err != nil {
			return nil, err
		}
	}
	return Read(d)
}

// Open pushes the integration branch and opens wave PR k into base. It refuses
// (exit 2) unless every task with wave <= k is merged, no wave PR is open, no
// row exists for a later wave, and wave k has no merged row. The new PR
// replaces a closed row for wave k. A dry run prints the PR and records
// nothing. Without repo.gh_account only a dry run is allowed (Contract 0.9).
func Open(repo string, d store.Dir, p *plan.Plan, k int, dryRun bool) (string, error) {
	if p.Repo.GHAccount == "" && !dryRun {
		return "", exit.New(exit.Refused, "repo.gh_account is empty: wave open works only with --dry-run")
	}
	rows, err := Refresh(d, p)
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if r.State == StateOpen {
			return "", exit.New(exit.Refused, "wave %d PR #%d is still open", r.Wave, r.PR)
		}
		if r.Wave > k || r.Wave == k && r.State != StateClosed {
			return "", exit.New(exit.Refused, "wave %d already has PR #%d (%s): open a later wave", r.Wave, r.PR, r.State)
		}
	}
	done, err := Done(repo, p)
	if err != nil {
		return "", err
	}
	inWave, missing := false, []string{}
	for _, t := range p.Tasks {
		inWave = inWave || t.Wave == k
		if t.Wave <= k && !done[t.ID] {
			missing = append(missing, t.ID)
		}
	}
	if !inWave {
		return "", exit.New(exit.Refused, "wave %d has no tasks", k)
	}
	if len(missing) > 0 {
		return "", exit.New(exit.Refused, "wave %d is not ready, not merged: %s", k, strings.Join(missing, ", "))
	}
	if _, err := Push(repo, d, p, dryRun); err != nil {
		return "", err
	}
	tip, err := gitx.RevParse(repo, "refs/heads/"+p.Repo.Integration)
	if err != nil {
		return "", err
	}
	base := "refs/remotes/origin/" + p.Repo.Base
	n, err := gitx.Run(repo, "rev-list", "--count", base+".."+tip)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(n) == "0" {
		return "", exit.New(exit.Refused, "origin/%s already contains %s: nothing to open", p.Repo.Base, short(tip))
	}
	title := plan.WaveTitle(p, k)
	body, err := prBody(repo, d, p, k, base, tip)
	if err != nil {
		return "", err
	}
	if dryRun {
		return title + "\n\n" + body, nil
	}
	f, err := os.CreateTemp(d.Run(), "wave-body-*.md")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	args := append([]string{"pr", "create"}, gh.RepoArgs(p.Repo.Remote)...)
	out, err := gh.Run(append(args, "--base", p.Repo.Base, "--head", p.Repo.Integration, "--title", title, "--body-file", f.Name())...)
	if err != nil {
		return "", err
	}
	m := prURLRe.FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("gh pr create printed no PR URL: %q", strings.TrimSpace(string(out)))
	}
	pr, _ := strconv.Atoi(m[1])
	kept := rows[:0]
	for _, r := range rows {
		if r.Wave != k {
			kept = append(kept, r)
		}
	}
	rows = append(kept, Row{Wave: k, PR: pr, HeadSHA: tip, State: StateOpen, OpenedAt: time.Now().UTC().Format(time.RFC3339)})
	if err := write(d, rows); err != nil {
		return "", err
	}
	return m[0], nil
}

// landing is one gang merge on the integration branch: the commit
// "merge <id>: <title>" whose second parent is the tip that was reviewed.
type landing struct{ ID, Tip, Merge string }

// landings walks the first-parent chain of rng, newest first. A commit there
// that is not a gang merge is returned in stray, as "<sha7> <subject>". A gang
// merge has the gang title, two parents, and the clean merge-tree of them.
func landings(repo, rng string) (merges []landing, stray []string, err error) {
	out, err := gitx.Run(repo, "log", "--first-parent", "--format=%H%x00%P%x00%T%x00%s", rng)
	if err != nil {
		return nil, nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		f := strings.SplitN(line, "\x00", 4)
		if len(f) != 4 {
			continue
		}
		parents := strings.Fields(f[1])
		if m := gangMergeRe.FindStringSubmatch(f[3]); m != nil && len(parents) == 2 {
			tree, conflicts, err := gitx.MergeTree(repo, parents[0], parents[1])
			if err != nil {
				return nil, nil, err
			}
			if len(conflicts) == 0 && tree == f[2] {
				merges = append(merges, landing{ID: m[1], Tip: parents[1], Merge: f[0]})
				continue
			}
		}
		stray = append(stray, short(f[0])+" "+f[3])
	}
	return merges, stray, nil
}

// verdict is CLEAN or NOT-VERIFIED with the missing lenses, as gang ledger
// check prints. An F<nn> id outside the plan resolves through
// repair.TaskFor (Contract 0.16).
func verdict(d store.Dir, p *plan.Plan, id, sha string) (bool, string, error) {
	t, ok := repair.TaskFor(p, id)
	if !ok {
		return false, "NOT-IN-PLAN", nil
	}
	clean, missing, err := ledger.Check(d, id, sha, ledger.RequiredLenses(t))
	if err != nil {
		return false, "", err
	}
	if clean {
		return true, "CLEAN", nil
	}
	names := make([]string, len(missing))
	for i, l := range missing {
		names[i] = fmt.Sprint(l)
	}
	return false, "NOT-VERIFIED " + strings.Join(names, ","), nil
}

func prBody(repo string, d store.Dir, p *plan.Plan, k int, base, tip string) (string, error) {
	merges, stray, err := landings(repo, base+".."+tip)
	if err != nil {
		return "", err
	}
	type line struct {
		wave int
		id   string
		text string
	}
	lines := make([]line, 0, len(merges))
	for _, m := range merges {
		_, v, err := verdict(d, p, m.ID, m.Tip)
		if err != nil {
			return "", err
		}
		t, ok := repair.TaskFor(p, m.ID)
		if !ok {
			lines = append(lines, line{1 << 30, m.ID, fmt.Sprintf("| %s | (not in plan) | - | - | %s |", cell(m.ID), v)})
			continue
		}
		reqs := strings.Join(t.Requirements, ", ")
		if reqs == "" {
			reqs = "-"
		}
		lines = append(lines, line{t.Wave, t.ID, fmt.Sprintf("| %s | %s | %d | %s | %s |",
			cell(t.ID), cell(t.Title), t.Wave, cell(reqs), v)})
	}
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].wave != lines[j].wave {
			return lines[i].wave < lines[j].wave
		}
		return lines[i].id < lines[j].id
	})
	var b strings.Builder
	fmt.Fprintf(&b, "Wave %d of gang run `%s`: %d tasks in origin/%s..%s at %s.\n\n", k, p.Slug, len(merges), p.Repo.Base, p.Repo.Integration, short(tip))
	b.WriteString("| Task | Title | Wave | Requirements | Ledger |\n|---|---|---|---|---|\n")
	for _, l := range lines {
		b.WriteString(l.text + "\n")
	}
	if len(stray) > 0 {
		b.WriteString("\nCommits that gang merge did not land (gang wave merge refuses these):\n\n")
		for _, s := range stray {
			b.WriteString("- " + cell(s) + "\n")
		}
	}
	b.WriteString("\nOpened by `gang wave open`. Ledger is the verdict for the tip each gang merge landed.\n")
	return b.String(), nil
}

func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "\n", " ", "\r", " ").Replace(s)
}
