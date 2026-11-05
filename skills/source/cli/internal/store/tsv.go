package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	if os.IsNotExist(err) {
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
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
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
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
