package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each named path is a fixed file or folder name directly under Root.
func TestDirPaths(t *testing.T) {
	d := Dir{Root: "/r"}
	for _, tc := range []struct{ name, got, want string }{
		{"State", d.State(), "/r/state.json"},
		{"Events", d.Events(), "/r/events.tsv"},
		{"Log", d.Log(), "/r/log.tsv"},
		{"Words", d.Words(), "/r/owner-words.md"},
		{"Charter", d.Charter(), "/r/charter.json"},
		{"CharterMD", d.CharterMD(), "/r/charter.md"},
		{"Locks", d.Locks(), "/r/locks"},
		{"Personas", d.Personas(), "/r/personas"},
		{"Watch", d.Watch(), "/r/watch"},
		{"Path", d.Path("a", "b.json"), "/r/a/b.json"},
		{"Path root", d.Path(), "/r"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

// A second write replaces the first, leaves no temp file, and sets perm.
func TestWriteFileAtomicReplaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub")
	p := filepath.Join(dir, "x.json")
	if err := WriteFileAtomic(p, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("b"), 0o640); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil || string(got) != "b" {
		t.Fatalf("file = %q (err %v), want %q", got, err, "b")
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640", fi.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temp file %s left in %s", e.Name(), dir)
		}
	}
	if len(entries) != 1 {
		t.Errorf("%s holds %d entries, want 1", dir, len(entries))
	}
}

// The temp file sits in the target folder, so TMPDIR plays no part.
func TestWriteFileAtomicTempInTargetDir(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.json")
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if err := WriteFileAtomic(p, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A write whose rename fails leaves no temp file behind.
func TestWriteFileAtomicCleansUpOnFailure(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x")
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "keep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("a"), 0o644); err == nil {
		t.Fatal("rename over a non-empty folder succeeded, want an error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("%s holds %d entries, want only the folder x", dir, len(entries))
	}
}

// WriteJSON writes two-space indent, a trailing newline, and mode 0644.
func TestWriteJSONFormat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	if err := WriteJSON(p, map[string]any{"a": []int{1}}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if want := "{\n  \"a\": [\n    1\n  ]\n}\n"; err != nil || string(got) != want {
		t.Errorf("file = %q (err %v), want %q", got, err, want)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", fi.Mode().Perm())
	}
	if err := WriteJSON(p, func() {}); err == nil {
		t.Error("WriteJSON of a func succeeded, want an encode error")
	}
}
