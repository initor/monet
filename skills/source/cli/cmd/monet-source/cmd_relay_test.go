package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"source/internal/exit"
	"source/internal/testkit"
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

// relayRun inits the run demo for brain h1 and dev d1, with the job state job
// and a transcript of one assistant entry (model claude-opus-5-5, no mode) for
// session s1, and the handoffs brain-1.md, dev-1.md, and a.txt. It returns the
// run, its home, and the skill dir.
func relayRun(t *testing.T, job string) (run, home, skill string) {
	run, skill = newRun(t, "demo"), testkit.Skill(t)
	jobDir, _, config := relayEnv(t)
	t.Setenv("CLAUDE_EFFORT", "")
	writeText(t, filepath.Join(jobDir, "state.json"), job)
	testkit.Transcript(t, config, "s1", testkit.Assistant(1, 0, 0))
	for _, args := range [][]string{{"init", "--slug", "demo", "--holder", "h1"}, {"dev", "hello", "--holder", "d1"}} {
		if _, e, code := runSource(t, &Ctx{Run: run, SkillDir: skill}, args...); code != exit.OK {
			t.Fatalf("%q: code %d, stderr %q", args, code, e)
		}
	}
	for _, f := range []string{"brain-1.md", "dev-1.md", "a.txt"} {
		writeText(t, filepath.Join(run, "handoff", f), "# handoff\n")
	}
	return run, filepath.Dir(filepath.Dir(filepath.Dir(run))), skill
}

// relay prompt writes the kickoff beside the handoff and prints the command
// of the next session: the name raised by one, the carried job flags, the
// effort of the role, the model alias, and the mode, in the folder of the
// role. It emits no event.
func TestRelayPrompt(t *testing.T) {
	const flags = `"respawnFlags": ["--effort", "high", "--name", "x", "--plugin-dir", "/p"]`
	for i, tc := range []struct {
		role, holder, mode, job, effort, name, next string
		words                                       []string
	}{
		{"brain", "h1", "auto", `{"name": "source demo brain", ` + flags + `}`, "", "source demo brain", "source demo brain 2",
			[]string{"--effort", "high", "--plugin-dir", "/p", "--model", "opus", "--permission-mode", "auto"}},
		{"dev", "d1", "", `{` + flags + `}`, "", "source demo dev", "source demo dev 2",
			[]string{"--plugin-dir", "/p", "--effort", "medium", "--model", "opus", "--permission-mode", "acceptEdits"}},
		// The job effort and model win over CLAUDE_EFFORT and the transcript.
		{"brain", "h1", "plan", `{"name": "b 7", "respawnFlags": ["--model", "x y", "--effort=low"]}`, "max", "b 7", "b 8",
			[]string{"--model", "x y", "--effort=low", "--permission-mode", "plan"}},
		{"brain", "h1", "plan", `{"respawnFlags": ["--plugin-dir", "/p"]}`, "max", "source demo brain", "source demo brain 2",
			[]string{"--plugin-dir", "/p", "--effort", "max", "--model", "opus", "--permission-mode", "plan"}},
		// The dev drops any job effort, and the mode comes from the job flags.
		{"dev", "d1", "", `{"respawnFlags": ["--effort=low", "--permission-mode", "plan"]}`, "max", "source demo dev", "source demo dev 2",
			[]string{"--effort", "medium", "--model", "opus", "--permission-mode", "plan"}},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			run, home, skill := relayRun(t, tc.job)
			t.Setenv("CLAUDE_EFFORT", tc.effort)
			handoff, prompt := filepath.Join(run, "handoff", tc.role+"-1.md"), filepath.Join(run, "handoff", tc.role+"-1.prompt.txt")
			events, _ := os.ReadFile(filepath.Join(run, "events.tsv"))
			args := []string{"relay", "prompt", "--role", tc.role, "--handoff", handoff, "--holder", tc.holder}
			if tc.mode != "" {
				args = append(args, "--mode", tc.mode)
			}
			folder, toks := home, make([]string, len(tc.words))
			if tc.role == "dev" {
				folder = filepath.Join(run, "dev")
			}
			for i, w := range tc.words {
				toks[i] = token(w)
			}
			want := relayCommand(folder, tc.next, toks, relayKickoffLine(tc.role, "demo", tc.name, tc.holder), prompt) + "\n"
			if out, e, code := runSource(t, &Ctx{Run: run, SkillDir: skill}, args...); code != exit.OK || out != want || e != "" {
				t.Fatalf("code %d, stderr %q\nstdout %q\nwant   %q", code, e, out, want)
			}
			if got, err := os.ReadFile(prompt); err != nil || string(got) != relayKickoff(tc.role, "demo", tc.name, tc.holder, run, skill, handoff) {
				t.Errorf("prompt %q, err %v", got, err)
			}
			if after, _ := os.ReadFile(filepath.Join(run, "events.tsv")); string(after) != string(events) {
				t.Errorf("events.tsv went from %q to %q", events, after)
			}
		})
	}
}

// relay prompt refuses another holder (4), a brain with no mode (2), a bad
// handoff (2), and a bad role, no handoff, an extra word, or no skill dir
// (64), and then writes no kickoff.
func TestRelayPromptRefuses(t *testing.T) {
	run, _, skill := relayRun(t, `{"respawnFlags": ["--effort", "high"]}`)
	t.Setenv("SOURCE_SKILL", "") // newRun set it for init
	h, txt := filepath.Join(run, "handoff", "brain-1.md"), filepath.Join(run, "handoff", "a.txt")
	usage := "monet-source: usage: monet-source relay prompt --role brain|dev --handoff <path> --holder <session> [--mode <mode>] [--transcript <path>]\n"
	for _, tc := range []struct {
		skill string
		args  []string
		code  int
		err   string
	}{
		{skill, []string{"--role", "brain", "--handoff", h, "--holder", "h2", "--mode", "auto"}, exit.Held, ""},
		{skill, []string{"--role", "brain", "--handoff", h, "--holder", "h1"}, exit.Refused,
			"monet-source: relay prompt: no permission mode found for this session, so pass --mode <mode>\n"},
		{skill, []string{"--role", "dev", "--handoff", txt, "--holder", "d1"}, exit.Refused,
			"monet-source: relay prompt: " + txt + " is not a .md file in " + run + "/handoff\n"},
		{skill, []string{"--role", "x", "--handoff", h, "--holder", "h1", "--mode", "auto"}, exit.Usage, usage},
		{skill, []string{"--role", "brain", "--holder", "h1", "--mode", "auto"}, exit.Usage, usage},
		{skill, []string{"--role", "brain", "--handoff", h, "--holder", "h1", "--mode", "auto", "x"}, exit.Usage, usage},
		{"", []string{"--role", "brain", "--handoff", h, "--holder", "h1", "--mode", "auto"}, exit.Usage,
			"monet-source: no skill dir: pass --skill or set SOURCE_SKILL\n"},
	} {
		out, e, code := runSource(t, &Ctx{Run: run, SkillDir: tc.skill}, append([]string{"relay", "prompt"}, tc.args...)...)
		if code != tc.code || out != "" || (tc.err != "" && e != tc.err) || e == "" {
			t.Errorf("%q: code %d, stdout %q, stderr %q", tc.args, code, out, e)
		}
	}
	if got := listRun(t, filepath.Join(run, "handoff")); got != "a.txt brain-1.md dev-1.md" {
		t.Errorf("handoff holds %q", got)
	}
}

// inHandoff passes a path, absolute or relative, exactly when its clean form
// is a .md file directly in <run>/handoff, and returns that form. It follows
// symlinks and refuses a folder.
func TestInHandoff(t *testing.T) {
	run, names := realTempDir(t), []string{"a.md", "a.txt", "a.md.bak"}
	for _, dir := range []string{"handoff", "x", "handoff/sub"} {
		if err := os.MkdirAll(filepath.Join(run, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, n := range names {
			writeText(t, filepath.Join(run, dir, n), "")
		}
	}
	t.Chdir(run)
	var rels []string
	for level, n := []string{""}, 0; n < 3; n++ {
		var next []string
		for _, r := range level {
			for _, s := range []string{"handoff", "x", "sub", "..", "."} {
				next = append(next, r+s+"/")
			}
		}
		level, rels = next, append(rels, next...)
	}
	for _, r := range rels {
		for _, n := range names {
			p := filepath.Clean(run + "/" + r + n)
			want := filepath.Dir(p) == run+"/handoff" && strings.HasSuffix(p, ".md")
			for _, arg := range []string{run + "/" + r + n, r + n} {
				got, err := inHandoff(run, arg)
				var e *exit.Error
				if want && (err != nil || got != p) || !want && (!errors.As(err, &e) || e.Code != exit.Refused || got != "" ||
					e.Msg != "relay prompt: "+arg+" is not a .md file in "+run+"/handoff") {
					t.Errorf("inHandoff(%q) = %q, %v; want ok %v", arg, got, err, want)
				}
			}
		}
	}
	for _, l := range [][2]string{{"handoff/a.md", "x/l.md"}, {"x/a.md", "handoff/out.md"}} {
		if err := os.Symlink(filepath.Join(run, l[0]), filepath.Join(run, l[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(run, "handoff", "d.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := inHandoff(run, run+"/x/l.md"); err != nil || got != run+"/handoff/a.md" {
		t.Errorf("a link to a handoff: %q, %v", got, err)
	}
	for _, p := range []string{"handoff/out.md", "handoff/d.md"} {
		if got, err := inHandoff(run, p); err == nil {
			t.Errorf("%s passes as %q", p, got)
		}
	}
}
