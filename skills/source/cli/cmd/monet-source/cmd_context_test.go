package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"source/internal/exit"
	"source/internal/testkit"
)

// contextRun inits the run demo with the session s1 and CLAUDE_CONFIG_DIR a
// temp dir, writes the dev config (50 and 200000), and returns the run, its
// home, and the config dir.
func contextRun(t *testing.T) (run, home, config string) {
	run, config = newRun(t, "demo"), t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "s1")
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK {
		t.Fatalf("init: code %d, stderr %q", code, e)
	}
	if err := os.MkdirAll(filepath.Join(run, "dev", ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(run, "dev", ".claude", "handoff-relay.json"), `{"procedure": "p", "threshold_pct": 50, "window_tokens": 200000}`)
	return run, filepath.Dir(filepath.Dir(filepath.Dir(run))), config
}

// treeBytes maps each file under dir to its text, so a test can tell a write.
func treeBytes(t *testing.T, dir string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			var b []byte
			b, err = os.ReadFile(p)
			m[p] = string(b)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// context reads the config of the role (brain: <home>, dev: <run>/dev) and
// the newest usage of the transcript of the session, or of --transcript, and
// prints the percent, the window, the threshold, and ok or relay, with exit 0
// either way. It writes nothing in the run.
func TestContextDecides(t *testing.T) {
	const w = " of 1000000 tokens, threshold 80%, "
	for _, tc := range []struct {
		role   string
		tokens int
		flag   bool
		want   string
	}{
		{"brain", 1, false, "context 0%" + w + "ok"},
		{"brain", 799400, false, "context 79.9%" + w + "ok"},
		{"brain", 800000, false, "context 80%" + w + "relay"},
		{"brain", 850000, false, "context 85%" + w + "relay"},
		{"brain", 1000000, false, "context 100%" + w + "relay"},
		{"dev", 100000, false, "context 50% of 200000 tokens, threshold 50%, relay"},
		{"dev", 99999, false, "context 50% of 200000 tokens, threshold 50%, relay"},
		{"dev", 99899, false, "context 49.9% of 200000 tokens, threshold 50%, ok"},
		// --transcript wins over the transcript of the session.
		{"brain", 123456, true, "context 12.3%" + w + "ok"},
	} {
		t.Run(fmt.Sprintf("%s %d", tc.role, tc.tokens), func(t *testing.T) {
			run, _, config := contextRun(t)
			args := []string{"context", "--role", tc.role}
			if tc.flag {
				testkit.Transcript(t, config, "s1", testkit.Assistant(900000, 0, 0))
				args = append(args, "--transcript", testkit.Transcript(t, t.TempDir(), "x", testkit.Assistant(tc.tokens, 0, 0)))
			} else {
				testkit.Transcript(t, config, "s1", testkit.Assistant(tc.tokens, 0, 0))
			}
			before := treeBytes(t, run)
			if out, e, code := runSource(t, &Ctx{Run: run}, args...); code != exit.OK || out != tc.want+"\n" || e != "" {
				t.Fatalf("code %d, stdout %q, stderr %q", code, out, e)
			}
			if after := treeBytes(t, run); len(after) != len(before) {
				t.Fatalf("the run went from %d to %d files", len(before), len(after))
			} else {
				for p, b := range before {
					if after[p] != b {
						t.Errorf("%s went from %q to %q", p, b, after[p])
					}
				}
			}
		})
	}
}

// context refuses a missing transcript, a transcript with no counting usage,
// a broken config, and a brain run outside <home>/.planning/source (2), and a
// bad role, an extra word, or a flag it does not take (64).
func TestContextRefuses(t *testing.T) {
	run, home, config := contextRun(t)
	other := realTempDir(t)
	projects := filepath.Join(config, "projects")
	usage := "monet-source: usage: monet-source context --role brain|dev [--transcript <path>]\n"
	missing := filepath.Join(config, "none.jsonl")
	for _, tc := range []struct {
		run  string
		args []string
		code int
		err  string
	}{
		{run, []string{"--role", "brain"}, exit.Refused, "monet-source: context: no transcript for session s1 under " + projects + "\n"},
		{run, []string{"--role", "dev", "--transcript", missing}, exit.Refused, "monet-source: context: no transcript at " + missing + "\n"},
		{other, []string{"--role", "brain"}, exit.Refused, "monet-source: run " + other + " is not at <home>/.planning/source/<slug>\n"},
		{run, []string{"--role", "x"}, exit.Usage, usage},
		{run, []string{}, exit.Usage, usage},
		{run, []string{"--role", "brain", "x"}, exit.Usage, usage},
		{run, []string{"--role", "brain", "--holder=h1"}, exit.Usage, usage},
		{run, []string{"--role", "brain", "--bogus"}, exit.Usage, usage},
	} {
		if out, e, code := runSource(t, &Ctx{Run: tc.run}, append([]string{"context"}, tc.args...)...); code != tc.code || out != "" || e != tc.err {
			t.Errorf("%q: code %d, stdout %q, stderr %q", tc.args, code, out, e)
		}
	}
	p := testkit.Transcript(t, config, "s1", testkit.Assistant(0, 0, 0))
	want := "monet-source: context: no assistant record with usage in " + p + "\n"
	if out, e, code := runSource(t, &Ctx{Run: run}, "context", "--role", "dev"); code != exit.Refused || out != "" || e != want {
		t.Errorf("zero usage: code %d, stdout %q, stderr %q", code, out, e)
	}
	testkit.Transcript(t, config, "s1", testkit.Assistant(1, 0, 0))
	writeText(t, filepath.Join(home, ".claude", "handoff-relay.json"), "{")
	out, e, code := runSource(t, &Ctx{Run: run}, "context", "--role", "brain")
	if want := "monet-source: context: relay config " + filepath.Join(home, ".claude", "handoff-relay.json") + ": unexpected end of JSON input\n"; code != exit.Refused || out != "" || e != want {
		t.Errorf("broken config: code %d, stdout %q, stderr %q", code, out, e)
	}
	if out, e, code := runSource(t, &Ctx{Run: run}, "context", "--role", "dev"); code != exit.OK || out != "context 0% of 200000 tokens, threshold 50%, ok\n" || e != "" {
		t.Errorf("dev beside a broken home config: code %d, stdout %q, stderr %q", code, out, e)
	}
}
