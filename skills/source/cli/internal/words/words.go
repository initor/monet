// Package words reads and appends owner-words.md, the words of the owner. Each entry is a
// header line, the text, and one blank line, as gang relay.mjs:173-174 writes.
package words

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
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

// idForm is the form of an entry ID that Append accepts, and headerLike finds each text line that Append escapes.
var idForm, headerLike = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`), regexp.MustCompile(`(?m)^<!-- owner-words `)

// Append adds e to path as its header line, its text, and one blank line in one O_APPEND write. It refuses a bad
// or existing ID, a header that does not read back as e, and a file that does not end in a newline, and gives one
// leading space to each text line that starts with "<!-- owner-words ", so that Read keeps the line as text.
// The ID check and the write are two steps, so callers that can race hold the run lock.
func Append(path string, e Entry) error {
	if !idForm.MatchString(e.ID) {
		return fmt.Errorf("owner words: bad id %q", e.ID)
	}
	h := "<!-- owner-words uuid=" + e.ID + " at=" + e.At + strings.Repeat(" supersedes="+e.Supersedes, min(len(e.Supersedes), 1)) + " -->"
	if m := header.FindStringSubmatch(h); m == nil || m[2] != e.At {
		return fmt.Errorf("owner words: entry %s: bad header %q", e.ID, h)
	}
	if entries, err := Read(path); err != nil || slices.ContainsFunc(entries, func(o Entry) bool { return o.ID == e.ID }) {
		return cmp.Or(err, fmt.Errorf("owner words: entry %s exists", e.ID))
	}
	if data, _ := os.ReadFile(path); len(data) > 0 && data[len(data)-1] != '\n' {
		return fmt.Errorf("owner words: %s does not end in a newline", path)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(h + "\n" + headerLike.ReplaceAllString(e.Text, " $0") + "\n\n")
	return errors.Join(err, f.Close())
}
