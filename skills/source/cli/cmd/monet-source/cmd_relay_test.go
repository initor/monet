package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/exit"
	"source/internal/testkit"
	"source/internal/words"
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

// ownerTranscript points the session s1 at a new config dir and writes its
// transcript of entries.
func ownerTranscript(t *testing.T, entries ...map[string]any) {
	t.Helper()
	config := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "s1")
	testkit.Transcript(t, config, "s1", entries...)
}

// relay words copies the typed messages of the session, without the relay
// kickoff and the slash command, into owner-words.md with the edit u3 naming
// u2, and prints the counts and the new ids. A second run adds nothing and
// keeps the file byte for byte. It takes no lease, and it reads and appends
// under <run>/locks/words.mutex.
func TestRelayWordsCopies(t *testing.T) {
	run, at := newRun(t, "demo"), "2026-01-02T03:04:05.000Z"
	file, mutex, old := filepath.Join(run, "owner-words.md"), filepath.Join(run, "locks", "words.mutex"), appendWord
	appendWord = func(p string, e words.Entry) error {
		if _, err := os.Stat(mutex); err != nil {
			return fmt.Errorf("append of %s outside the words mutex: %w", e.ID, err)
		}
		return words.Append(p, e)
	}
	defer func() { appendWord = old }()
	msgs := []map[string]any{testkit.Typed("u1", "p0", "Go on."),
		testkit.Typed("k1", "p1", "source relay kickoff: brain of run demo, from session x (s0)."),
		testkit.Typed("c1", "p1", "<command-name>/clear</command-name>"), testkit.Typed("u2", "p1", "Use v1."), testkit.Typed("u3", "p1", "Use v2.")}
	ownerTranscript(t, msgs...)
	if out, e, code := runSource(t, &Ctx{Run: run}, "relay", "words"); code != exit.OK || out != "owner-words: 3 new, 3 in "+file+"\nu1\nu2\nu3\n" || e != "" {
		t.Fatalf("first run: code %d, stdout %q, stderr %q", code, out, e)
	}
	want := []words.Entry{{ID: "u1", At: at, Text: "Go on."}, {ID: "u2", At: at, Text: "Use v1."}, {ID: "u3", At: at, Supersedes: "u2", Text: "Use v2."}}
	before, err := os.ReadFile(file)
	if got, rerr := words.Read(file); err != nil || rerr != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("words.Read = %+v, err %v %v", got, err, rerr)
	}
	if out, e, code := runSource(t, &Ctx{Run: run}, "relay", "words"); code != exit.OK || out != "owner-words: 0 new, 3 in "+file+"\n" || e != "" {
		t.Errorf("second run: code %d, stdout %q, stderr %q", code, out, e)
	}
	if after, _ := os.ReadFile(file); string(after) != string(before) {
		t.Errorf("the second run changed the file to %q", after)
	}
	// u4 lands in the file while the test holds the mutex, so the run that waits for it reads u4 and adds u5 once.
	ownerTranscript(t, append(msgs, testkit.Typed("u4", "p2", "Ship it."), testkit.Typed("u5", "p3", "Done."), testkit.Typed("u5", "p3", "Done."))...)
	done := make(chan string, 1)
	writeText(t, mutex, "1\n")
	go func() {
		out, e, code := runSource(t, &Ctx{Run: run}, "relay", "words")
		done <- fmt.Sprintf("%d %q %q", code, out, e)
	}()
	time.Sleep(200 * time.Millisecond)
	err = words.Append(file, words.Entry{ID: "u4", At: at, Text: "Ship it."})
	if rerr := os.Remove(mutex); err != nil || rerr != nil {
		t.Fatalf("append u4 while the mutex is held: %v, remove: %v", err, rerr)
	}
	if got, want := <-done, fmt.Sprintf("0 %q \"\"", "owner-words: 1 new, 5 in "+file+"\nu5\n"); got != want {
		t.Errorf("the run behind the mutex gives %s, want %s", got, want)
	}
	if got := listRun(t, run) + " | " + listRun(t, filepath.Join(run, "locks")); got != "locks/ owner-words.md | " {
		t.Errorf("run holds %q", got)
	}
}

// relay words gives 1 when the transcript holds no typed message and the file
// no entry, 2 for a missing transcript or an entry that words.Check refuses
// (before any append), and 64 for an extra word or flag, and then writes no
// file. A directory transcript, a broken file or a failed append gives 1 and
// keeps the file. A typed kickoff alone, or a file with an entry, is not a
// wrong transcript, and a new message then lands through the default append.
func TestRelayWordsRefuses(t *testing.T) {
	run, pre := newRun(t, "demo"), "monet-source: "
	file, gone := filepath.Join(run, "owner-words.md"), filepath.Join(run, "gone.jsonl")
	for _, tc := range []struct {
		entries     []map[string]any
		args        []string
		code        int
		out, stderr string
	}{
		{[]map[string]any{testkit.Assistant(1, 0, 0), testkit.Queued("q1", "hi", "agent")}, nil, exit.User, "",
			pre + "relay: no message from the user in the transcript, so it is the wrong transcript or its format changed\n"},
		{[]map[string]any{testkit.Typed("u1", "p0", "Go on."), testkit.Typed("a b", "p1", "Use v1.")}, nil, exit.Refused, "",
			pre + `relay words: entry a b: owner words: bad id "a b"` + "\n"},
		{nil, []string{"--transcript", gone}, exit.Refused, "", pre + "relay words: no transcript at " + gone + "\n"},
		{nil, []string{"--transcript", run}, exit.User, "", pre + "read " + run + ": is a directory\n"},
		{nil, []string{"x"}, exit.Usage, "", pre + "usage: monet-source relay words [--transcript <path> | --comment <id> --at <time>]\n"},
		{nil, []string{"--holder=x"}, exit.Usage, "", pre + "usage: monet-source relay words [--transcript <path> | --comment <id> --at <time>]\n"},
		{[]map[string]any{testkit.Typed("k1", "p0", "source relay kickoff: dev of run demo, from session x (d0).")}, nil, exit.OK,
			"owner-words: 0 new, 0 in " + file + "\n", ""},
	} {
		ownerTranscript(t, tc.entries...)
		out, e, code := runSource(t, &Ctx{Run: run}, append([]string{"relay", "words"}, tc.args...)...)
		if _, err := os.Stat(file); code != tc.code || out != tc.out || e != tc.stderr || !os.IsNotExist(err) {
			t.Errorf("%q: code %d, stdout %q, stderr %q, file err %v", tc.args, code, out, e, err)
		}
	}
	for _, tc := range []struct {
		text, stderr string
		entry        map[string]any
	}{{"<!-- owner-words uuid=bad at -->\n", "line 1: bad header", testkit.Assistant(1, 0, 0)},
		{"<!-- owner-words uuid=u0 at=x -->\nhi", "does not end in a newline", testkit.Typed("u1", "p0", "Go on.")}} {
		writeText(t, file, tc.text)
		ownerTranscript(t, tc.entry)
		out, e, code := runSource(t, &Ctx{Run: run}, "relay", "words")
		if after, _ := os.ReadFile(file); code != exit.User || out != "" || e != pre+"owner words: "+file+" "+tc.stderr+"\n" || string(after) != tc.text {
			t.Errorf("file %q: code %d, stdout %q, stderr %q, file %q", tc.text, code, out, e, after)
		}
	}
	if err := errors.Join(os.Remove(file), words.Append(file, words.Entry{ID: "u0", At: "x", Text: "hi"})); err != nil {
		t.Fatal(err)
	}
	ownerTranscript(t, testkit.Assistant(1, 0, 0))
	if out, e, code := runSource(t, &Ctx{Run: run}, "relay", "words"); code != exit.OK || out != "owner-words: 0 new, 1 in "+file+"\n" || e != "" {
		t.Errorf("a file with an entry: code %d, stdout %q, stderr %q", code, out, e)
	}
	ownerTranscript(t, testkit.Typed("u9", "p0", "Go on."))
	out, e, code := runSource(t, &Ctx{Run: run}, "relay", "words")
	if got, err := words.Read(file); code != exit.OK || out != "owner-words: 1 new, 2 in "+file+"\nu9\n" || e != "" || err != nil || len(got) != 2 || got[1] != (words.Entry{ID: "u9", At: "2026-01-02T03:04:05.000Z", Text: "Go on."}) {
		t.Errorf("the default append: code %d, stdout %q, stderr %q, words %+v, err %v", code, out, e, got, err)
	}
}

// relay words --comment appends the stdin text, with its trailing newlines cut,
// as the entry of the page comment, once, and reads no transcript. A bad time
// or a missing or extra flag gives 64, and a blank text or an id that
// words.Check refuses gives 2, and then the file does not change.
func TestRelayWordsComment(t *testing.T) {
	run, at, pre := newRun(t, "demo"), "2026-11-01T12:30:00Z", "monet-source: "
	file, usage := filepath.Join(run, "owner-words.md"), pre+"usage: monet-source relay words [--transcript <path> | --comment <id> --at <time>]\n"
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir()) // the session s1 has no transcript
	t.Setenv("CLAUDE_CODE_SESSION_ID", "s1")
	call := func(stdin string, args ...string) (string, string, int) {
		return runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(stdin)}, append([]string{"relay", "words"}, args...)...)
	}
	for _, tc := range []struct{ id, stdin, out, text string }{
		{"c-17", "Keep the v1 layout.\n\n", "owner-words: 1 new, 1 in " + file + "\nc-17\n", "Keep the v1 layout."},
		{"c-17", "Keep the v1 layout.\n\n", "owner-words: 0 new, 1 in " + file + "\n", "Keep the v1 layout."},
		{"c-19", "  Two lines.\nEnd. \n", "owner-words: 1 new, 2 in " + file + "\nc-19\n", "  Two lines.\nEnd. "},
	} {
		before, _ := os.ReadFile(file)
		out, e, code := call(tc.stdin, "--comment", tc.id, "--at", at)
		after, _ := os.ReadFile(file)
		got, err := words.Get(file, tc.id)
		// The file changes exactly when the call adds an entry.
		if code != exit.OK || out != tc.out || e != "" || err != nil || got != (words.Entry{ID: tc.id, At: at, Text: tc.text}) ||
			strings.Contains(out, " 0 new") != (string(after) == string(before)) {
			t.Errorf("%s: code %d, stdout %q, stderr %q, entry %+v, err %v, file %q", tc.id, code, out, e, got, err, after)
		}
	}
	before, _ := os.ReadFile(file)
	for _, tc := range []struct {
		stdin string
		args  []string
		code  int
		err   string
	}{
		{"x", []string{"--comment", "c-18", "--at", "yesterday"}, exit.Usage, usage},
		{" \n", []string{"--comment", "c-18", "--at", at}, exit.Refused, pre + "relay words: the comment text is blank\n"},
		{"x", []string{"--comment", "c 17", "--at", at}, exit.Refused, pre + `relay words: entry c 17: owner words: bad id "c 17"` + "\n"},
		{"x", []string{"--comment", "c-18"}, exit.Usage, usage},
		{"x", []string{"--at", at}, exit.Usage, usage},
		{"x", []string{"--comment", "c-18", "--at", at, "--transcript", file}, exit.Usage, usage},
	} {
		out, e, code := call(tc.stdin, tc.args...)
		if after, _ := os.ReadFile(file); code != tc.code || out != "" || e != tc.err || string(after) != string(before) {
			t.Errorf("%q: code %d, stdout %q, stderr %q, file %q", tc.args, code, out, e, after)
		}
	}
}
