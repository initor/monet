// Package gitx is a thin wrapper over the git CLI. Every call runs
// `git -C <dir> ...` with the inherited variables that would point git at
// another repository or index removed.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// ErrCASLost means UpdateRefCAS found the ref no longer at the expected value.
var ErrCASLost = errors.New("compare-and-swap lost: the ref moved")

// ErrNotFound means a revision does not resolve to a commit.
var ErrNotFound = errors.New("revision not found")

// redirects are variables a git hook or a parent git process can leave set.
// Each one would override -C and aim the command at a different repo or index.
var redirects = []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR",
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_NAMESPACE"}

func environ(extra []string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !slices.Contains(redirects, name) {
			env = append(env, kv)
		}
	}
	return append(env, extra...)
}

// git runs git -C dir args and returns raw stdout and the exit code. A
// non-zero code listed in ok is not an error.
func git(dir string, env []string, ok []int, args ...string) ([]byte, int, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = environ(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return nil, -1, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		code = ee.ExitCode()
	}
	if code != 0 && !slices.Contains(ok, code) {
		return stdout.Bytes(), code, fmt.Errorf("git %s: exit %d: %s",
			strings.Join(args, " "), code, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), code, nil
}

// Run runs git -C dir args and returns stdout without trailing newlines.
// The error carries git's stderr.
func Run(dir string, args ...string) (string, error) { return RunEnv(dir, nil, args...) }

// RunEnv is Run with extra environment entries, such as GIT_INDEX_FILE.
func RunEnv(dir string, env []string, args ...string) (string, error) {
	out, _, err := git(dir, env, nil, args...)
	return strings.TrimRight(string(out), "\n"), err
}

// RevParse resolves rev to a commit SHA. A rev that names no commit
// returns an error wrapping ErrNotFound.
func RevParse(dir, rev string) (string, error) {
	out, code, err := git(dir, nil, []int{1}, "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	if code == 1 {
		return "", fmt.Errorf("%s: %w", rev, ErrNotFound)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsAncestor reports whether commit a is an ancestor of (or equal to) b.
func IsAncestor(dir, a, b string) (bool, error) {
	_, code, err := git(dir, nil, []int{1}, "merge-base", "--is-ancestor", a, b)
	if err != nil {
		return false, err
	}
	return code == 0, nil
}

// MergeTree merges theirs into ours without touching any index or work tree.
// It returns the result tree. Exit 1 is a conflict even when git names no
// path, so conflicts is then never empty: the conflicted paths, else git's
// "CONFLICT (" messages, else one placeholder entry.
func MergeTree(dir, ours, theirs string) (tree string, conflicts []string, err error) {
	out, code, err := git(dir, nil, []int{1}, "merge-tree", "--write-tree", "--name-only", ours, theirs)
	if err != nil {
		return "", nil, err
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if code == 0 {
		return lines[0], nil, nil
	}
	// After the tree come the conflicted paths, an empty line, then messages.
	rest := lines[1:]
	for len(rest) > 0 && rest[0] != "" {
		conflicts, rest = append(conflicts, rest[0]), rest[1:]
	}
	if len(conflicts) == 0 {
		for _, l := range rest {
			if strings.HasPrefix(l, "CONFLICT (") {
				conflicts = append(conflicts, l)
			}
		}
	}
	if len(conflicts) == 0 {
		conflicts = []string{"merge-tree exited 1 without naming a path"}
	}
	return lines[0], conflicts, nil
}

// CommitTree creates a commit of tree with the given parents and message.
func CommitTree(dir, tree string, parents []string, msg string) (string, error) {
	args := []string{"commit-tree", tree}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	return Run(dir, append(args, "-m", msg)...)
}

// UpdateRefCAS points ref at newSHA only if ref is still at oldSHA. An empty
// oldSHA means ref must not exist yet. A moved ref returns ErrCASLost.
func UpdateRefCAS(dir, ref, newSHA, oldSHA string) error {
	if !strings.HasPrefix(ref, "refs/") {
		return fmt.Errorf("update-ref %s: want a full ref name under refs/", ref)
	}
	old := oldSHA
	if old == "" {
		old = strings.Repeat("0", len(newSHA))
	}
	_, err := Run(dir, "update-ref", ref, newSHA, old)
	if err == nil {
		return nil
	}
	cur, rerr := RevParse(dir, ref)
	switch {
	case errors.Is(rerr, ErrNotFound):
		if oldSHA != "" {
			return fmt.Errorf("%s: %w", ref, ErrCASLost)
		}
	case rerr != nil:
		return err
	case cur != oldSHA:
		return fmt.Errorf("%s: %w", ref, ErrCASLost)
	}
	return err
}

// taskLine is a whole message line that names a task. It need not be a git
// trailer: demo puts it above a "Testing Done" paragraph.
var taskLine = regexp.MustCompile(`^Task: ([A-Z]+[0-9]+)$`)

// TaskIDs returns the sorted, unique ids from lines that are exactly
// "Task: <id>" in the messages of commits reachable from rev. A rev that
// does not exist yields an empty list.
func TaskIDs(dir, rev string) ([]string, error) {
	if _, err := RevParse(dir, rev); errors.Is(err, ErrNotFound) {
		return []string{}, nil
	} else if err != nil {
		return nil, err
	}
	out, err := Run(dir, "log", "--format=%B%x00", rev, "--")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, msg := range strings.Split(out, "\x00") {
		for _, line := range strings.Split(msg, "\n") {
			m := taskLine.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
			if m != nil && !seen[m[1]] {
				seen[m[1]] = true
				ids = append(ids, m[1])
			}
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// MainRepo returns the main checkout that owns dir, even when dir is inside
// a linked worktree: the parent of the absolute git common dir.
func MainRepo(dir string) (string, error) {
	common, err := Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.Dir(common), nil
}

// Show returns the exact bytes of path at rev.
func Show(dir, rev, path string) ([]byte, error) {
	out, _, err := git(dir, nil, nil, "cat-file", "blob", rev+":"+path)
	if err != nil {
		return nil, err
	}
	return out, nil
}
