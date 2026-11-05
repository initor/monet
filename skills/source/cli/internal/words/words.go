// Package words reads owner-words.md, the words of the owner. Each entry is a
// header line, the text, and one blank line, as gang relay.mjs:173-174 writes.
package words

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Entry is one message of the owner with the fields of its header line.
type Entry struct{ ID, At, Supersedes, Text string }

// header is the form of each line that starts with "<!-- owner-words uuid=".
var header = regexp.MustCompile(`^<!-- owner-words uuid=(\S+) at=(\S+)(?: supersedes=(\S+))? -->$`)

// Read returns the entries of path in file order, none for a missing file, and an error for a broken header line.
func Read(path string) (out []Entry, err error) {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for i, line := range strings.SplitAfter(string(data), "\n") {
		switch m := header.FindStringSubmatch(strings.TrimSuffix(line, "\n")); {
		case m != nil:
			out = append(out, Entry{ID: m[1], At: m[2], Supersedes: m[3]})
		case strings.HasPrefix(line, "<!-- owner-words uuid="):
			return nil, fmt.Errorf("owner words: %s line %d: bad header", path, i+1)
		case len(out) > 0:
			out[len(out)-1].Text += line
		}
	}
	for i := range out {
		out[i].Text = strings.TrimSuffix(out[i].Text, "\n\n")
	}
	return out, nil
}

// Get returns the entry of path whose ID is id, or the error "owner words: no entry <id>".
func Get(path, id string) (Entry, error) {
	entries, err := Read(path)
	if err != nil {
		return Entry{}, err
	}
	for _, e := range entries {
		if e.ID == id {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("owner words: no entry %s", id)
}
