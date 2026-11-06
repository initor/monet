// Package preflight holds the local checks of monet-source preflight. It runs programs only
// through an injected Runner, so a test never starts one.
package preflight

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Check is one result: its name, whether it passed, and the reason when it did not.
type Check struct {
	Name   string
	OK     bool
	Detail string
}

// Runner runs a program and returns its combined output, with an error when the program does
// not run or exits non-zero.
type Runner func(name string, args ...string) (string, error)

var (
	goVersion   = regexp.MustCompile(`^go version go1\.(\d+)`)
	nodeVersion = regexp.MustCompile(`^v(\d+)\.`)
)

// goMinor reads the minor of the "go version go1.<minor>" that go version prints.
func goMinor(out string) (int, bool) { return number(goVersion, out) }

// nodeMajor reads the major of the "v<major>." that node --version prints.
func nodeMajor(out string) (int, bool) { return number(nodeVersion, out) }

func number(re *regexp.Regexp, out string) (int, bool) {
	m := re.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// Tools checks go (go1.24 or newer), node (v22 or newer), git, claude, and gh, in that order.
// git, claude, and gh pass when they run with --version and no error.
func Tools(run Runner) []Check {
	return []Check{
		version(run, "go", "version", goMinor, 24, "go1.%d is older than go1.24"),
		version(run, "node", "--version", nodeMajor, 22, "v%d is older than v22"),
		version(run, "git", "--version", nil, 0, ""),
		version(run, "claude", "--version", nil, 0, ""),
		version(run, "gh", "--version", nil, 0, ""),
	}
}

// version runs name with arg and, when read is set, needs the number it reads to be least or more.
func version(run Runner, name, arg string, read func(string) (int, bool), least int, older string) Check {
	out, err := run(name, arg)
	if err != nil {
		return Check{Name: name, Detail: err.Error()}
	}
	if read == nil {
		return Check{Name: name, OK: true}
	}
	n, ok := read(out)
	if !ok {
		line, _, _ := strings.Cut(out, "\n")
		return Check{Name: name, Detail: fmt.Sprintf("cannot read the version in %q", line)}
	}
	if n < least {
		return Check{Name: name, Detail: fmt.Sprintf(older, n)}
	}
	return Check{Name: name, OK: true}
}
