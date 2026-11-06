// Package testkit makes skill folders and session transcripts for _test.go files only.
package testkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// RelayText is the text of reference/relay.md in the folder that Skill makes.
const RelayText = "# Relay steps\n\nThe relay steps of the test skill.\n"

// Skill makes <temp>/skill/reference/relay.md holding RelayText and returns the real path of <temp>/skill.
func Skill(t testing.TB) string {
	ref := filepath.Join(t.TempDir(), "skill", "reference")
	err := errors.Join(os.MkdirAll(ref, 0o755), os.WriteFile(filepath.Join(ref, "relay.md"), []byte(RelayText), 0o644))
	skill, evalErr := filepath.EvalSymlinks(filepath.Dir(ref))
	if err = errors.Join(err, evalErr); err != nil {
		t.Fatal(err)
	}
	return skill
}

// Typed returns a user entry that the user typed, in the transcript shape of Claude Code.
func Typed(uuid, parent, text string) map[string]any {
	return map[string]any{"type": "user", "uuid": uuid, "parentUuid": parent, "timestamp": "2026-01-02T03:04:05.000Z", "isSidechain": false,
		"permissionMode": "acceptEdits", "origin": map[string]any{"kind": "human"}, "promptSource": "typed", "message": map[string]any{"role": "user", "content": text}}
}

// Queued returns a queued_command attachment entry whose origin has the given kind.
func Queued(uuid, text, origin string) map[string]any {
	return map[string]any{"type": "attachment", "uuid": uuid, "parentUuid": "pq", "timestamp": "2026-01-02T03:05:00.000Z",
		"isSidechain": false, "attachment": map[string]any{"type": "queued_command", "origin": map[string]any{"kind": origin}, "prompt": text}}
}

// Assistant returns an assistant entry whose usage holds the given token counts.
func Assistant(input, cacheWrite, cacheRead int) map[string]any {
	return map[string]any{"type": "assistant", "isSidechain": false, "message": map[string]any{"model": "claude-opus-5-5",
		"usage": map[string]any{"input_tokens": input, "cache_creation_input_tokens": cacheWrite, "cache_read_input_tokens": cacheRead}}}
}

// Transcript writes <configDir>/projects/p1/<session>.jsonl with one JSON line per entry and returns its path.
func Transcript(t testing.TB, configDir, session string, entries ...map[string]any) string {
	var buf bytes.Buffer
	path := filepath.Join(configDir, "projects", "p1", session+".jsonl")
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	for _, e := range entries {
		err = errors.Join(err, json.NewEncoder(&buf).Encode(e))
	}
	if err = errors.Join(err, os.WriteFile(path, buf.Bytes(), 0o644)); err != nil {
		t.Fatal(err)
	}
	return path
}
