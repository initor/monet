package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"source/internal/charter"
	"source/internal/exit"
	"source/internal/lock"
	"source/internal/store"
	"source/internal/words"
)

// writeCharter writes the charter fixture, after the replacements in edits,
// to <run>/charter.json, and returns its path.
func writeCharter(t *testing.T, run string, edits ...string) string {
	t.Helper()
	data, err := os.ReadFile("../../internal/charter/testdata/charter.json")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for i := 0; i+1 < len(edits); i += 2 {
		if !strings.Contains(s, edits[i]) {
			t.Fatalf("fixture holds no %q", edits[i])
		}
		s = strings.ReplaceAll(s, edits[i], edits[i+1])
	}
	p := filepath.Join(run, "charter.json")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// charter lint prints each problem on stdout and exits 2, or prints clean.
// It reads <run>/charter.json by default and writes nothing to the run.
func TestCharterLintCommand(t *testing.T) {
	run, other := realTempDir(t), realTempDir(t)
	writeCharter(t, run)
	if out, e, code := runSource(t, &Ctx{Run: run}, "charter", "lint"); code != exit.OK || out != "charter: clean\n" || e != "" {
		t.Fatalf("default path: code %d, stdout %q, stderr %q", code, out, e)
	}
	p := writeCharter(t, other, `"weight": "blocking"`, `"weight": "advisory"`)
	out, e, code := runSource(t, &Ctx{Run: run}, "charter", "lint", p)
	if want := "charter: personas: no blocking persona, so no persona can accept (call 4)\n"; code != exit.Refused || out != want || e != "" {
		t.Fatalf("two advisory: code %d, stdout %q, stderr %q", code, out, e)
	}
	p = writeCharter(t, other, `"version": 1`, `"version": 2`, `"words": "u1"`, `"words": ""`)
	if out, _, code := runSource(t, &Ctx{Run: run}, "charter", "lint", p); code != exit.Refused || out != "charter: version: want 1\ncharter: grants: 1: words: empty\n" {
		t.Fatalf("two problems: code %d, stdout %q", code, out)
	}
	p = writeCharter(t, other, `"version": 1,`, `"version": 1, "repo": "x",`)
	if out, e, code := runSource(t, &Ctx{Run: run}, "charter", "lint", p); code != exit.Refused || out != "" || e != "monet-source: charter: repo comes with step 2 (call 24)\n" {
		t.Fatalf("repo: code %d, stdout %q, stderr %q", code, out, e)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "lint", "a", "b"); code != exit.Usage || e != "monet-source: charter lint: want at most one path\n" {
		t.Fatalf("two paths: code %d, stderr %q", code, e)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter"); code != exit.Usage || !strings.Contains(e, "charter: want a subcommand: amend, lint, render, seal\n") {
		t.Fatalf("no subcommand: code %d, stderr %q", code, e)
	}
	if ents, err := os.ReadDir(run); err != nil || len(ents) != 1 {
		t.Fatalf("run dir holds %v, %v; want only charter.json", ents, err)
	}
}

// The rows run in order on one run, and nil edits keep the last charter.json.
// No refusal writes a page. A clean render renames charter.md over a read-only one.
func TestCharterRenderCommand(t *testing.T) {
	d := store.Dir{Root: realTempDir(t)}
	if _, err := lock.Acquire(d.Locks(), "brain", "h1", time.Minute, testNow); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		code        int
		edits, args []string
		out, e      string
	}{
		{exit.Refused, nil, []string{"--holder", "h1"}, "", "monet-source: open " + d.Charter() + ": no such file or directory\n"},
		{exit.Refused, []string{`"weight": "blocking"`, `"weight": "advisory"`}, []string{"--holder", "h1"}, "charter: personas: no blocking persona, so no persona can accept (call 4)\n", ""},
		{exit.Refused, []string{`"version": 1,`, `"version": 1, "repo": "x",`}, []string{"--holder", "h1"}, "", "monet-source: charter: repo comes with step 2 (call 24)\n"},
		{exit.Held, []string{}, []string{"--holder", "h2"}, "", "monet-source: lock held: brain is not held by h2\n"},
		{exit.Usage, nil, nil, "", "monet-source: --holder <session> is required: the brain lease holder\n"},
		{exit.Usage, nil, []string{"x", "--holder", "h1"}, "", "monet-source: charter render: want no arguments\n"},
	} {
		if tc.edits != nil {
			writeCharter(t, d.Root, tc.edits...)
		}
		out, e, code := runSource(t, &Ctx{Run: d.Root}, append([]string{"charter", "render"}, tc.args...)...)
		if _, err := os.Stat(d.CharterMD()); code != tc.code || out != tc.out || e != tc.e || !os.IsNotExist(err) {
			t.Fatalf("%q: code %d, stdout %q, stderr %q, charter.md stat err %v", tc.args, code, out, e, err)
		}
	}
	checkLease(t, d.Root, "brain", testNow.Add(12*time.Hour))
	data, _ := os.ReadFile(d.Charter())
	if err := os.WriteFile(d.CharterMD(), []byte("old\n"), 0o444); err != nil || os.Chmod(d.Root, 0o555) != nil {
		t.Fatalf("old page: write err %v, or chmod of the run failed", err)
	}
	t.Cleanup(func() { os.Chmod(d.Root, 0o755) })
	if out, _, code := runSource(t, &Ctx{Run: d.Root}, "charter", "render", "--holder", "h1"); code != exit.User || out != "" || os.Chmod(d.Root, 0o755) != nil {
		t.Fatalf("read-only run: code %d, stdout %q, want 1 and no path", code, out)
	}
	out, e, code := runSource(t, &Ctx{Run: d.Root}, "charter", "render", "--holder", "h1")
	got, err := os.ReadFile(d.CharterMD())
	want, _ := os.ReadFile("../../internal/charter/testdata/charter.md")
	if code != exit.OK || out != d.CharterMD()+"\n" || e != "" || err != nil || string(got) != string(want) || !strings.HasSuffix(string(got), "<!-- charter sha256="+charter.Hash(data)+" -->\n") {
		t.Fatalf("clean: code %d, stdout %q, stderr %q, charter.md err %v, got\n%s\nwant the golden page with the hash of charter.json", code, out, e, err, got)
	}
	if ents, err := os.ReadDir(d.Root); err != nil || len(ents) != 3 {
		t.Fatalf("run dir holds %v, %v; want charter.json, charter.md, locks", ents, err)
	}
	if doc, err := os.ReadFile("../../../../../docs/design/source.md"); err != nil || !strings.Contains(string(doc), "| `charter lint`, `render`, ") {
		t.Fatalf("design 5.9: the charter row names no render (err %v)", err)
	}
}

// newSealableRun makes a run with init, the fixture charter, a rendered
// charter.md, and the owner words w1. The holder h1 has brain.
func newSealableRun(t *testing.T) string {
	t.Helper()
	run := newRun(t, "demo")
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK {
		t.Fatalf("init: code %d, stderr %q", code, e)
	}
	writeCharter(t, run)
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "render", "--holder", "h1"); code != exit.OK {
		t.Fatalf("render: code %d, stderr %q", code, e)
	}
	backdateRender(t, run)
	if err := words.Append(store.Dir{Root: run}.Words(), words.Entry{ID: "w1", At: "2026-11-01T12:00:00Z", Text: "Yes, seal it."}); err != nil {
		t.Fatal(err)
	}
	return run
}

// backdateRender sets the time of charter.md to an hour before testNow, so that the fixture owner
// words, which are dated testNow or later, come after the render.
func backdateRender(t *testing.T, run string) {
	t.Helper()
	at := testNow.Add(-time.Hour)
	if err := os.Chtimes(store.Dir{Root: run}.CharterMD(), at, at); err != nil {
		t.Fatal(err)
	}
}

// sealFiles returns state.json and log.tsv, the files that a seal writes.
func sealFiles(t *testing.T, run string) string {
	t.Helper()
	d := store.Dir{Root: run}
	s, err := os.ReadFile(d.State())
	if err != nil {
		t.Fatal(err)
	}
	l, _ := os.ReadFile(d.Log()) // a run with no seal has no log.tsv
	return string(s) + "-- log.tsv\n" + string(l)
}

// A charter.json that still lints clean but changed after the render gives exit 3 and writes nothing.
func TestSealRefusesChangedCharter(t *testing.T) {
	run := newSealableRun(t) // init, the fixture charter, a rendered charter.md, and the entry w1
	path := filepath.Join(run, "charter.json")
	c, err := charter.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c.Goal += " (changed)" // the charter still lints clean, so only the hash check can refuse
	if err := store.WriteJSON(path, c); err != nil {
		t.Fatal(err)
	}
	before := sealFiles(t, run)
	_, errOut, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1")
	if code != exit.Conflict || !strings.Contains(errOut, "changed after charter.md was rendered") || sealFiles(t, run) != before {
		t.Fatalf("code %d, stderr %q, or the seal wrote", code, errOut)
	}
}

// Owner words typed before charter.md was rendered cannot confirm it: seal gives exit 2, names both
// times, and writes nothing. Words typed after the render seal. This is the kickoff where an early
// message ("i let u know") sealed the charter, since a yes in a question dialog is not in owner-words.md.
func TestSealNeedsWordsAfterRender(t *testing.T) {
	run := newSealableRun(t) // charter.md is dated testNow-1h, and w1 is dated testNow
	d := store.Dir{Root: run}
	if err := words.Append(d.Words(), words.Entry{ID: "w0", At: "2026-11-01T10:00:00Z", Text: "i let u know"}); err != nil {
		t.Fatal(err)
	}
	before := sealFiles(t, run)
	want := "monet-source: owner words w0 at 2026-11-01T10:00:00Z came before charter.md was rendered at 2026-11-01T11:00:00Z: ask the owner to type the yes in the chat, run relay words, and pass the new id\n"
	for _, sub := range []string{"seal", "amend"} {
		if _, e, code := runSource(t, &Ctx{Run: run}, "charter", sub, "--words", "w0", "--holder", "h1"); code != exit.Refused || e != want || sealFiles(t, run) != before {
			t.Fatalf("%s w0: code %d, stderr %q, or it wrote", sub, code, e)
		}
	}
	at := testNow.Add(-time.Hour) // an entry at the same instant as the render is not after it
	if err := words.Append(d.Words(), words.Entry{ID: "w-same", At: at.Format(time.RFC3339Nano), Text: "yes"}); err != nil {
		t.Fatal(err)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w-same", "--holder", "h1"); code != exit.Refused || !strings.Contains(e, "came before charter.md was rendered") || sealFiles(t, run) != before {
		t.Fatalf("w-same: code %d, stderr %q, or it wrote", code, e)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("w1 after the render: code %d, stderr %q", code, e)
	}
}

// An id with no entry in owner-words.md gives exit 2 and writes nothing.
func TestSealNeedsWords(t *testing.T) {
	run := newSealableRun(t)
	before := sealFiles(t, run)
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w9", "--holder", "h1"); code != exit.Refused || e != "monet-source: owner words: no entry w9\n" || sealFiles(t, run) != before {
		t.Fatalf("code %d, stderr %q, or the seal wrote", code, e)
	}
}

// A second seal, with the same or other words, gives exit 2 and keeps the first seal and its one log row.
func TestSealOnce(t *testing.T) {
	run := newSealableRun(t)
	if err := words.Append(store.Dir{Root: run}.Words(), words.Entry{ID: "w2", At: "2026-11-01T12:30:00Z", Text: "Seal it again."}); err != nil {
		t.Fatal(err)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("first: code %d, stderr %q", code, e)
	}
	before := sealFiles(t, run)
	for _, id := range []string{"w1", "w2"} {
		if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", id, "--holder", "h1"); code != exit.Refused || e != "monet-source: charter already sealed with words w1\n" || sealFiles(t, run) != before {
			t.Fatalf("%s: code %d, stderr %q, or the seal wrote", id, code, e)
		}
	}
}

// A seal stores the words, the clock, and the hash of charter.json, sets the first phase, logs one
// row with a UTC clock, renews brain, and prints nothing. Then status prints the seal.
// It seals with w2, not the first entry w1, so every place that names the words uses the given id.
func TestSealSetsFirstPhase(t *testing.T) {
	run := newSealableRun(t)
	data, err := os.ReadFile(store.Dir{Root: run}.Charter())
	if err != nil || words.Append(store.Dir{Root: run}.Words(), words.Entry{ID: "w2", At: "2026-11-01T12:30:00Z", Text: "Seal it."}) != nil {
		t.Fatalf("read charter.json: %v, or the append of w2 failed", err)
	}
	now := testNow.Add(time.Hour + 5).In(time.FixedZone("PST", -8*3600))
	if out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return now }}, "charter", "seal", "--words", "w2", "--holder", "h1"); code != exit.OK || out != "" || e != "" {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out, e)
	}
	want := "{\n  \"version\": 1,\n  \"slug\": \"demo\",\n  \"created\": \"2026-11-01T12:00:00Z\",\n  \"phase\": \"research\",\n" +
		"  \"sealed\": {\n    \"words\": \"w2\",\n    \"at\": \"2026-11-01T05:00:00.000000005-08:00\",\n    \"hash\": \"" + charter.Hash(data) + "\"\n  }\n}\n" +
		"-- log.tsv\nat\tline\n2026-11-01T13:00:00.000000005Z\tcharter sealed with words w2\n"
	if got := sealFiles(t, run); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	checkLease(t, run, "brain", now.Add(12*time.Hour))
	if out, e, code := runSource(t, &Ctx{Run: run}, "status"); code != exit.OK || !strings.Contains(out, "\nphase research\nround none\nwaiting none\ncharter sealed with words w2\nlease brain h1 ") {
		t.Fatalf("status: code %d, stdout %q, stderr %q", code, out, e)
	}
}

// Seal and amend check usage, then renew brain, then refuse in the order lint, words, hash, and state:
// a row that breaks two checks gets the refusal of the earlier one. No refusal writes state or log.
func TestSealChecksInOrder(t *testing.T) {
	advisory, changed := []string{`"weight": "blocking"`, `"weight": "advisory"`}, []string{`"One explainer page."`, `"Two explainer pages."`}
	usage := "monet-source: usage: monet-source charter <sub> --words <id> --holder <session>\n"
	rows := []struct {
		name   string
		setup  func(run string)
		args   []string
		code   int
		out, e string // <run> and <sub> in e stand for the run dir and the subcommand
	}{
		{"no words", nil, []string{"--holder", "h1"}, exit.Usage, "", usage},
		{"argument", nil, []string{"x", "--words", "w1", "--holder", "h1"}, exit.Usage, "", usage},
		{"bad flag", nil, []string{"--bogus"}, exit.Usage, "", "monet-source: charter <sub>: flag provided but not defined: -bogus\n"},
		{"no holder", nil, []string{"--words", "w1"}, exit.Usage, "", "monet-source: --holder <session> is required: the brain lease holder\n"},
		{"lease before lint", func(run string) { writeCharter(t, run, advisory...) }, []string{"--words", "w1", "--holder", "h2"}, exit.Held, "", "monet-source: lock held: brain is not held by h2\n"},
		{"no charter.json", func(run string) { os.Remove(filepath.Join(run, "charter.json")) }, []string{"--words", "w9", "--holder", "h1"}, exit.Refused, "", "monet-source: open <run>/charter.json: no such file or directory\n"},
		{"repo", func(run string) { writeCharter(t, run, `"version": 1,`, `"version": 1, "repo": "x",`) }, []string{"--words", "w9", "--holder", "h1"}, exit.Refused, "", "monet-source: charter: repo comes with step 2 (call 24)\n"},
		{"lint before words", func(run string) { writeCharter(t, run, advisory...) }, []string{"--words", "w9", "--holder", "h1"}, exit.Refused, "charter: personas: no blocking persona, so no persona can accept (call 4)\n", ""},
		{"words before hash", func(run string) { writeCharter(t, run, changed...) }, []string{"--words", "w9", "--holder", "h1"}, exit.Refused, "", "monet-source: owner words: no entry w9\n"},
		{"no charter.md", func(run string) { os.Remove(filepath.Join(run, "charter.md")) }, []string{"--words", "w1", "--holder", "h1"}, exit.Conflict, "", "monet-source: open <run>/charter.md: no such file or directory\n"},
		{"text after the hash line", func(run string) { // charter.md must END with the hash line
			f, err := os.OpenFile(filepath.Join(run, "charter.md"), os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.WriteString("An edit after the render.\n"); err != nil || f.Close() != nil {
				t.Fatalf("append to charter.md: %v", err)
			}
		}, []string{"--words", "w1", "--holder", "h1"}, exit.Conflict, "", "monet-source: charter.json changed after charter.md was rendered: render it and get the yes of the owner again\n"},
		{"hash before state", func(run string) {
			if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
				t.Fatalf("first seal: code %d, stderr %q", code, e)
			}
			writeCharter(t, run, changed...)
		}, []string{"--words", "w1", "--holder", "h1"}, exit.Conflict, "", "monet-source: charter.json changed after charter.md was rendered: render it and get the yes of the owner again\n"},
	}
	for i, tc := range append(rows, rows...) { // each row runs for seal, then for amend
		sub := []string{"seal", "amend"}[i/len(rows)]
		run := newSealableRun(t)
		if tc.setup != nil {
			tc.setup(run)
		}
		before, now := sealFiles(t, run), testNow.Add(time.Hour)
		out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return now }}, append([]string{"charter", sub}, tc.args...)...)
		if code != tc.code || out != tc.out || e != strings.NewReplacer("<run>", run, "<sub>", sub).Replace(tc.e) || sealFiles(t, run) != before {
			t.Errorf("%s %s: code %d, stdout %q, stderr %q, or it wrote", sub, tc.name, code, out, e)
		}
		lease := now.Add(12 * time.Hour) // a usage error or another holder renews nothing
		if tc.code == exit.Usage || tc.code == exit.Held {
			lease = testNow.Add(12 * time.Hour)
		}
		checkLease(t, run, "brain", lease)
	}
}

// Amend needs a seal, else exit 2 and no write. After the seal, each amend appends its words, clock, and
// the hash of the charter.json that charter.md shows, logs one row, keeps the seal, and prints nothing.
func TestAmend(t *testing.T) {
	run := newSealableRun(t)
	d, before := store.Dir{Root: run}, sealFiles(t, run)
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "amend", "--words", "w1", "--holder", "h1"); code != exit.Refused || e != "monet-source: charter not sealed: seal it before an amend\n" || sealFiles(t, run) != before {
		t.Fatalf("before the seal: code %d, stderr %q, or the amend wrote", code, e)
	}
	sealed, _ := os.ReadFile(d.Charter())
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("seal: code %d, stderr %q", code, e)
	}
	data, err := os.ReadFile(writeCharter(t, run, `"One explainer page."`, `"Two explainer pages."`))
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "render", "--holder", "h1"); err != nil || code != exit.OK || words.Append(d.Words(), words.Entry{ID: "w2", At: "2026-11-01T12:30:00Z", Text: "Amend it."}) != nil {
		t.Fatalf("render: code %d, stderr %q, read err %v, or the append of w2 failed", code, e, err)
	}
	backdateRender(t, run)
	amended, now := charter.Hash(data), testNow.Add(time.Hour)
	for _, id := range []string{"w2", "w1"} {
		if out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return now }}, "charter", "amend", "--words", id, "--holder", "h1"); code != exit.OK || out != "" || e != "" {
			t.Fatalf("amend %s: code %d, stdout %q, stderr %q", id, code, out, e)
		}
	}
	entry := func(id string) string {
		return "    {\n      \"words\": \"" + id + "\",\n      \"at\": \"2026-11-01T13:00:00Z\",\n      \"hash\": \"" + amended + "\"\n    }"
	}
	want := "{\n  \"version\": 1,\n  \"slug\": \"demo\",\n  \"created\": \"2026-11-01T12:00:00Z\",\n  \"phase\": \"research\",\n" +
		"  \"sealed\": {\n    \"words\": \"w1\",\n    \"at\": \"2026-11-01T12:00:00Z\",\n    \"hash\": \"" + charter.Hash(sealed) + "\"\n  },\n" +
		"  \"amends\": [\n" + entry("w2") + ",\n" + entry("w1") + "\n  ]\n}\n-- log.tsv\nat\tline\n2026-11-01T12:00:00Z\tcharter sealed with words w1\n" +
		"2026-11-01T13:00:00Z\tcharter amended with words w2\n2026-11-01T13:00:00Z\tcharter amended with words w1\n"
	if got := sealFiles(t, run); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	checkLease(t, run, "brain", now.Add(12*time.Hour))
}
