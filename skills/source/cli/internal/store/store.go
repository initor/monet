// Package store holds the run dir layout and the file primitives that every
// run-state writer uses, so a reader never sees a half-written file.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Dir is a run dir, <home>/.planning/source/<slug>.
type Dir struct{ Root string }

// Path joins parts under Root.
func (d Dir) Path(parts ...string) string {
	return filepath.Join(append([]string{d.Root}, parts...)...)
}

// State is the path of state.json.
func (d Dir) State() string { return d.Path("state.json") }

// Events is the path of events.tsv.
func (d Dir) Events() string { return d.Path("events.tsv") }

// Log is the path of log.tsv.
func (d Dir) Log() string { return d.Path("log.tsv") }

// Words is the path of owner-words.md.
func (d Dir) Words() string { return d.Path("owner-words.md") }

// Charter is the path of charter.json.
func (d Dir) Charter() string { return d.Path("charter.json") }

// CharterMD is the path of charter.md.
func (d Dir) CharterMD() string { return d.Path("charter.md") }

// Locks is the path of the locks folder.
func (d Dir) Locks() string { return d.Path("locks") }

// Personas is the path of the personas folder.
func (d Dir) Personas() string { return d.Path("personas") }

// Watch is the path of the watch folder.
func (d Dir) Watch() string { return d.Path("watch") }

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

// WriteJSON writes v as JSON with a two-space indent and a trailing newline,
// atomically and with mode 0644.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, append(data, '\n'), 0o644)
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

// DecodeStrict decodes the one JSON value in path into v. Unknown fields and
// trailing data are errors.
func DecodeStrict(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := dec.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: trailing data after the JSON value", path)
	}
	return nil
}
