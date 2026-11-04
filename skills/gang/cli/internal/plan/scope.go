package plan

import (
	"path"
	"slices"
	"strings"
)

// TestOnlyRule says what a test-only file allows. The brief prints it under
// the test_only list, and reference/schema.md states it word for word.
const TestOnlyRule = "A test_only file takes test edits only: new tests, the call sites and fakes that the change needs, " +
	"and updates to the expectations that pin behavior that the task changes. No edit may remove an assertion. " +
	"A file that a scope.write glob also matches takes the scope.write rules."

// TestOnlyGlobs is the test-only scope of s: the globs in s.TestOnly, then
// each glob of TestGlobs(s.Write) that they lack. gang computes the second
// part each time it reads a plan and never saves it, so it follows
// scope.write when a task changes.
func (s Scope) TestOnlyGlobs() []string {
	out := append([]string{}, s.TestOnly...)
	for _, g := range TestGlobs(s.Write) {
		if !slices.Contains(out, g) {
			out = append(out, g)
		}
	}
	return out
}

// TestGlobs returns "<dir>/*_test.go" for each Go package that write names,
// in write order, with no duplicates. A write glob names a Go package when its
// last segment ends in ".go" and no segment of its directory is testdata,
// which Go ignores. A package adds no glob when a write glob already matches
// each of its tests: "<dir>/*.go", "<dir>/*", "<dir>/**", "<dir>/", or a glob
// with a "**" segment above the package.
func TestGlobs(write []string) []string {
	out := []string{}
	for _, g := range write {
		dir, base := splitGlob(cleanGlob(g))
		if !strings.HasSuffix(base, ".go") || slices.Contains(strings.Split(dir, "/"), "testdata") || coversTests(write, dir) {
			continue
		}
		if tg := path.Join(dir, "*_test.go"); !slices.Contains(out, tg) {
			out = append(out, tg)
		}
	}
	return out
}

// cleanGlob drops a leading "./", and turns a trailing "/" into "/**": the
// whole folder, as lint rule 5 reads it.
func cleanGlob(g string) string {
	g = strings.TrimPrefix(g, "./")
	if strings.HasSuffix(g, "/") {
		g += "**"
	}
	return g
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
		d, b := splitGlob(cleanGlob(w))
		if b == "**" {
			d, b = path.Join(d, "**"), "*"
		}
		if matchSegments(segments(d), segments(dir)) && allTests(b) {
			return true
		}
	}
	return false
}

// allTests reports whether the segment glob base matches every name that ends
// in "_test.go": it is one or more "*", then a tail of "_test.go", such as
// "*", "*.go", or "*_test.go".
func allTests(base string) bool {
	tail := strings.TrimLeft(base, "*")
	return tail != base && strings.HasSuffix("_test.go", tail)
}

func segments(p string) []string {
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// matchSegments matches a directory against a glob, one segment at a time.
// A "**" segment matches any number of segments, and any other segment
// matches as path.Match does.
func matchSegments(pattern, dir []string) bool {
	if len(pattern) == 0 {
		return len(dir) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(dir); i++ {
			if matchSegments(pattern[1:], dir[i:]) {
				return true
			}
		}
		return false
	}
	if len(dir) == 0 {
		return false
	}
	ok, _ := path.Match(pattern[0], dir[0])
	return ok && matchSegments(pattern[1:], dir[1:])
}
