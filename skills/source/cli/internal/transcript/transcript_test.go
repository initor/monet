package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"source/internal/testkit"
)

func TestFindTranscript(t *testing.T) {
	cfg, home := t.TempDir(), t.TempDir()
	projects := filepath.Join(cfg, "projects")
	want := filepath.Join(projects, "b", "s1.jsonl")
	if err := os.MkdirAll(filepath.Join(projects, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{want, filepath.Join(projects, "c", "s1.jsonl")} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	missing := filepath.Join(cfg, "gone.jsonl")
	homeProjects := filepath.Join(home, ".claude", "projects")
	for _, tc := range []struct {
		name, cfg, flag, session, want, err string
	}{
		{"flag of a file", cfg, want, "s9", want, ""},
		{"flag of a missing file", cfg, missing, "s1", "", "no transcript at " + missing},
		{"session", cfg, "", "s1", want, ""},
		{"missing session", cfg, "", "s9", "", "no transcript for session s9 under " + projects},
		{"no session", cfg, "", "", "", "no transcript for session (CLAUDE_CODE_SESSION_ID is not set) under " + projects},
		{"home", "", "", "s1", "", "no transcript for session s1 under " + homeProjects},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CONFIG_DIR", tc.cfg)
			t.Setenv("HOME", home)
			got, err := Find(tc.flag, tc.session)
			if got != tc.want || (err == nil) != (tc.err == "") || (err != nil && err.Error() != tc.err) {
				t.Errorf("Find(%q, %q) = %q, %v; want %q, %q", tc.flag, tc.session, got, err, tc.want, tc.err)
			}
		})
	}
	t.Run("home holds it", func(t *testing.T) {
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		t.Setenv("HOME", home)
		p := testkit.Transcript(t, filepath.Join(home, ".claude"), "s1")
		if got, err := Find("", "s1"); got != p || err != nil {
			t.Errorf("Find = %q, %v; want %q", got, err, p)
		}
	})
}

func TestEachSkipsBadLines(t *testing.T) {
	big := strings.Repeat("y", 3<<20)
	lines := []string{"", `""`, "{", "[1]", `"x"`, `{"isSidechain": "yes"}`}
	for _, e := range []map[string]any{testkit.Typed("u1", "p0", "hi"), testkit.Typed("u2", "u1", big)} {
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, string(raw))
	}
	path := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []Entry
	if err := Each(path, func(e Entry) { got = append(got, e) }); err != nil || len(got) != 2 {
		t.Fatalf("Each = %d entries, err %v", len(got), err)
	}
	for i, want := range []struct{ uuid, parent, text string }{{"u1", "p0", "hi"}, {"u2", "u1", big}} {
		e := got[i]
		var text string
		if e.Message == nil || json.Unmarshal(e.Message.Content, &text) != nil || text != want.text {
			t.Errorf("entry %d: message %v, text of %d bytes", i, e.Message, len(text))
		}
		e.Message = nil
		wantEntry := Entry{Type: "user", UUID: want.uuid, ParentUUID: want.parent, Timestamp: "2026-01-02T03:04:05.000Z",
			Origin: &Origin{Kind: "human"}, PromptSource: "typed", PermissionMode: "acceptEdits"}
		if !reflect.DeepEqual(e, wantEntry) {
			t.Errorf("entry %d = %s, want %s", i, js(e), js(wantEntry))
		}
	}
	if err := Each(filepath.Join(t.TempDir(), "none.jsonl"), func(Entry) {}); err == nil {
		t.Error("Each of a missing file gave no error")
	}
	if err := Each(t.TempDir(), func(Entry) {}); err == nil {
		t.Error("Each of a folder gave no error")
	}
}

// js prints v as JSON, so failure lines show the values behind pointer fields.
func js(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestEntryDecodesEveryField(t *testing.T) {
	line := `{"type":"t","uuid":"u","parentUuid":"pu","promptId":"pi","timestamp":"ts","isSidechain":true,"isMeta":true,` +
		`"origin":{"kind":"k"},"promptSource":"ps","message":{"content":[1],"usage":{"input_tokens":1,"cache_creation_input_tokens":2,` +
		`"cache_read_input_tokens":3},"model":"m"},"attachment":{"type":"at","origin":{"kind":"ak"},"prompt":"pr"},` +
		`"agentName":"an","customTitle":"ct","permissionMode":"pm","extra":1}`
	var e Entry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatal(err)
	}
	want := Entry{Type: "t", UUID: "u", ParentUUID: "pu", PromptID: "pi", Timestamp: "ts", IsSidechain: true, IsMeta: true,
		Origin: &Origin{Kind: "k"}, PromptSource: "ps",
		Message:    &Message{Content: json.RawMessage(`[1]`), Usage: &Usage{Input: 1, CacheCreation: 2, CacheRead: 3}, Model: "m"},
		Attachment: &Attachment{Type: "at", Origin: &Origin{Kind: "ak"}, Prompt: json.RawMessage(`"pr"`)},
		AgentName:  "an", CustomTitle: "ct", PermissionMode: "pm"}
	if !reflect.DeepEqual(e, want) {
		t.Errorf("Entry = %s, want %s", js(e), js(want))
	}
}
