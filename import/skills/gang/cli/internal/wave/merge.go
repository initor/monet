package wave

import (
	"fmt"
	"strconv"
	"strings"

	"gang/internal/exit"
	"gang/internal/gh"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/store"
)

// Merge merges open wave PR k with a merge commit and returns the merge SHA.
// It refuses (exit 2) unless every commit on the first-parent chain of
// origin/<base>..<head_sha> is a gang merge whose landed tip is ledger CLEAN
// for its required lenses, the PR head is still head_sha, and the PR is green.
func Merge(repo string, d store.Dir, p *plan.Plan, k int) (string, error) {
	rows, err := Read(d)
	if err != nil {
		return "", err
	}
	var row *Row
	for i := range rows {
		if rows[i].Wave == k && rows[i].State == StateOpen {
			row = &rows[i]
		}
	}
	if row == nil {
		return "", exit.New(exit.Refused, "wave %d has no open PR", k)
	}
	if _, err := gitx.Run(repo, "fetch", "--quiet", "origin"); err != nil {
		return "", fmt.Errorf("fetch origin: %w", err)
	}
	merges, stray, err := landings(repo, "refs/remotes/origin/"+p.Repo.Base+".."+row.HeadSHA)
	if err != nil {
		return "", err
	}
	var bad []string
	for _, s := range stray {
		bad = append(bad, "not landed by gang merge: "+s)
	}
	for _, m := range merges {
		clean, v, err := verdict(d, p, m.ID, m.Tip)
		if err != nil {
			return "", err
		}
		if !clean {
			bad = append(bad, fmt.Sprintf("%s %s %s", m.ID, short(m.Tip), v))
		}
	}
	if len(bad) > 0 {
		return "", exit.New(exit.Refused, "wave %d: %s", k, strings.Join(bad, "; "))
	}
	pr := strconv.Itoa(row.PR)
	st, err := gh.PRState(p.Repo.Remote, row.PR)
	if err != nil {
		return "", err
	}
	if st.State != "OPEN" {
		return "", exit.New(exit.Refused, "PR #%d is %s", row.PR, st.State)
	}
	if st.HeadSHA != row.HeadSHA {
		return "", exit.New(exit.Refused, "PR #%d head moved: %s, recorded %s", row.PR, short(st.HeadSHA), short(row.HeadSHA))
	}
	if green, _, _ := gh.Classify(st); !green {
		return "", exit.New(exit.Refused, "PR #%d is not green", row.PR)
	}
	repoArgs := gh.RepoArgs(p.Repo.Remote)
	if _, err := gh.Run(append(append([]string{"pr", "merge", pr}, repoArgs...), "--merge", "--match-head-commit", row.HeadSHA)...); err != nil {
		return "", err
	}
	out, err := gh.Run(append(append([]string{"pr", "view", pr}, repoArgs...), "--json", "mergeCommit", "--jq", ".mergeCommit.oid")...)
	if err != nil {
		return "", err
	}
	if err := SetState(d, k, StateMerged); err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
