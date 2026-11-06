package testkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSkillHoldsRelayText(t *testing.T) {
	skill := Skill(t)
	real, err := filepath.EvalSymlinks(skill)
	got, _ := os.ReadFile(filepath.Join(skill, "reference", "relay.md"))
	if err != nil || !filepath.IsAbs(skill) || real != skill || string(got) != RelayText || RelayText != "# Relay steps\n\nThe relay steps of the test skill.\n" {
		t.Fatalf("Skill = %q (real path %q, %v), relay.md = %q, RelayText = %q", skill, real, err, got, RelayText)
	}
}

func TestTranscriptWritesLines(t *testing.T) {
	dir := t.TempDir()
	entries := []map[string]any{Typed("u1", "p0", "hi"), Assistant(1, 2, 3)}
	path := Transcript(t, dir, "s1", entries...)
	data, err := os.ReadFile(filepath.Join(dir, "projects", "p1", "s1.jsonl"))
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if err != nil || path != filepath.Join(dir, "projects", "p1", "s1.jsonl") || len(lines) != len(entries) {
		t.Fatalf("Transcript = %q, err %v, lines %q", path, err, lines)
	}
	for i, line := range lines {
		var got, want map[string]any
		raw, _ := json.Marshal(entries[i])
		if json.Unmarshal([]byte(line), &got) != nil || json.Unmarshal(raw, &want) != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("line %d = %s, want %s", i, line, raw)
		}
	}
}

func TestBuildersKeepTranscriptShapes(t *testing.T) {
	if got, _ := json.Marshal([]any{Typed("u1", "p0", "hi"), Queued("q1", "later", "task"), Assistant(1, 2, 3)}); string(got) != `[{"isSidechain":false,"message":{"content":"hi","role":"user"},"origin":{"kind":"human"},"parentUuid":"p0","permissionMode":"acceptEdits","promptSource":"typed","timestamp":"2026-01-02T03:04:05.000Z","type":"user","uuid":"u1"},{"attachment":{"origin":{"kind":"task"},"prompt":"later","type":"queued_command"},"isSidechain":false,"parentUuid":"pq","timestamp":"2026-01-02T03:05:00.000Z","type":"attachment","uuid":"q1"},{"isSidechain":false,"message":{"model":"claude-opus-5-5","usage":{"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"input_tokens":1}},"type":"assistant"}]` {
		t.Errorf("entries = %s", got)
	}
}
