// Package size counts the changed lines of a task diff, the measure that the
// size budget limits.
package size

import (
	"fmt"
	pathpkg "path" // Excluded names its parameter path
	"strconv"
	"strings"

	"gang/internal/gitx"
)

// DefaultExclude lists the lock files that a size count leaves out.
var DefaultExclude = []string{"go.sum", "package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "*.lock"}

// Excluded reports whether path matches one of globs. Each glob is matched
// with path.Match against the whole slash-separated path and against its base
// name, so "go.sum" matches "tools/go.sum". A malformed glob matches nothing.
func Excluded(path string, globs []string) bool {
	base := pathpkg.Base(path)
	for _, g := range globs {
		if ok, _ := pathpkg.Match(g, path); ok {
			return true
		}
		if ok, _ := pathpkg.Match(g, base); ok {
			return true
		}
	}
	return false
}

// Count returns the lines added plus the lines deleted between the tree-ish
// values from and to, as git diff --numstat counts them with rename detection.
// It skips binary files and the paths that Excluded matches. For a rename it
// matches the new path.
func Count(repo, from, to string, exclude []string) (int, error) {
	// --end-of-options and -- keep a revision from being read as an option or
	// as a path.
	out, err := gitx.Run(repo, "diff", "--numstat", "-z", "-M", "--end-of-options", from, to, "--")
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
