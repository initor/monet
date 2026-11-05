package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"source/internal/exit"
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
