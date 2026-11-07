package main

import (
	"path/filepath"
	"strings"
	"testing"

	"source/internal/exit"
	"source/internal/relay"
)

// toolCalls is every program call that the checks of preflight.All make, in order.
const toolCalls = "go version|node --version|git --version|claude --version|gh --version|pmset -g batt|ioreg -r -k AppleClamshellState|"

func TestPreflightCommand(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "auto") // the default runTool sets GOTOOLCHAIN=local and keeps stderr
	if out, err := runTool("/bin/sh", "-c", `printf %s "$GOTOOLCHAIN"; printf ' err' >&2; exit 3`); err == nil || out != "local err" || func() bool { _, e := runTool("/bin/sh", "-c", "exit 0"); return e != nil }() {
		t.Errorf("default runTool: out %q, err %v (and exit 0 must give no error)", out, err)
	}
	run := newRun(t, "demo")
	runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1")
	home := filepath.Dir(filepath.Dir(filepath.Dir(run)))
	outs := map[string]string{"go": "go version go1.24.1 darwin/arm64\n", "node": "v22.3.0\n", "git": "git version 2.47.0\n",
		"claude": "2.1.0\n", "gh": "gh version 2.60.0\n", "pmset": "Now drawing from 'AC Power'\n", "ioreg": ""}
	var calls strings.Builder
	old := runTool
	t.Cleanup(func() { runTool = old })
	runTool = func(name string, args ...string) (string, error) {
		calls.WriteString(strings.Join(append([]string{name}, args...), " ") + "|")
		return outs[name], nil
	}
	err := relay.WriteConfig(home, filepath.Join(run, "relay", "brain.md")) // a config that names this run passes
	if out, e, code := runSource(t, &Ctx{Run: run}, "preflight"); err != nil || code != exit.OK || out != "preflight: clean\n" || e != "" {
		t.Errorf("clean: err %v, code %d, stdout %q, stderr %q", err, code, out, e)
	}
	outs["go"], outs["pmset"] = "go version go1.23.4 darwin/arm64\n", "Now drawing from 'Battery Power'\n"
	if err := relay.WriteConfig(home, "/elsewhere/relay/brain.md"); err != nil {
		t.Fatal(err)
	}
	want := "FAIL go: go1.23 is older than go1.24\nFAIL power: the Mac runs on battery\n" +
		"FAIL relay config: " + relay.ConfigPath(home) + " names /elsewhere/relay/brain.md, not this run\n"
	if out, e, code := runSource(t, &Ctx{Run: run}, "preflight"); code != exit.Refused || out != want || e != "" {
		t.Errorf("failing: code %d, stdout %q, stderr %q", code, out, e)
	}
	if got := listRun(t, filepath.Join(run, "locks")); got != "brain.json" {
		t.Errorf("locks hold %q after preflight, want only the brain lease of init", got)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "preflight", "x"); code != exit.Usage || e != "monet-source: usage: monet-source preflight\n" {
		t.Errorf("preflight x: code %d, stderr %q", code, e)
	}
	if out, _, code := runSource(t, &Ctx{Run: realTempDir(t)}, "preflight"); code != exit.Refused || out != "" {
		t.Errorf("odd run: code %d, stdout %q", code, out)
	}
	if got := calls.String(); got != toolCalls+toolCalls {
		t.Errorf("calls %q, want the local checks twice", got)
	}
}
