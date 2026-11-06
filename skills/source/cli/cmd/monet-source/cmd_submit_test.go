package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/artifact"
	"source/internal/events"
	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
)

// newLoopRun makes a run sealed with w1, with briefs for reviewer and reader, where h1
// has brain and d1 has dev.
func newLoopRun(t *testing.T) string {
	t.Helper()
	run := newSealableRun(t)
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("seal: code %d, stderr %q", code, e)
	}
	setBrief(t, run, "reviewer", "# reviewer\nCheck each claim against the code.\n")
	setBrief(t, run, "reader", "# reader\nRead the page as a newcomer.\n")
	if _, e, code := runSource(t, &Ctx{Run: run}, "dev", "hello", "--holder", "d1"); code != exit.OK {
		t.Fatalf("dev hello: code %d, stderr %q", code, e)
	}
	return run
}

// submitFile writes body to <run>/dev/<name>, submits it as d1, needs exit 0, and
// returns stdout without its newline.
func submitFile(t *testing.T, run, name, body string) string {
	t.Helper()
	p := filepath.Join(run, "dev", name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, e, code := runSource(t, &Ctx{Run: run}, "submit", "--path", p, "--holder", "d1")
	if code != exit.OK || !strings.HasSuffix(out, "\n") {
		t.Fatalf("submit %s: code %d, stdout %q, stderr %q", name, code, out, e)
	}
	return strings.TrimSuffix(out, "\n")
}

// Each submit freezes a new version of the phase, counts it in Versions, and records it.
// The first version of design is v1, after two of research.
func TestSubmitFreezesVersion(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	for k, body := range []string{"one\n", "two\n", "three\n"} {
		ph, v := []string{"research", "research", "design"}[k], []string{"v1", "v2", "v1"}[k]
		if err := state.Update(d, func(s *state.State) error { s.Phase = ph; return nil }); err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(run, "artifacts", ph, v)
		if got := submitFile(t, run, "a.md", body); got != dest {
			t.Fatalf("submit %d printed %q, want %q", k+1, got, dest)
		}
		var m artifact.Manifest
		copied, err := os.ReadFile(filepath.Join(dest, "a.md"))
		if err != nil || string(copied) != body || store.DecodeStrict(filepath.Join(dest, "manifest.json"), &m) != nil ||
			m.Phase != ph || m.Version != []int{1, 2, 1}[k] || !m.At.Equal(testNow) || len(m.Files) != 1 || m.Files[0].Path != filepath.Join(dest, "a.md") {
			t.Fatalf("%s: copy %q (err %v), manifest %+v", v, copied, err, m)
		}
	}
	s, err := state.Load(d)
	if err != nil || !reflect.DeepEqual(s.Versions, map[string]int{"research": 2, "design": 1}) {
		t.Fatalf("versions %v, load err %v; want research 2, design 1", s.Versions, err)
	}
	lines, _, err := events.For(d, "brain", 0)
	if want := []string{"VERSION research v1", "VERSION research v2", "VERSION design v1"}; err != nil || !reflect.DeepEqual(lines, want) {
		t.Fatalf("brain events %q, err %v; want %q", lines, err, want)
	}
	if l := readOr(t, d.Log()); !strings.HasSuffix(l, "\tVERSION research v1\n2026-11-01T12:00:00Z\tVERSION research v2\n2026-11-01T12:00:00Z\tVERSION design v1\n") {
		t.Fatalf("log.tsv %q; want the three VERSION rows last", l)
	}
}

// underNotes agrees with a plain prefix check over every folder that a 1 to 3 segment path
// names. A symlink to a note or to notes, a symlink and then .., a relative path, or (on a
// case-insensitive disk) NOTES into notes is refused with no version written.
func TestSubmitRefusesNotes(t *testing.T) {
	run := newLoopRun(t)
	names, notes := []string{"notes", "notes2", "note", "x"}, filepath.Join(run, "notes")
	for i := 0; i < 64; i++ { // each path of three names, so every folder to a depth of 3
		if err := os.MkdirAll(filepath.Join(run, names[i/16], names[i/4%4], names[i%4]), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	segs, paths, seen := []string{"notes", "notes2", "note", "x", "..", "."}, []string{run}, map[bool]int{}
	for depth := 1; depth <= 3; depth++ {
		var next []string
		for _, p := range paths {
			for _, s := range segs {
				abs := p + "/" + s
				next = append(next, abs)
				if fi, err := os.Stat(abs); err == nil && fi.IsDir() {
					c := filepath.Clean(abs)
					want := c == notes || strings.HasPrefix(c, notes+"/")
					if got, err := underNotes(run, abs); got != want || err != nil {
						t.Errorf("underNotes(%q) = %v, %v; want %v", abs, got, err, want)
					}
					seen[want]++
				}
			}
		}
		paths = next
	}
	if seen[true] == 0 || seen[false] == 0 {
		t.Fatalf("paths under notes %d, outside %d; want both", seen[true], seen[false])
	}
	link, nl, note, deep, s := filepath.Join(run, "dev", "link"), filepath.Join(run, "dev", "nl"), filepath.Join(notes, "n.md"), filepath.Join(notes, "x", "m.md"), filepath.Join(run, "dev", "s")
	if os.WriteFile(note, []byte("a note\n"), 0o644) != nil || os.WriteFile(deep, []byte("a note\n"), 0o644) != nil || os.Symlink(note, link) != nil || os.Symlink(notes, nl) != nil || os.Symlink(filepath.Join(notes, "x"), s) != nil || os.WriteFile(filepath.Join(run, "dev", "n.md"), nil, 0o644) != nil {
		t.Fatal("write the notes or the symlinks")
	}
	// link links to a note, nl to notes, and s to notes/x, so s/.. is notes, not dev with its decoy n.md. The working folder is s.
	refused := []string{link, filepath.Join(nl, "n.md"), "m.md", s + "/../n.md", "../n.md"}
	if _, err := os.Stat(filepath.Join(run, "NOTES")); err == nil { // a case-insensitive disk
		refused = append(refused, filepath.Join(run, "NOTES", "n.md"))
	}
	t.Chdir(s)
	for _, p := range refused {
		_, e, code := runSource(t, &Ctx{Run: run}, "submit", "--path", p, "--holder", "d1")
		if _, err := os.Stat(filepath.Join(run, "artifacts", "research", "v1")); code != exit.Refused || e != "monet-source: submit: "+p+" is in "+notes+"\n" || !os.IsNotExist(err) {
			t.Fatalf("%s: code %d, stderr %q, v1 stat err %v", p, code, e, err)
		}
	}
}

// submit checks usage, then renews dev, then refuses in the order charter lint, seal and phase,
// notes, and Freeze, under state.mutex, then records after it. No refusal writes state, an artifact,
// or an event. Usage and another holder renew nothing. The OK row freezes two files.
func TestSubmitNeedsPhaseAndLease(t *testing.T) {
	run := newSealableRun(t)
	d := store.Dir{Root: run}
	if _, e, code := runSource(t, &Ctx{Run: run}, "dev", "hello", "--holder", "d1"); code != exit.OK {
		t.Fatalf("dev hello: code %d, stderr %q", code, e)
	}
	f, g, note, folder := filepath.Join(run, "dev", "a.md"), filepath.Join(run, "dev", "g.md"), filepath.Join(run, "notes", "n.md"), filepath.Join(run, "dev")
	if os.WriteFile(f, []byte("text\n"), 0o644) != nil || os.WriteFile(g, []byte("more\n"), 0o644) != nil || os.WriteFile(note, []byte("text\n"), 0o644) != nil {
		t.Fatal("write the files")
	}
	seal := func() {
		if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
			t.Fatalf("seal: code %d, stderr %q", code, e)
		}
	}
	phase := func(p string) func() {
		return func() {
			if err := state.Update(d, func(s *state.State) error { s.Phase = p; return nil }); err != nil {
				t.Fatal(err)
			}
		}
	}
	usage, lease, v1 := "usage: monet-source submit --path <file>... --holder <session>", testNow.Add(leaseTTL), filepath.Join(run, "artifacts", "design", "v1")
	phases, mutex, locked, last := `["research", "design"],`, d.Path("locks", "state.mutex"), false, true
	for i, tc := range []struct {
		setup func() // runs before the row
		args  []string
		code  int
		out   string // stdout
		e     string // stderr after "monet-source: "
	}{
		{nil, []string{"--holder", "d1"}, exit.Usage, "", usage},
		{nil, []string{"x", "--path", f, "--holder", "d1"}, exit.Usage, "", usage},
		{nil, []string{"--path", f}, exit.Usage, "", "--holder <session> is required: the dev lease holder"},
		{nil, []string{"--path", note, "--holder", "d2"}, exit.Held, "", "lock held: dev is not held by d2"},
		{func() { os.Remove(d.Charter()) }, []string{"--path", note, "--holder", "d1"}, exit.Refused, "", "open " + d.Charter() + ": no such file or directory"},
		{func() { writeCharter(t, run, phases, `["research", "design", ""],`) }, []string{"--path", f, "--holder", "d1"}, exit.Refused, "charter: phases: : want research or design\n", ""},
		{func() { writeCharter(t, run) }, []string{"--path", note, "--holder", "d1"}, exit.Refused, "", "submit: phase  takes no version"},
		{phase("research"), []string{"--path", f, "--holder", "d1"}, exit.Refused, "", "submit: phase research takes no version"},
		{seal, []string{"--path", folder, "--path", note, "--holder", "d1"}, exit.Refused, "", "submit: " + note + " is in " + filepath.Join(run, "notes")},
		{nil, []string{"--path", f, "--path", folder, "--holder", "d1"}, exit.Refused, "", "submit: read " + folder + ": is a directory"},
		{nil, []string{"--path", f, "--path", filepath.Join(folder, "b.md"), "--holder", "d1"}, exit.Refused, "", "submit: lstat " + filepath.Join(folder, "b.md") + ": no such file or directory"},
		{phase("acceptance"), []string{"--path", f, "--holder", "d1"}, exit.Refused, "", "submit: phase acceptance takes no version"},
		{func() { writeCharter(t, run, phases, `["research", "design", "acceptance"],`) }, []string{"--path", f, "--holder", "d1"}, exit.Refused, "charter: phases: acceptance: want research or design\n", ""},
		{func() { writeCharter(t, run); phase("design")() }, []string{"--path", f, "--path", g, "--holder", "d1"}, exit.OK, v1 + "\n", ""},
	} {
		if tc.setup != nil {
			tc.setup()
		}
		before, evs, now := readOr(t, d.State()), readOr(t, d.Events()), testNow.Add(time.Duration(i+1)*time.Hour)
		// Freeze takes c.Now() and record reads Now last: a call under state.mutex puts Freeze in state.Update, and a last call with none puts record after it.
		at := func() time.Time { _, err := os.Stat(mutex); last = err == nil; locked = locked || last; return now }
		out, e, code := runSource(t, &Ctx{Run: run, Now: at}, append([]string{"submit"}, tc.args...)...)
		wantOut, wantErr := tc.out, ""
		if tc.e != "" {
			wantErr = "monet-source: " + tc.e + "\n"
		}
		if tc.code != exit.Usage && tc.code != exit.Held {
			lease = now.Add(leaseTTL)
		}
		ents, _ := os.ReadDir(filepath.Join(run, "artifacts"))
		wrote := readOr(t, d.State()) != before || readOr(t, d.Events()) != evs || len(ents) != 0
		if code != tc.code || out != wantOut || e != wantErr || (tc.code != exit.OK && wrote) {
			t.Errorf("row %d %q: code %d, stdout %q, stderr %q, wrote %v", i, tc.args, code, out, e, wrote)
		}
		checkLease(t, run, "dev", lease)
	}
	s, err := state.Load(d)
	lines, _, _ := events.For(d, "brain", 0)
	if err != nil || !reflect.DeepEqual(s.Versions, map[string]int{"design": 1}) || !reflect.DeepEqual(lines, []string{"VERSION design v1"}) || !locked || last ||
		readOr(t, filepath.Join(v1, "a.md")) != "text\n" || readOr(t, filepath.Join(v1, "g.md")) != "more\n" {
		t.Fatalf("versions %v, brain events %q, load err %v, Now under state.mutex %v, last Now under it %v, v1 a.md or g.md wrong; want design 1, true, false", s.Versions, lines, err, locked, last)
	}
}
