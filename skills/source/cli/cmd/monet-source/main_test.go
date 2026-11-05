package main

import (
	"bytes"
	"errors"
	"io"
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

// A --run must name an existing folder, and with no run dir the command does
// not run.
func TestDispatchMissingRun(t *testing.T) {
	t.Setenv("SOURCE_RUN", "")
	t.Setenv("SOURCE_SKILL", "")
	var got []string
	commands["t-run"] = func(c *Ctx, args []string) error { got = append([]string{c.Run, c.SkillDir}, args...); return nil }
	t.Cleanup(func() { delete(commands, "t-run") })
	root := realTempDir(t)
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

// realTempDir returns a new temp folder with its symlinks resolved.
func realTempDir(t *testing.T) string {
	t.Helper()
	d, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// withRunProbe registers the test command t-run, which prints c.Run.
func withRunProbe(t *testing.T) {
	t.Helper()
	commands["t-run"] = func(c *Ctx, _ []string) error { _, err := io.WriteString(c.Stdout, c.Run); return err }
	t.Cleanup(func() { delete(commands, "t-run") })
}

// The engine stores and prints real paths, so a --run through a symlinked
// folder, absolute or relative, resolves to the real folder.
func TestResolveRunRealPath(t *testing.T) {
	withRunProbe(t)
	root := realTempDir(t)
	want := filepath.Join(root, "real", "run")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	out, e, code := runSource(t, &Ctx{Run: filepath.Join(root, "link", "run")}, "t-run")
	if code != exit.OK || out != want {
		t.Fatalf("code %d, stderr %q, run %q, want %q", code, e, out, want)
	}
	t.Chdir(filepath.Join(root, "link"))
	if out, e, code := runSource(t, &Ctx{Run: "run"}, "t-run"); code != exit.OK || out != want {
		t.Fatalf("relative: code %d, stderr %q, run %q, want %q", code, e, out, want)
	}
	t.Setenv("SOURCE_RUN", "run")
	if out, e, code := runSource(t, nil, "t-run"); code != exit.OK || out != want {
		t.Fatalf("relative SOURCE_RUN: code %d, stderr %q, run %q, want %q", code, e, out, want)
	}
}

// The run dir comes from --run, then SOURCE_RUN, then <dir> when the binary
// is <dir>/bin/monet-source. Each source gives the real path, and a source
// that names an absent folder is refused, with no fallback to the next one.
func TestResolveRunOrder(t *testing.T) {
	withRunProbe(t)
	root := realTempDir(t)
	flagDir, envDir, home := filepath.Join(root, "flag"), filepath.Join(root, "env"), filepath.Join(root, "home")
	for _, d := range []string{flagDir, envDir, home} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range [][2]string{{envDir, "envlink"}, {home, "homelink"}} {
		if err := os.Symlink(l[0], filepath.Join(root, l[1])); err != nil {
			t.Fatal(err)
		}
	}
	envLink, absent := filepath.Join(root, "envlink"), filepath.Join(root, "absent")
	bin := filepath.Join(root, "homelink", "bin", "monet-source")
	gone := "monet-source: run dir " + absent + " does not exist\n"
	old := executable
	t.Cleanup(func() { executable = old })
	for _, tc := range []struct {
		name, flag, env, exe string
		exeErr               error
		want, err            string
	}{
		{"flag over env", flagDir, envLink, bin, nil, flagDir, ""},
		{"flag over stale env", flagDir, absent, bin, nil, flagDir, ""},
		{"env over binary", "", envLink, bin, nil, envDir, ""},
		{"env over failing binary", "", envLink, bin, errors.New("no path"), envDir, ""},
		{"binary", "", "", bin, nil, home, ""},
		{"absent flag, no fallback", absent, envDir, bin, nil, "", gone},
		{"absent env, no fallback", "", absent, bin, nil, "", gone},
		{"other binary name", "", "", filepath.Join(home, "bin", "monet-source.test"), nil, "", ""},
		{"binary not in bin", "", "", filepath.Join(home, "sbin", "monet-source"), nil, "", ""},
		{"executable fails", "", "", bin, errors.New("no path"), "", ""},
	} {
		t.Setenv("SOURCE_RUN", tc.env)
		executable = func() (string, error) { return tc.exe, tc.exeErr }
		out, e, code := runSource(t, &Ctx{Run: tc.flag}, "t-run")
		wantCode, wantErr := exit.OK, ""
		if tc.want == "" {
			wantCode, wantErr = exit.Usage, tc.err
		}
		if tc.want == "" && tc.err == "" {
			wantErr = "monet-source: no run dir: pass --run, set SOURCE_RUN, or run <dir>/bin/monet-source\n"
		}
		if code != wantCode || e != wantErr || out != tc.want {
			t.Errorf("%s: code %d, stderr %q, run %q, want %q", tc.name, code, e, out, tc.want)
		}
	}
}

// The binary source reads the path of the running binary by default.
func TestExecutableDefault(t *testing.T) {
	got, gerr := executable()
	want, werr := os.Executable()
	if got != want || (gerr == nil) != (werr == nil) {
		t.Fatalf("executable() = %q, %v; os.Executable() = %q, %v", got, gerr, want, werr)
	}
}

// The skill dir comes from --skill, then SOURCE_SKILL, then the built-in
// skillDir, and needSkill refuses when none is set.
func TestSkillDirOrder(t *testing.T) {
	commands["t-skill"] = func(c *Ctx, _ []string) error {
		dir, err := needSkill(c)
		if err == nil {
			_, err = io.WriteString(c.Stdout, dir)
		}
		return err
	}
	t.Cleanup(func() { delete(commands, "t-skill") })
	old := skillDir
	t.Cleanup(func() { skillDir = old })
	run := realTempDir(t)
	for _, tc := range []struct{ flag, env, built, want string }{
		{"/flag", "/env", "/built", "/flag"},
		{"", "/env", "/built", "/env"},
		{"", "", "/built", "/built"},
		{"", "", "", ""},
	} {
		t.Setenv("SOURCE_SKILL", tc.env)
		skillDir = tc.built
		out, e, code := runSource(t, &Ctx{Run: run, SkillDir: tc.flag}, "t-skill")
		wantCode, wantErr := exit.OK, ""
		if tc.want == "" {
			wantCode, wantErr = exit.Usage, "monet-source: no skill dir: pass --skill or set SOURCE_SKILL\n"
		}
		if code != wantCode || e != wantErr || out != tc.want {
			t.Errorf("%+v: code %d, stderr %q, out %q", tc, code, e, out)
		}
	}
}

// Flags parse before, between, or after the positionals, and "--" ends flag
// parsing, so a flag-shaped word after it stays a positional.
func TestParseFlagsAnywhere(t *testing.T) {
	for _, tc := range []struct {
		args, pos []string
		x         string
		c         bool
	}{
		{[]string{"a", "--x", "1", "b", "--", "--c"}, []string{"a", "b", "--c"}, "1", false},
		{[]string{"--x", "1", "a", "b", "--c"}, []string{"a", "b"}, "1", true},
		{[]string{"--x", "1", "--c"}, nil, "1", true},
		{[]string{"a", "--", "b", "--c"}, []string{"a", "b", "--c"}, "", false},
	} {
		fs := newFlagSet("t")
		x, c := fs.String("x", "", ""), fs.Bool("c", false, "")
		pos, err := parseFlags(fs, tc.args)
		if err != nil || !reflect.DeepEqual(pos, tc.pos) || *x != tc.x || *c != tc.c {
			t.Errorf("%q: pos %q, x %q, c %v, err %v", tc.args, pos, *x, *c, err)
		}
	}
	_, err := parseFlags(newFlagSet("t"), []string{"a", "--nope"})
	var e *exit.Error
	if !errors.As(err, &e) || e.Code != exit.Usage || e.Msg != "t: flag provided but not defined: -nope" {
		t.Fatalf("bad flag: err %v", err)
	}
}

// A multi-word command dispatches on its first argument. No subcommand or an
// unknown one is a usage error that lists the subcommands in sorted order.
func TestSubcommands(t *testing.T) {
	var got []string
	probe := func(name string) func(*Ctx, []string) error {
		return func(_ *Ctx, args []string) error { got = append([]string{name}, args...); return nil }
	}
	// p shows that the handler gets the Ctx and that its error is returned.
	p := func(c *Ctx, _ []string) error { io.WriteString(c.Stdout, c.Run); return exit.New(exit.Refused, "no") }
	commands["t-sub"] = subcommands("t-sub", map[string]func(c *Ctx, args []string) error{"q": probe("q"), "p": p})
	t.Cleanup(func() { delete(commands, "t-sub") })
	run := realTempDir(t)
	if _, e, code := runSource(t, &Ctx{Run: run}, "t-sub", "q", "a", "--b"); code != exit.OK || !reflect.DeepEqual(got, []string{"q", "a", "--b"}) {
		t.Fatalf("q: code %d, stderr %q, got %q", code, e, got)
	}
	if out, e, code := runSource(t, &Ctx{Run: run}, "t-sub", "p"); code != exit.Refused || out != run || e != "monet-source: no\n" {
		t.Fatalf("p: code %d, stdout %q, stderr %q", code, out, e)
	}
	got = nil
	// Map order is random, so the repeats pin the sort.
	for i := 0; i < 20; i++ {
		if _, e, code := runSource(t, &Ctx{Run: run}, "t-sub"); code != exit.Usage || e != "monet-source: t-sub: want a subcommand: p, q\n" {
			t.Fatalf("none: code %d, stderr %q", code, e)
		}
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "t-sub", "r"); code != exit.Usage || e != "monet-source: t-sub: unknown subcommand \"r\": want p, q\n" || got != nil {
		t.Fatalf("unknown: code %d, stderr %q, got %q", code, e, got)
	}
}
