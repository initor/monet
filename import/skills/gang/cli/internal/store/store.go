// Package store holds the gang dir layout and the file primitives every
// run-state writer uses: atomic writes, one-line appends, and sanitized TSV.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Dir is a gang dir, <repo>/.planning/gang/<slug>.
type Dir struct{ Root string }

// Plan is the path of plan/plan.json.
func (d Dir) Plan() string { return filepath.Join(d.Root, "plan", "plan.json") }

// PlanMD is the path of plan/plan.md.
func (d Dir) PlanMD() string { return filepath.Join(d.Root, "plan", "plan.md") }

// Run joins parts under run/.
func (d Dir) Run(parts ...string) string {
	return filepath.Join(append([]string{d.Root, "run"}, parts...)...)
}

// WriteFileAtomic writes data to a temp file in the target directory, then
// renames it over path, so a reader sees the old file or the new one.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // a no-op once the rename succeeds
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// AppendLine appends line and a newline in one O_APPEND write, creating the
// file and its parent dirs when missing. A line holding CR or LF is an error.
func AppendLine(path, line string) error {
	if strings.ContainsAny(line, "\r\n") {
		return fmt.Errorf("append %s: line holds a newline", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// SanitizeCell makes s safe for one TSV cell: tab, CR, and LF become spaces,
// and a leading =, +, -, or @ gets a ' prefix so a spreadsheet reads text.
func SanitizeCell(s string) string {
	s = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(s)
	if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
		s = "'" + s
	}
	return s
}

// ReadTSV returns the rows of a TSV file whose first line must equal header.
// A missing or empty file has no rows. A row with the wrong cell count fails.
func ReadTSV(path string, header []string) ([][]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	want := strings.Join(header, "\t")
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if lines[0] != want {
		return nil, fmt.Errorf("%s: header %q, want %q", path, lines[0], want)
	}
	var rows [][]string
	for i, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != len(header) {
			return nil, fmt.Errorf("%s:%d: %d cells, want %d", path, i+2, len(cells), len(header))
		}
		rows = append(rows, cells)
	}
	return rows, nil
}

// AppendTSV appends one row with every cell sanitized. When the file is new,
// the header and the row appear together through a hard link, so no reader
// sees a file without its header, even when two writers race to create it.
func AppendTSV(path string, header []string, row []string) error {
	if len(row) != len(header) {
		return fmt.Errorf("append %s: %d cells, want %d", path, len(row), len(header))
	}
	cells := make([]string, len(row))
	for i, c := range row {
		cells[i] = SanitizeCell(c)
	}
	line := strings.Join(cells, "\t")
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		created, err := createWith(path, strings.Join(header, "\t")+"\n"+line+"\n")
		if err != nil || created {
			return err
		}
	}
	return AppendLine(path, line)
}

// createWith creates path holding content in one step. It reports false,
// with no error, when path already exists.
func createWith(path, content string) (bool, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".new-*")
	if err != nil {
		return false, err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return false, err
	}
	if err := os.Link(tmp, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
