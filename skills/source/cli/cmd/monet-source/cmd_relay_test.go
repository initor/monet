package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"source/internal/exit"
)

// relay mute sets muted in the hook state of this session, keeps its other
// fields, and writes nothing in the run. An empty or unsafe session id is
// refused, and an argument or a flag is a usage error that writes nothing.
func TestRelayMute(t *testing.T) {
	run, dir, link := realTempDir(t), realTempDir(t), filepath.Join(realTempDir(t), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_RELAY_STATE_DIR", link)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "s1")
	p := filepath.Join(dir, "monet-relay-s1.json")
	if err := os.WriteFile(p, []byte(`{"pct": 81}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, e, code := runSource(t, &Ctx{Run: run}, "relay", "mute"); code != exit.OK || out != "relay: muted "+p+"\n" || e != "" {
		t.Fatalf("mute: code %d, stdout %q, stderr %q", code, out, e)
	}
	var m map[string]any
	data, err := os.ReadFile(p)
	if err != nil || json.Unmarshal(data, &m) != nil || len(m) != 2 || m["muted"] != true || m["pct"] != 81.0 {
		t.Fatalf("state %s, err %v", data, err)
	}
	if ents, err := os.ReadDir(run); err != nil || len(ents) != 0 {
		t.Fatalf("run holds %v, err %v", ents, err)
	}
	for _, id := range []string{"", "a/b"} {
		t.Setenv("CLAUDE_CODE_SESSION_ID", id)
		want := "monet-source: relay: CLAUDE_CODE_SESSION_ID is not set, so this session has no hook state to mute\n"
		if out, e, code := runSource(t, &Ctx{Run: run}, "relay", "mute"); code != exit.Refused || out != "" || e != want {
			t.Errorf("id %q: code %d, stdout %q, stderr %q", id, code, out, e)
		}
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "s2")
	for _, args := range [][]string{{"x"}, {"--holder", "h"}} {
		out, e, code := runSource(t, &Ctx{Run: run}, append([]string{"relay", "mute"}, args...)...)
		if code != exit.Usage || out != "" || e != "monet-source: usage: monet-source relay mute\n" {
			t.Errorf("%q: code %d, stdout %q, stderr %q", args, code, out, e)
		}
	}
	// Later tasks add subcommands, so only the prefix is pinned.
	if out, e, code := runSource(t, &Ctx{Run: run}, "relay"); code != exit.Usage || out != "" || !strings.HasPrefix(e, "monet-source: relay: want a subcommand: ") {
		t.Errorf("no subcommand: code %d, stdout %q, stderr %q", code, out, e)
	}
	if _, err := os.Stat(filepath.Join(dir, "monet-relay-s2.json")); !os.IsNotExist(err) {
		t.Fatalf("a usage error wrote the state: %v", err)
	}
	relaySubs["t-probe"] = func(*Ctx, []string) error { return exit.New(exit.Refused, "probe") }
	defer delete(relaySubs, "t-probe")
	if _, e, code := runSource(t, &Ctx{Run: run}, "relay", "t-probe"); code != exit.Refused || e != "monet-source: probe\n" {
		t.Errorf("relay does not read relaySubs: code %d, stderr %q", code, e)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "s1")
	// A nil *os.File fails every Write with os.ErrInvalid.
	if err := relayMute(&Ctx{Stdout: (*os.File)(nil)}, nil); err != os.ErrInvalid {
		t.Errorf("a failed print of the muted line gives err %v", err)
	}
}
