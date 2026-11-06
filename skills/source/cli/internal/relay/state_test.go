package relay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// oneByteEdits returns s, every deletion of one byte, and every insertion
// and replacement of one byte from 0x20 to 0x7e (a copy of package round).
func oneByteEdits(s string) []string {
	out := []string{s}
	for i := range len(s) + 1 {
		out = append(out, s[:i]+s[min(i+1, len(s)):])
		for b := byte(0x20); b < 0x7f; b++ {
			out = append(out, s[:i]+string(b)+s[i:], s[:i]+string(b)+s[min(i+1, len(s)):])
		}
	}
	return out
}

// stateDir points the hook state of session id at a new temp dir and returns
// it. A non-nil body is written as the state file.
func stateDir(t *testing.T, id string, body *string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CLAUDE_RELAY_STATE_DIR", dir)
	t.Setenv("CLAUDE_CODE_SESSION_ID", id)
	if body != nil {
		if err := os.WriteFile(filepath.Join(dir, "monet-relay-"+id+".json"), []byte(*body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSessionIDForm(t *testing.T) {
	for _, id := range append(oneByteEdits("s1-ab"), "", ".", "..", "a..b", "a.b", "a/b", `a\b`) {
		t.Setenv("CLAUDE_CODE_SESSION_ID", id)
		want := ""
		if id != "" && !strings.ContainsAny(id, `/\`) && !strings.Contains(id, "..") {
			want = id
		}
		if got := SessionID(); got != want {
			t.Fatalf("id %q: got %q, want %q", id, got, want)
		}
	}
}

func TestMuteKeepsFields(t *testing.T) {
	body := `{"fired": true, "permissionMode": "plan", "pct": 81.2}`
	dir := stateDir(t, "s1", &body)
	got, err := Mute()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := resolved + "/monet-relay-s1.json"; got != want {
		t.Fatalf("path: got %q, want %q", got, want)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"fired":true,"muted":true,"pct":81.2,"permissionMode":"plan"}`; string(data) != want {
		t.Fatalf("file: got %s, want %s", data, want)
	}
	if fi, err := os.Stat(got); err != nil || fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode: %v, %v", fi, err)
	}
	if p, err := StatePath(); err != nil || p != filepath.Join(dir, "monet-relay-s1.json") {
		t.Fatalf("StatePath with a dir: %q, %v", p, err)
	}
	os.Unsetenv("CLAUDE_RELAY_STATE_DIR")
	if p, err := StatePath(); err != nil || p != "/tmp/monet-relay-s1.json" {
		t.Fatalf("StatePath with no dir: %q, %v", p, err)
	}
}

func TestMuteStartsFromBadState(t *testing.T) {
	bodies := []string{"{", "null", "[1]", ""}
	for i := -1; i < len(bodies); i++ {
		var body *string
		if i >= 0 {
			body = &bodies[i]
		}
		dir := stateDir(t, "s1", body)
		p, err := Mute()
		if err != nil {
			t.Fatalf("body %d: %v", i, err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "monet-relay-s1.json"))
		if err != nil || string(data) != `{"muted":true}` || !strings.HasSuffix(p, "/monet-relay-s1.json") {
			t.Fatalf("body %d: file %s, path %q, %v", i, data, p, err)
		}
	}
	const msg = "relay: CLAUDE_CODE_SESSION_ID is not set, so this session has no hook state to mute"
	for _, id := range []string{"", "a/b", ".."} {
		dir := stateDir(t, id, nil)
		if p, err := Mute(); err == nil || err.Error() != msg || p != "" {
			t.Fatalf("id %q: got %q, %v", id, p, err)
		}
		if _, err := StatePath(); err == nil || err.Error() != msg {
			t.Fatalf("id %q: StatePath error %v", id, err)
		}
		if ents, err := os.ReadDir(dir); err != nil || len(ents) != 0 {
			t.Fatalf("id %q: dir holds %v, %v", id, ents, err)
		}
	}
}

func TestHookMode(t *testing.T) {
	plan, five, bad := `{"permissionMode": "plan"}`, `{"permissionMode": 5}`, "{"
	for _, c := range []struct {
		id   string
		body *string
		want string
	}{
		{"s1", &plan, "plan"},
		{"s1", &five, ""},
		{"s1", &bad, ""},
		{"s1", nil, ""},
		{"", &plan, ""},
		{"a..b", &plan, ""},
	} {
		stateDir(t, c.id, c.body)
		if got := HookMode(); got != c.want {
			t.Fatalf("id %q, body %v: got %q, want %q", c.id, c.body, got, c.want)
		}
	}
}
