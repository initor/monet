package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
