package store

import (
	"fmt"
	"os"
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
