package store

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// The paths come from Contract 4 of the overview plan.
func TestDirLayout(t *testing.T) {
	d := Dir{Root: filepath.Join("r", ".planning", "gang", "demo")}
	if got, want := d.Plan(), filepath.Join(d.Root, "plan", "plan.json"); got != want {
		t.Errorf("Plan() = %s, want %s", got, want)
	}
	if got, want := d.PlanMD(), filepath.Join(d.Root, "plan", "plan.md"); got != want {
		t.Errorf("PlanMD() = %s, want %s", got, want)
	}
	if got, want := d.Run("tasks", "A1", "report.md"), filepath.Join(d.Root, "run", "tasks", "A1", "report.md"); got != want {
		t.Errorf("Run() = %s, want %s", got, want)
	}
	if got, want := d.Run(), filepath.Join(d.Root, "run"); got != want {
		t.Errorf("Run() = %s, want %s", got, want)
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "state.json")
	if err := WriteFileAtomic(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two\n" {
		t.Errorf("content %q, want %q", data, "two\n")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("perm %v, want 0644", fi.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("dir holds %d entries, want only state.json (no temp files left)", len(entries))
	}
}

func TestAppendLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "events.log")
	for _, l := range []string{"first", "second"} {
		if err := AppendLine(path, l); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first\nsecond\n" {
		t.Errorf("content %q", data)
	}
	if err := AppendLine(path, "bad\nline"); err == nil {
		t.Error("a line holding a newline was accepted")
	}
}

// The rules come from the Global Constraints: tabs and newlines become spaces,
// and a leading =, +, -, or @ gets a ' prefix.
func TestSanitizeCell(t *testing.T) {
	cases := map[string]string{
		"plain":       "plain",
		"a\tb":        "a b",
		"a\nb\r\nc":   "a b  c",
		"=SUM(A1)":    "'=SUM(A1)",
		"+1":          "'+1",
		"-1":          "'-1",
		"@cmd":        "'@cmd",
		"mid=dle":     "mid=dle",
		"":            "",
		"\t=lead tab": " =lead tab",
	}
	for in, want := range cases {
		if got := SanitizeCell(in); got != want {
			t.Errorf("SanitizeCell(%q) = %q, want %q", in, got, want)
		}
	}
}

var header = []string{"task", "sha", "note"}

func TestTSVRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "ledger.tsv")
	rows, err := ReadTSV(path, header)
	if err != nil || rows != nil {
		t.Fatalf("missing file: rows %v err %v, want nil nil", rows, err)
	}
	if err := AppendTSV(path, header, []string{"A1", "abc", "has\ttab"}); err != nil {
		t.Fatal(err)
	}
	if err := AppendTSV(path, header, []string{"B1", "def", "=formula"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "task\tsha\tnote\nA1\tabc\thas tab\nB1\tdef\t'=formula\n"
	if string(data) != want {
		t.Errorf("file %q, want %q", data, want)
	}
	rows, err = ReadTSV(path, header)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rows, [][]string{{"A1", "abc", "has tab"}, {"B1", "def", "'=formula"}}) {
		t.Errorf("rows %v", rows)
	}
	if err := AppendTSV(path, header, []string{"short"}); err == nil {
		t.Error("a row with the wrong cell count was appended")
	}
}

func TestReadTSVRejectsBadFiles(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"wrong header": "task\tsha\n",
		"wrong cells":  "task\tsha\tnote\nA1\tabc\n",
	}
	for name, content := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".tsv")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadTSV(path, header); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestAppendTSVConcurrentCreateWritesOneHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.tsv")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AppendTSV(path, header, []string{fmt.Sprintf("T%d", i), "sha", "n"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	rows, err := ReadTSV(path, header)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 20 {
		t.Errorf("%d rows, want 20", len(rows))
	}
}
