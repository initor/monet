package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"source/internal/exit"
	"source/internal/lock"
	"source/internal/state"
	"source/internal/store"
	"source/internal/testkit"
)

// wantInitUsage is the init usage line, typed out so that a change to
// initUsage fails a test.
const wantInitUsage = "usage: monet-source init --slug <slug> --holder <session>"

// newRun makes the folder <temp>/.planning/source/<name>, sets SOURCE_SKILL to a
// new testkit.Skill, and returns the real path of the folder.
func newRun(t *testing.T, name string) string {
	t.Helper()
	t.Setenv("SOURCE_SKILL", testkit.Skill(t))
	run := filepath.Join(realTempDir(t), ".planning", "source", name)
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	return run
}

// listRun returns the sorted names in run, joined by spaces, with "/" after
// each folder.
func listRun(t *testing.T, run string) string {
	t.Helper()
	ents, err := os.ReadDir(run)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		if names = append(names, e.Name()); e.IsDir() {
			names[len(names)-1] += "/"
		}
	}
	return strings.Join(names, " ")
}

// init makes exactly the run folders and state.json, prints nothing, and
// gives brain to the holder for 12 hours.
func TestInitLayout(t *testing.T) {
	run := newRun(t, "demo")
	out, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1")
	if code != exit.OK || out != "" || e != "" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out, e)
	}
	if got, want := listRun(t, run), "artifacts/ bin/ dev/ handoff/ items/ locks/ notes/ personas/ relay/ rounds/ state.json watch/"; got != want {
		t.Errorf("run holds %q, want %q", got, want)
	}
	for _, name := range runFolders { // each folder holds no folder, and the holder can write into it
		if got := listRun(t, filepath.Join(run, name)); strings.Contains(got, "/") {
			t.Errorf("%s holds %q", name, got)
		}
		if err := os.WriteFile(filepath.Join(run, name, "probe"), nil, 0o644); err != nil {
			t.Errorf("write into %s: %v", name, err)
		}
	}
	d := store.Dir{Root: run}
	if s, err := state.Load(d); err != nil || s.Version != 1 || s.Slug != "demo" || !s.Created.Equal(testNow) || s.Phase != "" {
		t.Errorf("state %+v, err %v", s, err)
	}
	if l, ok, err := lock.Read(d.Locks(), "brain"); err != nil || !ok || l.Holder != "h1" || !l.Expires.Equal(testNow.Add(12*time.Hour)) {
		t.Errorf("brain lease %+v, ok %v, err %v", l, ok, err)
	}
}

// A second init, by the holder or by another session and later, is exit 2
// and changes neither state.json nor the lease. When another session holds
// brain, init is exit 4 and writes nothing.
func TestInitRefusesSecond(t *testing.T) {
	run := newRun(t, "demo")
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK {
		t.Fatalf("first init: code %d, stderr %q", code, e)
	}
	before, err := os.ReadFile(store.Dir{Root: run}.State())
	if err != nil {
		t.Fatal(err)
	}
	later := func() time.Time { return testNow.Add(time.Hour) }
	for _, h := range []string{"h1", "h2"} {
		_, e, code := runSource(t, &Ctx{Run: run, Now: later}, "init", "--slug", "demo", "--holder", h)
		after, err := os.ReadFile(store.Dir{Root: run}.State())
		if code != exit.Refused || e != "monet-source: run demo already has state\n" || err != nil || string(after) != string(before) {
			t.Errorf("%s: code %d, stderr %q, state %q (%v), want %q", h, code, e, after, err, before)
		}
		checkLease(t, run, "brain", testNow.Add(12*time.Hour))
	}
	held := newRun(t, "demo")
	if _, err := lock.Acquire(store.Dir{Root: held}.Locks(), "brain", "h2", 12*time.Hour, testNow); err != nil {
		t.Fatal(err)
	}
	_, e, code := runSource(t, &Ctx{Run: held}, "init", "--slug", "demo", "--holder", "h1")
	if got := listRun(t, held); code != exit.Held || !strings.Contains(e, "lock held: brain by h2") || got != "locks/" {
		t.Fatalf("held: code %d, stderr %q, run holds %q", code, e, got)
	}
	// A folder or a dangling symlink at state.json is state too, and stays.
	for _, tc := range []struct {
		make func(string) error
		list string
	}{
		{func(p string) error { return os.Mkdir(p, 0o755) }, "state.json/"},
		{func(p string) error { return os.Symlink("gone", p) }, "state.json"},
	} {
		entry := newRun(t, "demo")
		if err := tc.make(store.Dir{Root: entry}.State()); err != nil {
			t.Fatal(err)
		}
		_, e, code := runSource(t, &Ctx{Run: entry}, "init", "--slug", "demo", "--holder", "h1")
		link, _ := os.Readlink(store.Dir{Root: entry}.State())
		if got := listRun(t, entry); code != exit.Refused || e != "monet-source: run demo already has state\n" || got != tc.list || (tc.list == "state.json" && link != "gone") {
			t.Errorf("%s: code %d, stderr %q, run holds %q, link %q", tc.list, code, e, got, link)
		}
	}
}

// A run that is not at <home>/.planning/source/<slug> is exit 2 and writes
// nothing. The slug check comes first, and the home check comes before the
// skill checks (SOURCE_SKILL is empty) and the state check.
func TestInitRefusesHomeShape(t *testing.T) {
	t.Setenv("SOURCE_SKILL", "")
	run := filepath.Join(realTempDir(t), "demo")
	if err := os.Mkdir(run, 0o755); err != nil {
		t.Fatal(err)
	}
	want := "monet-source: run " + run + " is not at <home>/.planning/source/<slug>\n"
	_, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1")
	if got := listRun(t, run); code != exit.Refused || e != want || got != "" {
		t.Errorf("code %d, stderr %q, run holds %q", code, e, got)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "other", "--holder", "h1"); code != exit.Usage {
		t.Errorf("bad slug: code %d, stderr %q", code, e)
	}
	if err := os.Mkdir(store.Dir{Root: run}.State(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.Refused || e != want || listRun(t, run) != "state.json/" {
		t.Errorf("with state: code %d, stderr %q, run holds %q", code, e, listRun(t, run))
	}
}

// When the state check, the lease, a folder, the relay copy, the config of
// <home>, or the state.json write fails, init is exit 1 and leaves no
// state.json, and the holder can run init again.
func TestInitFailsWithoutState(t *testing.T) {
	blocked := newRun(t, "demo")
	if err := os.WriteFile(filepath.Join(blocked, "bin"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// The relay copy comes after the folders, so a failed folder leaves relay/ empty.
	if _, e, code := runSource(t, &Ctx{Run: blocked}, "init", "--slug", "demo", "--holder", "h1"); code != exit.User || !strings.Contains(e, "not a directory") || strings.Contains(listRun(t, blocked), "state.json") || listRun(t, filepath.Join(blocked, "relay")) != "" {
		t.Errorf("file at bin: code %d, stderr %q, run holds %q, relay holds %q", code, e, listRun(t, blocked), listRun(t, filepath.Join(blocked, "relay")))
	}
	if err := os.Remove(filepath.Join(blocked, "bin")); err != nil {
		t.Fatal(err)
	}
	_, _, code2 := runSource(t, &Ctx{Run: blocked}, "init", "--slug", "demo", "--holder", "h2")
	if _, e, code := runSource(t, &Ctx{Run: blocked}, "init", "--slug", "demo", "--holder", "h1"); code2 != exit.Held || code != exit.OK {
		t.Errorf("init again: h2 code %d, h1 code %d, stderr %q", code2, code, e)
	}
	// A folder at relay/brain.md or a dangling symlink at <home>/.claude fails
	// the relay copy or the config write, and no config names the copy.
	for _, block := range []func(run string) error{
		func(run string) error { return os.MkdirAll(filepath.Join(run, "relay", "brain.md", "x"), 0o755) },
		func(run string) error { return os.Symlink("gone", filepath.Join(run, "../../..", ".claude")) },
	} {
		run := newRun(t, "demo")
		if err := block(run); err != nil {
			t.Fatal(err)
		}
		_, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1")
		_, config, _ := relayFiles(run)
		if _, cerr := os.Lstat(config); code != exit.User || e == "" || strings.Contains(listRun(t, run), "state.json") || !os.IsNotExist(cerr) {
			t.Errorf("failed relay write: code %d, stderr %q, run holds %q, config lstat %v", code, e, listRun(t, run), cerr)
		}
	}
	// A lease that does not decode is exit 1, not exit 4 (held).
	corrupt := newRun(t, "demo")
	if err := os.Mkdir(store.Dir{Root: corrupt}.Locks(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir{Root: corrupt}.Locks(), "brain.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, e, code := runSource(t, &Ctx{Run: corrupt}, "init", "--slug", "demo", "--holder", "h1"); code != exit.User || listRun(t, corrupt) != "locks/" {
		t.Errorf("corrupt lease: code %d, stderr %q, run holds %q", code, e, listRun(t, corrupt))
	}
	// A state.json path past PATH_MAX (locks still fits) fails the Lstat: exit 1, no lease.
	long := filepath.Join(realTempDir(t), ".planning", "source", "demo")
	for _, err := os.Lstat(long + "/state.json"); os.IsNotExist(err); _, err = os.Lstat(long + "/state.json") {
		long = filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(long))), "pppp", ".planning", "source", "demo")
	}
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, e, code := runSource(t, &Ctx{Run: long}, "init", "--slug", "demo", "--holder", "h1"); code != exit.User || e != "monet-source: lstat "+filepath.Join(long, "state.json")+": file name too long\n" || listRun(t, long) != "" {
		t.Errorf("long path: code %d, stderr %q, run holds %q", code, e, listRun(t, long))
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes into a read-only folder")
	}
	// A run folder with no search bit fails the state.json Lstat: exit 1.
	closed := newRun(t, "demo")
	if err := os.Chmod(closed, 0o600); err != nil {
		t.Fatal(err)
	}
	_, e, code := runSource(t, &Ctx{Run: closed}, "init", "--slug", "demo", "--holder", "h1")
	if err := os.Chmod(closed, 0o755); err != nil {
		t.Fatal(err)
	}
	if code != exit.User || e != "monet-source: lstat "+filepath.Join(closed, "state.json")+": permission denied\n" || listRun(t, closed) != "" {
		t.Errorf("unsearchable run: code %d, stderr %q, run holds %q", code, e, listRun(t, closed))
	}
	ro := newRun(t, "demo")
	for _, name := range runFolders {
		if err := os.Mkdir(filepath.Join(ro, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ro, 0o755) })
	if _, e, code := runSource(t, &Ctx{Run: ro}, "init", "--slug", "demo", "--holder", "h1"); code != exit.User || !strings.Contains(e, "permission denied") || strings.Contains(listRun(t, ro), "state.json") {
		t.Errorf("read-only run: code %d, stderr %q, run holds %q", code, e, listRun(t, ro))
	}
}

// The slug matches ^[a-z0-9][a-z0-9-]*$ and equals the base name of the run
// folder, and init takes --holder and no positional argument. Each refusal is
// exit 64 and leaves the run folder empty.
func TestInitSlugMustMatchDir(t *testing.T) {
	bad := func(slug, dir string) string {
		return "init: slug " + strconv.Quote(slug) + " must match ^[a-z0-9][a-z0-9-]*$ and equal the run folder name " + strconv.Quote(dir)
	}
	odd := testNow.Add(97*time.Minute + 3*time.Second + 5) // an accepted init stores this exact clock
	for _, tc := range []struct {
		dir    string
		args   []string
		stderr string // "" means exit 0
	}{
		{"other", []string{"--slug", "demo", "--holder", "h1"}, bad("demo", "other")},
		{"demo-demo", []string{"--slug", "demo", "--holder", "h1"}, bad("demo", "demo-demo")},
		{"Demo", []string{"--slug", "demo", "--holder", "h1"}, bad("demo", "Demo")},
		{"Demo", []string{"--slug", "Demo", "--holder", "h1"}, bad("Demo", "Demo")},
		{"-demo", []string{"--slug=-demo", "--holder", "h1"}, bad("-demo", "-demo")},
		{"de_mo", []string{"--slug", "de_mo", "--holder", "h1"}, bad("de_mo", "de_mo")},
		{"demo", []string{"--holder", "h1"}, bad("", "demo")},
		{"demo", []string{"--slug", "demo"}, wantInitUsage},
		{"demo", []string{"--slug", "demo", "--holder", "h1", "x"}, wantInitUsage},
		{"demo", []string{"--slug", "demo", "--holder", "h1", "--bogus"}, "init: flag provided but not defined: -bogus"},
		{"9-x-", []string{"--slug", "9-x-", "--holder", "s-42"}, ""},
		{"a", []string{"--slug", "a", "--holder", "x"}, ""},
		{"run-2026", []string{"--slug", "run-2026", "--holder", "h1"}, ""},
	} {
		run := newRun(t, tc.dir)
		_, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return odd }}, append([]string{"init"}, tc.args...)...)
		wantCode, wantErr := exit.Usage, "monet-source: "+tc.stderr+"\n"
		if tc.stderr == "" {
			wantCode, wantErr = exit.OK, ""
		}
		if empty := listRun(t, run) == ""; code != wantCode || e != wantErr || empty != (wantCode != exit.OK) {
			t.Errorf("%s %q: code %d, stderr %q, empty run %v; want %d, %q", tc.dir, tc.args, code, e, empty, wantCode, wantErr)
		}
		if wantCode == exit.OK { // args are --slug <dir> --holder <holder>
			d := store.Dir{Root: run}
			s, err := state.Load(d)
			l, _, lerr := lock.Read(d.Locks(), "brain")
			if err != nil || s.Slug != tc.dir || !s.Created.Equal(odd) || lerr != nil || l.Holder != tc.args[3] || !l.Expires.Equal(odd.Add(12*time.Hour)) {
				t.Errorf("%s: state %+v (%v), lease %+v (%v)", tc.dir, s, err, l, lerr)
			}
		}
	}
}

// relayFiles returns <run>/relay/brain.md, the config path of <home>, and the
// config bytes that name only that copy (R5).
func relayFiles(run string) (brain, config, r5 string) {
	brain = filepath.Join(run, "relay", "brain.md")
	return brain, filepath.Join(run, "../../..", ".claude", "handoff-relay.json"), "{\n  \"procedure\": \"" + brain + "\"\n}\n"
}

// init copies reference/relay.md of the skill to <run>/relay/brain.md and
// writes the config of <home> that names that copy.
func TestInitWritesRelayConfig(t *testing.T) {
	run := newRun(t, "demo")
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK || e != "" {
		t.Fatalf("init: code %d, stderr %q", code, e)
	}
	brain, config, r5 := relayFiles(run)
	got, err := os.ReadFile(brain)
	cfg, cerr := os.ReadFile(config)
	if err != nil || string(got) != testkit.RelayText || cerr != nil || string(cfg) != r5 {
		t.Errorf("brain.md %q (%v), config %q (%v), want %q", got, err, cfg, cerr, r5)
	}
	// The copy holds the bytes of the skill file, whatever they are.
	run, steps := newRun(t, "demo"), "# Other steps\n\nNot the test text.\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("SOURCE_SKILL"), "reference", "relay.md"), []byte(steps), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK || e != "" {
		t.Fatalf("other steps: code %d, stderr %q", code, e)
	}
	if got, err := os.ReadFile(filepath.Join(run, "relay", "brain.md")); err != nil || string(got) != steps {
		t.Errorf("other steps: brain.md %q (%v), want %q", got, err, steps)
	}
}

// init keeps a config of <home> that names another procedure or does not
// parse, byte for byte, and rewrites one that names <run>/relay/brain.md to
// the file of R5. A folder at the config path cannot be read, so it stays.
func TestInitKeepsOtherRelayConfig(t *testing.T) {
	for _, tc := range []struct {
		body string
		keep bool
	}{
		{`{"procedure": ".planning/gang/x/run/relay.md"}`, true},
		{`{"procedure": "OTHER"}`, true}, // the copy of another run in the same home
		{"{", true},
		{`{"procedure": "BRAIN", "threshold_pct": 70}`, false},
	} {
		run := newRun(t, "demo")
		brain, config, r5 := relayFiles(run)
		other := filepath.Join(filepath.Dir(run), "other", "relay", "brain.md")
		body, want := strings.NewReplacer("BRAIN", brain, "OTHER", other).Replace(tc.body), r5
		if tc.keep {
			want = body
		}
		if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil || os.WriteFile(config, []byte(body), 0o644) != nil {
			t.Fatalf("write %s: %v", config, err)
		}
		_, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1")
		if got, err := os.ReadFile(config); code != exit.OK || e != "" || err != nil || string(got) != want {
			t.Errorf("%s: code %d, stderr %q, config %q (%v), want %q", tc.body, code, e, got, err, want)
		}
		checkCopied(t, run)
	}
	run := newRun(t, "demo")
	_, config, _ := relayFiles(run)
	if err := os.MkdirAll(config, 0o755); err != nil {
		t.Fatal(err)
	}
	_, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1")
	if fi, err := os.Stat(config); code != exit.OK || e != "" || err != nil || !fi.IsDir() {
		t.Errorf("folder config: code %d, stderr %q, stat %v", code, e, err)
	}
	checkCopied(t, run)
}

// checkCopied fails t unless <run>/relay/brain.md holds testkit.RelayText and
// state.json exists: init copies the steps and writes state.json also when it
// keeps the config of <home>.
func checkCopied(t *testing.T, run string) {
	t.Helper()
	brain, _, _ := relayFiles(run)
	got, err := os.ReadFile(brain)
	if _, serr := os.Stat(store.Dir{Root: run}.State()); err != nil || string(got) != testkit.RelayText || serr != nil {
		t.Errorf("%s: brain.md %q (%v), state.json %v", run, got, err, serr)
	}
}

// With no skill dir (exit 64), or a reference/relay.md that is missing or
// cannot be read (exit 2), init writes nothing, and it checks both before the
// state check.
func TestInitNeedsRelaySteps(t *testing.T) {
	old := skillDir
	t.Cleanup(func() { skillDir = old })
	skillDir = ""
	bare, folder := realTempDir(t), realTempDir(t)
	if err := os.MkdirAll(filepath.Join(folder, "reference", "relay.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		skill, stderr string
		code          int
	}{
		{bare, "monet-source: init: " + filepath.Join(bare, "reference", "relay.md") + ": no such file or directory\n", exit.Refused},
		{folder, "monet-source: init: " + filepath.Join(folder, "reference", "relay.md") + ": is a directory\n", exit.Refused},
		{"", "monet-source: no skill dir: pass --skill or set SOURCE_SKILL\n", exit.Usage},
	} {
		for _, list := range []string{"", "state.json/"} {
			run := newRun(t, "demo")
			t.Setenv("SOURCE_SKILL", tc.skill)
			if err := os.MkdirAll(filepath.Join(run, list), 0o755); err != nil {
				t.Fatal(err)
			}
			_, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1")
			if _, err := os.Lstat(filepath.Join(run, "../../..", ".claude")); code != tc.code || e != tc.stderr || listRun(t, run) != list || !os.IsNotExist(err) {
				t.Errorf("skill %q, run holds %q: code %d, stderr %q, <home>/.claude %v", tc.skill, list, code, e, err)
			}
		}
	}
}
