package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/exit"
)

// testNow is the clock of every test that runs through runSource.
var testNow = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

// wantUsage is the usage line that the CLI contract fixes, typed out so that
// a change to usageLine fails a test.
const wantUsage = "usage: monet-source [--run <run dir>] [--skill <skill dir>] <command> [args]"

// runSource runs dispatch with captured output. A non-empty c.Run becomes a
// leading --run flag and a non-empty c.SkillDir a leading --skill flag.
// c.Stdin defaults to empty input and c.Now to testNow. c may be nil. The
// caller's Ctx is not modified.
func runSource(t *testing.T, c *Ctx, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var cc Ctx
	if c != nil {
		cc = *c
	}
	var out, errb bytes.Buffer
	cc.Stdout, cc.Stderr = &out, &errb
	if cc.Stdin == nil {
		cc.Stdin = strings.NewReader("")
	}
	if cc.Now == nil {
		cc.Now = func() time.Time { return testNow }
	}
	var global []string
	if cc.Run != "" {
		global = append(global, "--run", cc.Run)
	}
	if cc.SkillDir != "" {
		global = append(global, "--skill", cc.SkillDir)
	}
	code = dispatch(&cc, append(global, args...))
	return out.String(), errb.String(), code
}

func TestDispatchUsage(t *testing.T) {
	_, errOut, code := runSource(t, nil)
	if code != exit.Usage || errOut != "monet-source: "+wantUsage+"\n" {
		t.Fatalf("code %d, stderr %q", code, errOut)
	}
}

// The global flags are --run and --skill. Any other flag before the command is
// a usage error, and the command does not run.
func TestDispatchUnknownGlobalFlag(t *testing.T) {
	ran := false
	commands["t-ok"] = func(*Ctx, []string) error { ran = true; return nil }
	t.Cleanup(func() { delete(commands, "t-ok") })
	_, e, code := runSource(t, &Ctx{Run: t.TempDir()}, "--dir", "x", "t-ok")
	if want := "monet-source: flag provided but not defined: -dir\n" + wantUsage + "\n"; code != exit.Usage || e != want || ran {
		t.Fatalf("code %d, stderr %q, ran %v", code, e, ran)
	}
}

func TestDispatchMapsErrors(t *testing.T) {
	commands["t-refuse"] = func(*Ctx, []string) error { return exit.New(exit.Refused, "no") }
	commands["t-plain"] = func(*Ctx, []string) error { return errors.New("boom") }
	commands["t-silent"] = func(*Ctx, []string) error { return &exit.Error{Code: exit.Held} }
	t.Cleanup(func() { delete(commands, "t-refuse"); delete(commands, "t-plain"); delete(commands, "t-silent") })
	run := t.TempDir()
	if _, e, code := runSource(t, &Ctx{Run: run}, "t-refuse"); code != exit.Refused || e != "monet-source: no\n" {
		t.Fatalf("refuse: code %d, stderr %q", code, e)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "t-plain"); code != exit.User || e != "monet-source: boom\n" {
		t.Fatalf("plain: code %d, stderr %q", code, e)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "t-silent"); code != exit.Held || e != "" {
		t.Fatalf("silent: code %d, stderr %q", code, e)
	}
}

// An unknown command is refused before the run dir is read, so no --run is needed.
func TestDispatchUnknownCommand(t *testing.T) {
	_, e, code := runSource(t, nil, "nope")
	if code != exit.Usage || !strings.Contains(e, `unknown command "nope"`) {
		t.Fatalf("code %d, stderr %q", code, e)
	}
}

// --run is the only source of the run dir, and it must name an existing folder.
func TestDispatchMissingRun(t *testing.T) {
	var got []string
	commands["t-run"] = func(c *Ctx, args []string) error { got = append([]string{c.Run, c.SkillDir}, args...); return nil }
	t.Cleanup(func() { delete(commands, "t-run") })
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, run := range []string{filepath.Join(root, "absent"), file} {
		if _, e, code := runSource(t, &Ctx{Run: run}, "t-run"); code != exit.Usage || !strings.Contains(e, "does not exist") {
			t.Fatalf("run %s: code %d, stderr %q", run, code, e)
		}
	}
	if _, e, code := runSource(t, nil, "t-run"); code != exit.Usage || !strings.Contains(e, "no run dir: pass --run") {
		t.Fatalf("no flag: code %d, stderr %q", code, e)
	}
	if got != nil {
		t.Fatalf("the command ran without a run dir: %q", got)
	}
	if _, e, code := runSource(t, &Ctx{Run: root, SkillDir: "/flag"}, "t-run", "a", "--b"); code != exit.OK || !reflect.DeepEqual(got, []string{root, "/flag", "a", "--b"}) {
		t.Fatalf("existing run: code %d, stderr %q, got %q", code, e, got)
	}
	old := skillDir
	skillDir = "/built"
	t.Cleanup(func() { skillDir = old })
	if _, _, code := runSource(t, &Ctx{Run: root}, "t-run"); code != exit.OK || !reflect.DeepEqual(got, []string{root, "/built"}) {
		t.Fatalf("built-in skill dir: code %d, got %q", code, got)
	}
}
