// Package size counts the changed lines of a task diff, the measure that the
// size budget limits.
package size

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"gang/internal/gitx"
	"gang/internal/plan"
)

// DefaultExclude lists the lock files that a size count leaves out.
var DefaultExclude = []string{"go.sum", "package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "*.lock"}

// Excluded reports whether name matches one of globs. Each glob is matched
// with path.Match against the whole slash-separated name and against its base
// name, so "go.sum" matches "tools/go.sum". A malformed glob matches nothing.
func Excluded(name string, globs []string) bool {
	base := path.Base(name)
	for _, g := range globs {
		if ok, _ := path.Match(g, name); ok {
			return true
		}
		if ok, _ := path.Match(g, base); ok {
			return true
		}
	}
	return false
}

// Count returns the lines added plus the lines deleted between the tree-ish
// values from and to, as git diff --numstat counts them with rename detection.
// It skips binary files and the paths that Excluded matches. For a rename it
// matches the new path. It needs git 2.40, for --attr-source.
func Count(repo, from, to string, exclude []string) (int, error) {
	// --attr-source reads attributes from the from tree, so no .gitattributes in
	// the checkout or in the task can mark a file -diff and hide its lines. -l1000
	// pins git's default rename limit: diff.renameLimit cannot turn a rename into a
	// delete and an add, and the rename search stays bounded.
	// --diff-algorithm=myers keeps a diff.algorithm setting from changing a count.
	// --end-of-options and -- keep a revision from being read as an option or
	// as a path.
	out, err := gitx.Run(repo, "--attr-source="+from, "diff", "--numstat", "-z", "-M", "-l1000", "--diff-algorithm=myers", "--end-of-options", from, to, "--")
	if err != nil {
		return 0, err
	}
	return countNumstat(out, exclude)
}

// countNumstat sums the counts in git diff --numstat -z output. A record is
// "added TAB deleted TAB path NUL". A rename leaves the path empty and is
// followed by the old and the new path, each ending in NUL. A binary file has
// "-" for both counts. The final NUL may be missing.
func countNumstat(out string, exclude []string) (int, error) {
	out = strings.TrimSuffix(out, "\x00")
	if out == "" {
		return 0, nil
	}
	fields := strings.Split(out, "\x00")
	total := 0
	for i := 0; i < len(fields); i++ {
		rec := fields[i]
		parts := strings.SplitN(rec, "\t", 3)
		if len(parts) != 3 {
			return 0, fmt.Errorf("git diff --numstat: record %q has no counts", rec)
		}
		added, deleted, name := parts[0], parts[1], parts[2]
		if name == "" {
			if i+2 >= len(fields) {
				return 0, fmt.Errorf("git diff --numstat: rename record %q has no new path", rec)
			}
			name = fields[i+2]
			i += 2
		}
		if (added == "-" && deleted == "-") || Excluded(name, exclude) {
			continue
		}
		a, errA := strconv.Atoi(added)
		d, errD := strconv.Atoi(deleted)
		if errA != nil || errD != nil {
			return 0, fmt.Errorf("git diff --numstat: record %q has counts that are not numbers", rec)
		}
		total += a + d
	}
	return total, nil
}

// Check returns "" when a task of n changed lines fits the budget l, and
// otherwise the text that says what to do. A size above l.TaskLines needs a
// non-blank reason (the task's size.reason), and no size may go above
// l.SplitLines. Callers match the two texts by prefix.
func Check(l plan.Limits, reason string, n int) string {
	switch {
	case n <= l.TaskLines:
		return ""
	case n > l.SplitLines:
		return fmt.Sprintf("size %d is above limits.split_lines %d: split the task", n, l.SplitLines)
	case strings.TrimSpace(reason) == "":
		return fmt.Sprintf("size %d is above limits.task_lines %d: split the task, or give size.reason", n, l.TaskLines)
	}
	return ""
}

// Measure counts the changed lines from from to to, the way gang merge counts
// a task (Count with DefaultExclude), and returns Check's refusal text for it
// ("" within the budget).
func Measure(repo, from, to string, l plan.Limits, reason string) (n int, refusal string, err error) {
	if n, err = Count(repo, from, to, DefaultExclude); err != nil {
		return 0, "", err
	}
	return n, Check(l, reason, n), nil
}
