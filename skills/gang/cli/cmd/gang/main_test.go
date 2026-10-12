package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/store"
)

// runGang runs dispatch with captured output. A non-empty c.Dir becomes a
// leading --dir flag and a non-empty c.SkillDir a leading --skill flag.
// c.Stdin defaults to empty input and c.Now to 2026-10-11T12:00:00Z.
// c may be nil. The caller's Ctx is not modified.
func runGang(t *testing.T, c *Ctx, args ...string) (stdout, stderr string, code int) {
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
		cc.Now = func() time.Time { return time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC) }
	}
	var global []string
	if cc.Dir != "" {
		global = append(global, "--dir", cc.Dir)
	}
	if cc.SkillDir != "" {
		global = append(global, "--skill", cc.SkillDir)
	}
	code = dispatch(&cc, append(global, args...))
	return out.String(), errb.String(), code
}

// gitT runs git in dir and fails the test on error.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitx.Run(dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// newTestRepo makes a git repo on branch main with local user.name and
// user.email, signing off, global excludes off (a machine-wide ignore rule
// cannot hide files from a test), and one commit holding README.md
// ("test repo\n"). The path has its symlinks resolved, as git reports them.
func newTestRepo(t *testing.T) (repo string) {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "init", "--quiet", "--initial-branch=main")
	gitT(t, repo, "config", "user.name", "Gang Test")
	gitT(t, repo, "config", "user.email", "gang-test@example.com")
	gitT(t, repo, "config", "commit.gpgsign", "false")
	gitT(t, repo, "config", "core.excludesFile", os.DevNull)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("test repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", "README.md")
	gitT(t, repo, "commit", "--quiet", "--no-verify", "-m", "initial")
	return repo
}

// newTestGangDir creates <repo>/.planning/gang/<p.Slug>/ with plan/plan.json
// (written by plan.Save) and an empty run/, and returns the gang dir.
func newTestGangDir(t *testing.T, repo string, p *plan.Plan) string {
	t.Helper()
	dir := filepath.Join(repo, ".planning", "gang", p.Slug)
	if err := os.MkdirAll(filepath.Join(dir, "run"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := plan.Save(store.Dir{Root: dir}.Plan(), p); err != nil {
		t.Fatal(err)
	}
	return dir
}

// testPlan returns a lint-clean two-task plan with slug "demo". B1 depends
// on A1, holds lock "db", and has risk high. A1 is wave 1, B1 is wave 2.
func testPlan() *plan.Plan {
	p := &plan.Plan{
		Version:  1,
		Slug:     "demo",
		Repo:     plan.Repo{Remote: "github.com/example/demo", Base: "main", Integration: "gang-demo", GHAccount: "example"},
		Commands: plan.Commands{Gate: []string{"true"}, Health: []string{"true"}, WaveGate: []string{"true"}},
		Locks:    []string{"db"},
		Done:     "every task is reachable from origin/main and no wave PR is open",
		Requirements: []plan.Requirement{
			{ID: "REQ-1", Source: "spec.md:1"},
			{ID: "REQ-2", Source: "spec.md:2"},
		},
		Tasks: []plan.Task{
			{
				ID: "A1", Title: "add alpha", Goal: "Add alpha/a.txt.",
				Scope:        plan.Scope{Write: []string{"alpha/**"}},
				Requirements: []string{"REQ-1"},
				Interfaces:   plan.Interfaces{Produces: []string{"file:alpha/a.txt"}},
				Acceptance:   []string{"alpha/a.txt exists"},
				Verify:       plan.Verify{TestsFirst: []string{"test:alpha"}},
				Size:         plan.Size{Lines: 200}, TimeboxMin: 30, Wave: 1,
			},
			{
				ID: "B1", Title: "add beta", Goal: "Add beta/b.txt.",
				Scope:        plan.Scope{Write: []string{"beta/**"}},
				DependsOn:    []string{"A1"},
				Requirements: []string{"REQ-2"},
				Interfaces:   plan.Interfaces{Consumes: []string{"file:alpha/a.txt"}},
				Acceptance:   []string{"beta/b.txt exists"},
				Verify:       plan.Verify{TestsFirst: []string{"test:beta"}},
				Size:         plan.Size{Lines: 300}, TimeboxMin: 45, Locks: []string{"db"}, Risk: "high", Wave: 2,
			},
		},
	}
	p.Normalize()
	return p
}

// withProbe registers a "probe" command for one test.
func withProbe(t *testing.T, h func(c *Ctx, args []string) error) {
	t.Helper()
	commands["probe"] = h
	t.Cleanup(func() { delete(commands, "probe") })
}

// echoProbe prints the resolved context and the remaining args.
func echoProbe(c *Ctx, args []string) error {
	fmt.Fprintf(c.Stdout, "dir=%s\nrepo=%s\nskill=%s\nargs=%s\n", c.Dir, c.Repo, c.SkillDir, strings.Join(args, ","))
	return nil
}

func TestDispatchUsageErrors(t *testing.T) {
	t.Setenv("GANG_DIR", "")
	withProbe(t, echoProbe)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no command", nil, "usage: gang"},
		{"unknown command", []string{"nope"}, `unknown command "nope"`},
		{"bad global flag", []string{"--bogus", "probe"}, "flag provided but not defined"},
		{"no gang dir", []string{"probe"}, "no gang dir"},
		{"missing gang dir", []string{"--dir", filepath.Join(t.TempDir(), "missing"), "probe"}, "does not exist"},
		{"not a repo", []string{"--dir", t.TempDir(), "probe"}, "not in a git repo"},
	}
	for _, c := range cases {
		_, stderr, code := runGang(t, nil, c.args...)
		if code != exit.Usage || !strings.Contains(stderr, c.want) {
			t.Errorf("%s: code %d stderr %q, want 64 and %q", c.name, code, stderr, c.want)
		}
	}
}

func TestDispatchResolvesDirRepoAndSkill(t *testing.T) {
	repo := newTestRepo(t)
	dir := newTestGangDir(t, repo, testPlan())
	withProbe(t, echoProbe)
	t.Setenv("GANG_DIR", "")
	t.Setenv("GANG_SKILL", "")

	out, _, code := runGang(t, &Ctx{Dir: dir, SkillDir: "/skills/gang"}, "probe", "x", "--y")
	want := fmt.Sprintf("dir=%s\nrepo=%s\nskill=/skills/gang\nargs=x,--y\n", dir, repo)
	if code != 0 || out != want {
		t.Errorf("flags: code %d out %q, want %q", code, out, want)
	}

	t.Setenv("GANG_DIR", dir)
	t.Setenv("GANG_SKILL", "/env/skill")
	out, _, _ = runGang(t, nil, "probe")
	if !strings.HasPrefix(out, "dir="+dir+"\n") || !strings.Contains(out, "skill=/env/skill\n") {
		t.Errorf("env: out %q", out)
	}

	t.Setenv("GANG_DIR", "")
	t.Setenv("GANG_SKILL", "")
	oldExe, oldSkill := executable, skillDir
	t.Cleanup(func() { executable, skillDir = oldExe, oldSkill })
	executable = func() (string, error) { return filepath.Join(dir, "run", "bin", "gang"), nil }
	skillDir = "/built/skill"
	out, _, _ = runGang(t, nil, "probe")
	if !strings.HasPrefix(out, "dir="+dir+"\n") || !strings.Contains(out, "skill=/built/skill\n") {
		t.Errorf("executable location: out %q", out)
	}
}

func TestDispatchRepoIsMainCheckoutFromWorktree(t *testing.T) {
	repo := newTestRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "w1")
	gitT(t, repo, "worktree", "add", "--quiet", "-b", "w1", wt)
	dir := newTestGangDir(t, wt, testPlan())
	withProbe(t, echoProbe)
	out, _, code := runGang(t, &Ctx{Dir: dir}, "probe")
	if code != 0 || !strings.Contains(out, "repo="+repo+"\n") {
		t.Errorf("code %d out %q, want repo=%s", code, out, repo)
	}
}

func TestDispatchMapsErrors(t *testing.T) {
	repo := newTestRepo(t)
	dir := newTestGangDir(t, repo, testPlan())
	cases := []struct {
		name   string
		err    error
		code   int
		stderr string
	}{
		{"nil", nil, 0, ""},
		{"exit error", exit.New(exit.Held, "lock %s held", "driver"), 4, "gang: lock driver held\n"},
		{"wrapped exit error", fmt.Errorf("merge: %w", exit.New(exit.Conflict, "trunk moved")), 3, "gang: merge: trunk moved\n"},
		{"silent exit error", exit.New(exit.User, ""), 1, ""},
		{"plain error", errors.New("boom"), 1, "gang: boom\n"},
	}
	for _, c := range cases {
		withProbe(t, func(*Ctx, []string) error { return c.err })
		_, stderr, code := runGang(t, &Ctx{Dir: dir}, "probe")
		if code != c.code || stderr != c.stderr {
			t.Errorf("%s: code %d stderr %q, want %d %q", c.name, code, stderr, c.code, c.stderr)
		}
	}
}

func TestParseFlagsAllowsInterspersedFlags(t *testing.T) {
	fs := newFlagSet("ledger record")
	lens := fs.String("lens", "", "")
	gate := fs.Bool("gate-ran", false, "")
	pos, err := parseFlags(fs, []string{"A1", "--lens", "review", "abc", "--", "--literal", "--gate-ran"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pos, []string{"A1", "abc", "--literal", "--gate-ran"}) || *lens != "review" || *gate {
		t.Errorf("pos %v lens %q gate %v, want --gate-ran kept as a positional after --", pos, *lens, *gate)
	}
	_, err = parseFlags(newFlagSet("x"), []string{"--nope"})
	var e *exit.Error
	if !errors.As(err, &e) || e.Code != exit.Usage {
		t.Errorf("bad flag: err %v, want a usage error", err)
	}
}

func TestSubcommands(t *testing.T) {
	repo := newTestRepo(t)
	dir := newTestGangDir(t, repo, testPlan())
	h := subcommands("probe", map[string]func(c *Ctx, args []string) error{
		"one": echoProbe,
		"two": echoProbe,
	})
	withProbe(t, h)
	out, _, code := runGang(t, &Ctx{Dir: dir}, "probe", "one", "a")
	if code != 0 || !strings.HasSuffix(out, "args=a\n") {
		t.Errorf("one: code %d out %q", code, out)
	}
	_, stderr, code := runGang(t, &Ctx{Dir: dir}, "probe")
	if code != exit.Usage || !strings.Contains(stderr, "want a subcommand: one, two") {
		t.Errorf("none: code %d stderr %q", code, stderr)
	}
	_, stderr, code = runGang(t, &Ctx{Dir: dir}, "probe", "three")
	if code != exit.Usage || !strings.Contains(stderr, `unknown subcommand "three"`) {
		t.Errorf("unknown: code %d stderr %q", code, stderr)
	}
}

func TestTestPlanHelpersWriteAPlanThatLoads(t *testing.T) {
	repo := newTestRepo(t)
	dir := newTestGangDir(t, repo, testPlan())
	if want := filepath.Join(repo, ".planning", "gang", "demo"); dir != want {
		t.Errorf("gang dir %s, want %s", dir, want)
	}
	p, err := plan.Load(store.Dir{Root: dir}.Plan())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, testPlan()) {
		t.Error("the saved plan does not load back equal to testPlan()")
	}
	if fi, err := os.Stat(store.Dir{Root: dir}.Run()); err != nil || !fi.IsDir() {
		t.Errorf("run/ missing: %v", err)
	}
	if branch := gitT(t, repo, "branch", "--show-current"); branch != "main" {
		t.Errorf("branch %q, want main", branch)
	}
}

func TestDecodeStrict(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]bool{
		`{"a":1}`:        true,
		`{"a":1}` + "\n": true,
		`{"a":1,"b":2}`:  false,
		`{"a":1} {}`:     false,
		`{"a":`:          false,
	}
	for content, ok := range cases {
		path := filepath.Join(dir, "v.json")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		var v struct{ A int }
		if err := decodeStrict(path, &v); (err == nil) != ok {
			t.Errorf("decodeStrict(%q) = %v, want ok %v", content, err, ok)
		}
	}
}
