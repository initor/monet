package preflight

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// fakeRun gives outs[name] for a program, or an error for a name it lacks.
func fakeRun(outs map[string]string) Runner {
	return func(name string, args ...string) (string, error) {
		out, ok := outs[name]
		if !ok {
			return "", errors.New(name + ": executable file not found in $PATH")
		}
		return out, nil
	}
}

// wantDetail is the detail of a version check whose output reads n, ok against least.
func wantDetail(out string, n int, ok bool, least int, older string) string {
	line, _, _ := strings.Cut(out, "\n")
	switch {
	case !ok:
		return fmt.Sprintf("cannot read the version in %q", line)
	case n < least:
		return fmt.Sprintf(older, n)
	}
	return ""
}

func TestGoVersionRule(t *testing.T) {
	for minor := 0; minor <= 40; minor++ {
		for _, f := range []string{"go version go1.%d darwin/arm64\n", "go version go1.%d.3 linux/amd64\n",
			"go version go1.%drc1 darwin/arm64\n", "go version go2.%d darwin/arm64\n", "go version devel go1.%d-x\n",
			"x go version go1.%d darwin/arm64\n"} {
			out := fmt.Sprintf(f, minor)
			want, wantOK := 0, false
			if fs := strings.Fields(out); len(fs) >= 3 && strings.HasPrefix(fs[2], "go1.") {
				rest := strings.TrimPrefix(fs[2], "go1.")
				i := strings.IndexFunc(rest, func(r rune) bool { return r < '0' || r > '9' })
				if i < 0 {
					i = len(rest)
				}
				n, err := strconv.Atoi(rest[:i])
				want, wantOK = n, err == nil
			}
			if got, ok := goMinor(out); ok != wantOK || got != want {
				t.Fatalf("%q: got %d %v, want %d %v", out, got, ok, want, wantOK)
			}
			c := Tools(fakeRun(map[string]string{"go": out}))[0]
			if c.Name != "go" || c.OK != (wantOK && want >= 24) || c.Detail != wantDetail(out, want, wantOK, 24, "go1.%d is older than go1.24") {
				t.Fatalf("%q: check %+v", out, c)
			}
		}
	}
}

func TestNodeVersionRule(t *testing.T) {
	for major := 0; major <= 40; major++ {
		for _, f := range []string{"v%d.1.0\n", "v%d\n", "%d.1.0\n", "vv%d.1\n"} {
			out := fmt.Sprintf(f, major)
			want, wantOK := 0, false
			if rest, ok := strings.CutPrefix(out, "v"); ok {
				if i := strings.IndexByte(rest, '.'); i > 0 {
					n, err := strconv.Atoi(rest[:i])
					want, wantOK = n, err == nil
				}
			}
			if got, ok := nodeMajor(out); ok != wantOK || got != want {
				t.Fatalf("%q: got %d %v, want %d %v", out, got, ok, want, wantOK)
			}
			c := Tools(fakeRun(map[string]string{"node": out}))[1]
			if c.Name != "node" || c.OK != (wantOK && want >= 22) || c.Detail != wantDetail(out, want, wantOK, 22, "v%d is older than v22") {
				t.Fatalf("%q: check %+v", out, c)
			}
		}
	}
}

func TestToolChecks(t *testing.T) {
	outs := map[string]string{"go": "go version go1.24.2 darwin/arm64\n", "node": "v22.11.0\n",
		"git": "git version 2.47.1\n", "claude": "2.1.300 (Claude Code)\n", "gh": "gh version 2.63.0\n"}
	var calls []string
	run := func(name string, args ...string) (string, error) {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return fakeRun(outs)(name, args...)
	}
	got := Tools(run)
	want := []Check{{Name: "go", OK: true}, {Name: "node", OK: true}, {Name: "git", OK: true},
		{Name: "claude", OK: true}, {Name: "gh", OK: true}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("all present: got %+v, want %+v", got, want)
	}
	wantCalls := "go version|node --version|git --version|claude --version|gh --version"
	if strings.Join(calls, "|") != wantCalls {
		t.Fatalf("calls: got %q, want %q", calls, wantCalls)
	}

	delete(outs, "gh")
	if c := Tools(fakeRun(outs))[4]; c != (Check{"gh", false, "gh: executable file not found in $PATH"}) {
		t.Fatalf("gh missing: got %+v", c)
	}
	for i, c := range Tools(fakeRun(nil)) {
		name := want[i].Name
		if c != (Check{name, false, name + ": executable file not found in $PATH"}) {
			t.Fatalf("none present: check %d is %+v", i, c)
		}
	}
}
