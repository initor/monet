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
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter"); code != exit.Usage || !strings.Contains(e, "charter: want a subcommand: lint") {
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
