// Package gh runs the gh CLI and reads wave PR state.
package gh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Run runs gh from PATH and returns its stdout. The error carries stderr.
func Run(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// Check is one CI check. StatusContext items map to Status COMPLETED (or
// PENDING) with their state as the Conclusion.
type Check struct{ Name, Status, Conclusion string }

// State is a PR's state, mergeability, head commit, and checks.
type State struct {
	State, Mergeable, HeadSHA string
	Checks                    []Check
}

// RepoArgs returns the --repo flag for remote, or nothing when remote is empty.
func RepoArgs(remote string) []string {
	if remote == "" {
		return nil
	}
	return []string{"--repo", remote}
}

// PRState reads PR n of remote ([HOST/]OWNER/REPO, as in plan repo.remote).
func PRState(remote string, n int) (State, error) {
	args := append([]string{"pr", "view", strconv.Itoa(n)}, RepoArgs(remote)...)
	out, err := Run(append(args, "--json", "state,mergeable,headRefOid,statusCheckRollup")...)
	if err != nil {
		return State{}, err
	}
	var raw struct {
		State      string `json:"state"`
		Mergeable  string `json:"mergeable"`
		HeadRefOid string `json:"headRefOid"`
		Rollup     []struct {
			Typename   string `json:"__typename"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			Context    string `json:"context"`
			State      string `json:"state"`
		} `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return State{}, fmt.Errorf("gh pr view %d: %w", n, err)
	}
	s := State{State: raw.State, Mergeable: raw.Mergeable, HeadSHA: raw.HeadRefOid, Checks: []Check{}}
	for _, it := range raw.Rollup {
		if it.Typename == "StatusContext" || (it.Typename == "" && it.Context != "") {
			status := "COMPLETED"
			if it.State == "PENDING" || it.State == "EXPECTED" {
				status = "PENDING"
			}
			s.Checks = append(s.Checks, Check{Name: it.Context, Status: status, Conclusion: it.State})
			continue
		}
		s.Checks = append(s.Checks, Check{Name: it.Name, Status: it.Status, Conclusion: it.Conclusion})
	}
	return s, nil
}

// PostStatus posts the commit status context on sha in remote ([HOST/]OWNER/REPO), with gh api.
func PostStatus(remote, sha, state, context, description string) error {
	host, repo := "github.com", remote
	if parts := strings.Split(remote, "/"); len(parts) == 3 {
		host, repo = parts[0], parts[1]+"/"+parts[2]
	}
	_, err := Run("api", "--hostname", host, "--method", "POST", "repos/"+repo+"/statuses/"+sha,
		"-f", "state="+state, "-f", "context="+context, "-f", "description="+description)
	return err
}

var (
	passing = []string{"SUCCESS", "NEUTRAL", "SKIPPED"}
	failing = []string{"FAILURE", "ERROR", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED", "STARTUP_FAILURE"}
)

// Classify applies Contract 7. Zero checks is neither green nor red. A pending
// check blocks green. Any failed check is red, with names sorted and unique.
func Classify(s State) (green, red bool, failed []string) {
	if len(s.Checks) == 0 {
		return false, false, nil
	}
	pending := false
	for _, c := range s.Checks {
		switch {
		case slices.Contains(failing, c.Conclusion):
			failed = append(failed, c.Name)
		case slices.Contains(passing, c.Conclusion) && (c.Status == "" || c.Status == "COMPLETED"):
		default:
			pending = true
		}
	}
	if len(failed) > 0 {
		sort.Strings(failed)
		return false, true, slices.Compact(failed)
	}
	return !pending && s.Mergeable == "MERGEABLE", false, nil
}
