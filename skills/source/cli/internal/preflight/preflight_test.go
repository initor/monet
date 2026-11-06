package preflight

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"source/internal/relay"
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

// oneByteEdits returns s, each deletion of one byte, and each substitution or
// insertion of one byte from 0x20 to 0x7e (a copy of the helper of package round).
func oneByteEdits(s string) []string {
	out := []string{s}
	for i := range len(s) + 1 {
		out = append(out, s[:i]+s[min(i+1, len(s)):])
		for b := byte(0x20); b < 0x7f; b++ {
			out = append(out, s[:i]+string(b)+s[i:], s[:i]+string(b)+s[min(i+1, len(s)):])
		}
	}
	return out
}

// outputs gives line alone, above another line, below it, and twice.
func outputs(line, other string) []string {
	return []string{line, line + "\n" + other + "\n", other + "\n" + line + "\n", line + "\n" + line + "\n"}
}

// errRun gives a passing output for pmset and ioreg, with an error.
func errRun(name string, args ...string) (string, error) {
	return "Now drawing from 'AC Power'\n\"AppleClamshellState\" = No\n", errors.New(name + ": exit status 1")
}

func TestPowerRule(t *testing.T) {
	for _, line := range oneByteEdits("Now drawing from 'AC Power'") {
		for _, out := range outputs(line, " -InternalBattery-0 (id=4653155)\t100%; charged; 0:00 remaining present: true") {
			want := Check{Name: "power", OK: strings.Contains(out, "'AC Power'")}
			if !want.OK {
				want.Detail = "the Mac runs on battery"
			}
			if c := Power(fakeRun(map[string]string{"pmset": out})); c != want {
				t.Fatalf("%q: got %+v, want %+v", out, c, want)
			}
		}
	}
}

func TestLidRule(t *testing.T) {
	for _, line := range oneByteEdits("\"AppleClamshellState\" = Yes") {
		for _, out := range outputs(line, "+-o IOPMrootDomain  <class IOPMrootDomain>") {
			want := Check{Name: "lid", OK: !strings.Contains(out, "\"AppleClamshellState\" = Yes")}
			if !want.OK {
				want.Detail = "the lid is closed"
			}
			if c := Lid(fakeRun(map[string]string{"ioreg": out})); c != want {
				t.Fatalf("%q: got %+v, want %+v", out, c, want)
			}
		}
	}
	for _, c := range [][2]Check{
		{Power(fakeRun(nil)), {"power", false, "pmset: executable file not found in $PATH"}},
		{Lid(fakeRun(nil)), {"lid", false, "ioreg: executable file not found in $PATH"}},
		{Power(errRun), {"power", false, "pmset: exit status 1"}},
		{Lid(errRun), {"lid", false, "ioreg: exit status 1"}},
		{Power(fakeRun(map[string]string{"pmset": ""})), {"power", false, "the Mac runs on battery"}},
		{Lid(fakeRun(map[string]string{"ioreg": ""})), {Name: "lid", OK: true}},
	} {
		if c[0] != c[1] {
			t.Errorf("fixed output: got %+v, want %+v", c[0], c[1])
		}
	}
}

// putConfig writes body as the relay config of home, or removes it when body is empty.
func putConfig(t *testing.T, home, body string) {
	t.Helper()
	p := relay.ConfigPath(home)
	if err := os.RemoveAll(p); err != nil {
		t.Fatal(err)
	} else if body == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRelayConfigCheck(t *testing.T) {
	home := t.TempDir()
	run, p := filepath.Join(home, ".planning", "source", "s"), relay.ConfigPath(home)
	other := filepath.Join(home, ".planning", "source", "t")
	names := func(proc string) string { return p + " names " + proc + ", not this run" }
	for _, c := range []struct{ body, detail string }{
		{"", ""},
		{`{"procedure": "` + run + `/relay/brain.md"}`, ""},
		{`{"procedure": ".planning/source/s/relay/brain.md", "threshold_pct": 50}`, ""},
		{`{"procedure": ".planning/gang/x/run/relay.md"}`, names(home + "/.planning/gang/x/run/relay.md")},
		{`{"procedure": "` + run + `/relay/dev.md"}`, names(run + "/relay/dev.md")},
		{`{"procedure": "` + other + `/relay/brain.md"}`, names(other + "/relay/brain.md")},
		{`{"procedure": "` + run + `/relay/brain.md.bak"}`, names(run + "/relay/brain.md.bak")},
		{`{"procedure": "` + run + `/relay"}`, names(run + "/relay")},
		{`{"procedure": "` + run[1:] + `/relay/brain.md"}`, names(home + run + "/relay/brain.md")},
		{`{}`, names(home + "/.planning/handoff-relay.md")},
		{"{", "relay config " + p + ": unexpected end of JSON input"},
	} {
		putConfig(t, home, c.body)
		want := Check{Name: "relay config", OK: c.detail == "", Detail: c.detail}
		if got := RelayConfig(home, run); got != want {
			t.Errorf("config %q: got %+v, want %+v", c.body, got, want)
		}
	}
	// Every one-byte edit of the path below home, home in upper case, and every
	// tail of the path that starts at a slash (each outside home).
	exact := filepath.Join(run, "relay", "brain.md")
	procs := []string{strings.ToUpper(home) + exact[len(home):]}
	for _, s := range oneByteEdits(exact[len(home):]) {
		procs = append(procs, home+s)
	}
	for i := 1; i < len(exact); i++ {
		if exact[i] == '/' {
			procs = append(procs, exact[i:])
		}
	}
	for _, proc := range procs {
		b, err := json.Marshal(map[string]string{"procedure": proc})
		if err != nil {
			t.Fatal(err)
		}
		putConfig(t, home, string(b))
		want := Check{Name: "relay config", OK: proc == exact}
		if !want.OK {
			want.Detail = names(proc)
		}
		if got := RelayConfig(home, run); got != want {
			t.Fatalf("procedure %q: got %+v, want %+v", proc, got, want)
		}
	}
	// A config path that is a folder reads as no file found, with an error.
	putConfig(t, home, "")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	_, found, rerr := relay.ReadConfig(home)
	if got := RelayConfig(home, run); found || rerr == nil || got != (Check{"relay config", false, rerr.Error()}) {
		t.Errorf("config folder: got %+v, read found %v, err %v", got, found, rerr)
	}
}

func TestAllChecks(t *testing.T) {
	home := t.TempDir()
	runDir, p := filepath.Join(home, ".planning", "source", "s"), relay.ConfigPath(home)
	outs := map[string]string{"go": "go version go1.24.2 darwin/arm64\n", "node": "v22.11.0\n",
		"git": "git version 2.47.1\n", "claude": "2.1.300 (Claude Code)\n", "gh": "gh version 2.63.0\n",
		"pmset": "Now drawing from 'AC Power'\n", "ioreg": "\"AppleClamshellState\" = No\n"}
	var calls []string
	run := func(name string, args ...string) (string, error) {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return fakeRun(outs)(name, args...)
	}
	putConfig(t, home, `{"procedure": "`+runDir+`/relay/brain.md"}`)
	want := append(Tools(fakeRun(outs)), Check{Name: "power", OK: true}, Check{Name: "lid", OK: true}, Check{Name: "relay config", OK: true})
	if got := All(run, home, runDir); !slices.Equal(got, want) {
		t.Fatalf("all pass: got %+v, want %+v", got, want)
	}
	wantCalls := "go version|node --version|git --version|claude --version|gh --version|pmset -g batt|ioreg -r -k AppleClamshellState"
	if strings.Join(calls, "|") != wantCalls {
		t.Fatalf("calls: got %q, want %q", calls, wantCalls)
	}

	outs["pmset"], outs["ioreg"] = "Now drawing from 'Battery Power'\n", "\"AppleClamshellState\" = Yes\n"
	putConfig(t, home, `{"procedure": ".planning/gang/x/run/relay.md"}`)
	want = append(Tools(fakeRun(outs)), Check{"power", false, "the Mac runs on battery"}, Check{"lid", false, "the lid is closed"},
		Check{"relay config", false, p + " names " + home + "/.planning/gang/x/run/relay.md, not this run"})
	if got := All(fakeRun(outs), home, runDir); !slices.Equal(got, want) {
		t.Fatalf("all fail: got %+v, want %+v", got, want)
	}
}
