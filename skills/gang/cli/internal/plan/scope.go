package plan

import (
	"path"
	"slices"
	"strings"
)

// testProbes are two names of test files. A glob segment that matches both
// counts as a match for every _test.go file of its directory.
var testProbes = []string{"a_test.go", "z_test.go"}

// TestGlobs returns the scope.test_only globs that write implies: the glob
// "<dir>/*_test.go" for each Go package that write names, in write order,
// with no duplicates. A write glob names a Go package when its last path
// segment ends in ".go", such as "cli/internal/ledger/ledger.go". A package
// whose tests write already covers adds no glob: write holds a glob such as
// "<dir>/*.go", "<dir>/*", or "<dir>/**", or a "**" glob above the package.
func TestGlobs(write []string) []string {
	out := []string{}
	for _, g := range write {
		dir, base := splitGlob(g)
		if !strings.HasSuffix(base, ".go") || coversTests(write, dir) {
			continue
		}
		if tg := path.Join(dir, "*_test.go"); !slices.Contains(out, tg) {
			out = append(out, tg)
		}
	}
	return out
}

// splitGlob splits a glob at its last "/" into the directory and the last segment.
func splitGlob(g string) (dir, base string) {
	i := strings.LastIndex(g, "/")
	if i < 0 {
		return "", g
	}
	return g[:i], g[i+1:]
}

// coversTests reports whether a glob in write matches every _test.go file of dir.
func coversTests(write []string, dir string) bool {
	for _, w := range write {
		d, b := splitGlob(w)
		if b == "**" && (d == "" || d == dir || strings.HasPrefix(dir, d+"/")) {
			return true
		}
		if ok, _ := path.Match(d, dir); ok && matchesEvery(b, testProbes) {
			return true
		}
	}
	return false
}

func matchesEvery(pattern string, names []string) bool {
	for _, n := range names {
		if ok, _ := path.Match(pattern, n); !ok {
			return false
		}
	}
	return true
}
