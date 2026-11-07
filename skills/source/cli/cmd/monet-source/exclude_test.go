package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"source/internal/exit"
)

// excludeText adds each excludeLines entry that no line of old equals, after
// one newline when old ends without one, checked against a plain reference.
func TestExcludeText(t *testing.T) {
	var olds []string
	for _, w := range excludeLines {
		for _, e := range oneByteEdits(w) {
			olds = append(olds, e+"\n", "# x\n"+e)
		}
	}
	olds = append(olds, "", "x", excludeLines[0]+"\n", excludeLines[1]+"\n"+excludeLines[0]+"\n", excludeLines[0]+"\n"+excludeLines[1])
	for _, old := range olds {
		want := ""
		for _, w := range excludeLines {
			if !slices.Contains(strings.Split(old, "\n"), w) {
				want += w + "\n"
			}
		}
		if want != "" && old != "" && !strings.HasSuffix(old, "\n") {
			want = "\n" + want
		}
		if got := excludeText(old); got != want {
			t.Fatalf("old %q: got %q, want %q", old, got, want)
		}
	}
}

// init writes the two lines to <home>/.git/info/exclude when <home>/.git is a
// folder, and writes nothing when it is a file or is absent.
func TestInitExcludesRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(git string) error
		want string // "" means no exclude file
	}{
		{"folder", func(git string) error { return os.Mkdir(git, 0o755) }, "/.planning/source/\n/.claude/handoff-relay.json\n"},
		{"kept", func(git string) error { // the old bytes stay, and a missing newline comes first
			return errors.Join(os.MkdirAll(filepath.Join(git, "info"), 0o755), os.WriteFile(filepath.Join(git, "info", "exclude"), []byte("*.o"), 0o644))
		}, "*.o\n/.planning/source/\n/.claude/handoff-relay.json\n"},
		{"file", func(git string) error { return os.WriteFile(git, []byte("gitdir: x\n"), 0o644) }, ""},
		{"none", func(string) error { return nil }, ""},
		{"link", func(git string) error { return errors.Join(os.Mkdir(git+"-to", 0o755), os.Symlink(git+"-to", git)) }, ""},
	} {
		run := newRun(t, "demo")
		git := filepath.Join(run, "../../..", ".git")
		if err := tc.make(git); err != nil {
			t.Fatal(err)
		}
		if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK || e != "" {
			t.Fatalf("%s: code %d, stderr %q", tc.name, code, e)
		}
		got, err := os.ReadFile(filepath.Join(git, "info", "exclude"))
		if (tc.want == "" && err == nil) || (tc.want != "" && (err != nil || string(got) != tc.want)) {
			t.Errorf("%s: exclude %q (%v), want %q", tc.name, got, err, tc.want)
		}
		data, gerr := os.ReadFile(git)
		if tc.name == "file" && string(data) != "gitdir: x\n" || tc.name == "none" && !os.IsNotExist(gerr) {
			t.Errorf("%s: .git holds %q (%v)", tc.name, data, gerr)
		}
	}
	// A folder at info/exclude fails excludeRun: init is exit 1 after the config write, with no state.json.
	run := newRun(t, "demo")
	if err := os.MkdirAll(filepath.Join(run, "../../..", ".git", "info", "exclude", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1")
	_, config, r5 := relayFiles(run)
	if got, _ := os.ReadFile(config); code != exit.User || e == "" || strings.Contains(listRun(t, run), "state.json") || string(got) != r5 {
		t.Errorf("failed exclude: code %d, stderr %q, run holds %q, config %q", code, e, listRun(t, run), got)
	}
}
