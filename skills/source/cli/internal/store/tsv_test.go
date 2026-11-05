package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// A tab, CR, or LF becomes a space, and a leading =, +, -, or @ gets a ' prefix.
func TestSanitizeCell(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""}, {"plain", "plain"}, {"a=b", "a=b"},
		{"a\tb\rc\nd", "a b c d"},
		{"=1+2", "'=1+2"}, {"+1", "'+1"}, {"-1", "'-1"}, {"@x", "'@x"},
	} {
		if got := SanitizeCell(tc.in); got != tc.want {
			t.Errorf("SanitizeCell(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ReadTSV gives no rows for a missing or empty file, fails on a directory, and names the bad header or line.
func TestReadTSV(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       [][]string
		wantErr    []string
	}{
		{"missing", "", nil, nil}, {"empty", "", nil, nil}, {"dir", "", nil, []string{"is a directory"}},
		{"rows", "key\tat\na\t1\nb\t2\n", [][]string{{"a", "1"}, {"b", "2"}}, nil},
		{"header", "key\twhen\na\t1\n", nil, []string{`"key\twhen"`, `"key\tat"`}},
		{"cells", "key\tat\na\t1\nb\n", nil, []string{":3:", "1 cells, want 2"}}, {"wide", "key\tat\na\t1\tx\n", nil, []string{":2:", "3 cells, want 2"}},
	} {
		p := filepath.Join(t.TempDir(), strings.TrimPrefix(tc.name, "dir")) // the dir case reads the temp dir itself
		if tc.name != "missing" && tc.name != "dir" && os.WriteFile(p, []byte(tc.body), 0o644) != nil {
			t.Fatalf("%s: write %s failed", tc.name, p)
		}
		rows, err := ReadTSV(p, []string{"key", "at"})
		if (err == nil) != (tc.wantErr == nil) {
			t.Errorf("%s: err %v, want error %v", tc.name, err, tc.wantErr != nil)
			continue
		}
		for _, w := range tc.wantErr {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: err %q does not name %s", tc.name, err, w)
			}
		}
		if !reflect.DeepEqual(rows, tc.want) {
			t.Errorf("%s: rows %q, want %q", tc.name, rows, tc.want)
		}
	}
}

// Eight racing appends to a new file give one header and eight whole rows.
func TestAppendTSVRace(t *testing.T) {
	p, h := filepath.Join(t.TempDir(), "e.tsv"), []string{"key", "at"}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := AppendTSV(p, h, []string{fmt.Sprint("k", i), "t"}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if rows, err := ReadTSV(p, h); err != nil || len(rows) != 8 {
		t.Fatalf("rows %d, err %v", len(rows), err)
	}
}

// AppendTSV sanitizes cells, refuses a bad row or a failed create, makes a 0644 file from a temp file in the target dir, and leaves no temp file; createWith keeps an existing file.
func TestAppendTSV(t *testing.T) {
	p, h := filepath.Join(t.TempDir(), "sub", "e.tsv"), []string{"key", "at"}
	t.Setenv("TMPDIR", filepath.Join(p, "none")) // a temp file made outside the target dir fails the create
	if err := errors.Join(AppendTSV(p, h, []string{"a\tb", "=1"}), AppendTSV(p, h, []string{"c", "2"})); err != nil {
		t.Fatal(err)
	}
	long := filepath.Join(filepath.Dir(p), strings.Repeat("x", 250)) // its temp name passes NAME_MAX, so createWith fails
	if err := AppendTSV(p, h, []string{"short"}); err == nil || !strings.Contains(err.Error(), "1 cells, want 2") || AppendTSV(long, h, h) == nil {
		t.Errorf("short row: err %v, want 1 cells, want 2; or the failed create of %s gave no error", err, long)
	}
	if created, err := createWith(p, "x\n"); created || err != nil {
		t.Errorf("createWith over an existing file: created %v, err %v", created, err)
	}
	fi, err := os.Stat(p)
	if data, _ := os.ReadFile(p); string(data) != "key\tat\na b\t'=1\nc\t2\n" || err != nil || fi.Mode().Perm() != 0o644 {
		t.Errorf("file %q, stat err %v, want mode 0644", data, err)
	}
	if ents, _ := os.ReadDir(filepath.Dir(p)); len(ents) != 1 {
		t.Errorf("dir holds %d entries, want only e.tsv", len(ents))
	}
}
