// Package relay reads and writes the files of the handoff relay hook.
package relay

import (
	"cmp"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"source/internal/store"
)

// errNoSession is the error of a session with no usable id.
var errNoSession = errors.New("relay: CLAUDE_CODE_SESSION_ID is not set, so this session has no hook state to mute")

// SessionID returns CLAUDE_CODE_SESSION_ID, or "" when it is empty or holds
// a slash, a backslash, or "..", so the id is safe in a file name.
func SessionID() string {
	id := os.Getenv("CLAUDE_CODE_SESSION_ID")
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return ""
	}
	return id
}

// StatePath returns <CLAUDE_RELAY_STATE_DIR or /tmp>/monet-relay-<id>.json,
// the hook state file of this session.
func StatePath() (string, error) {
	id := SessionID()
	if id == "" {
		return "", errNoSession
	}
	dir := cmp.Or(os.Getenv("CLAUDE_RELAY_STATE_DIR"), "/tmp")
	return filepath.Join(dir, "monet-relay-"+id+".json"), nil
}

// readState decodes the hook state file loosely. A missing file or one that
// is not a JSON object gives an empty map.
func readState(p string) map[string]any {
	var m map[string]any
	data, err := os.ReadFile(p)
	if err != nil || json.Unmarshal(data, &m) != nil || m == nil {
		return map[string]any{}
	}
	return m
}

// HookMode returns the permissionMode string of the hook state, or "" when
// there is no id, no file, no parse, or no string.
func HookMode() string {
	p, err := StatePath()
	if err != nil {
		return ""
	}
	mode, _ := readState(p)["permissionMode"].(string)
	return mode
}

// Mute sets muted to true in the hook state, keeps every other field, writes
// the file atomically, and returns its path with the folder resolved.
func Mute() (string, error) {
	p, err := StatePath()
	if err != nil {
		return "", err
	}
	m := readState(p)
	m["muted"] = true
	data, err := json.Marshal(m)
	if err == nil {
		err = store.WriteFileAtomic(p, data, 0o644)
	}
	if err != nil {
		return "", err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.Base(p)), nil
}
