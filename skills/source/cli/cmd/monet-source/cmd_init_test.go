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
)

// wantInitUsage is the init usage line, typed out so that a change to
// initUsage fails a test.
const wantInitUsage = "usage: monet-source init --slug <slug> --holder <session>"

// newRun makes the folder <temp>/<name> and returns its real path.
func newRun(t *testing.T, name string) string {
	t.Helper()
	run := filepath.Join(realTempDir(t), name)
	if err := os.Mkdir(run, 0o755); err != nil {
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

// When the state check, the lease, a folder, or the state.json write fails,
// init is exit 1 and leaves no state.json, and the holder can run init again.
func TestInitFailsWithoutState(t *testing.T) {
	blocked := newRun(t, "demo")
	if err := os.WriteFile(filepath.Join(blocked, "bin"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, e, code := runSource(t, &Ctx{Run: blocked}, "init", "--slug", "demo", "--holder", "h1"); code != exit.User || !strings.Contains(e, "not a directory") || strings.Contains(listRun(t, blocked), "state.json") {
		t.Errorf("file at bin: code %d, stderr %q, run holds %q", code, e, listRun(t, blocked))
	}
	if err := os.Remove(filepath.Join(blocked, "bin")); err != nil {
		t.Fatal(err)
	}
	_, _, code2 := runSource(t, &Ctx{Run: blocked}, "init", "--slug", "demo", "--holder", "h2")
	if _, e, code := runSource(t, &Ctx{Run: blocked}, "init", "--slug", "demo", "--holder", "h1"); code2 != exit.Held || code != exit.OK {
		t.Errorf("init again: h2 code %d, h1 code %d, stderr %q", code2, code, e)
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
	long := filepath.Join(realTempDir(t), "demo")
	for _, err := os.Lstat(long + "/state.json"); os.IsNotExist(err); _, err = os.Lstat(long + "/state.json") {
		long = filepath.Join(filepath.Dir(long), "pppp", "demo")
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
