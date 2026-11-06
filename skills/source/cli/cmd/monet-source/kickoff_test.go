package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestShellQuoteRoundTrip(t *testing.T) {
	for b := 1; b < 128; b++ {
		for _, s := range []string{string(rune(b)), "a" + string(rune(b)) + "b", string(rune(b)) + "'"} {
			out, err := exec.Command("/bin/sh", "-c", "printf %s "+shellQuote(s)).Output()
			if err != nil || string(out) != s {
				t.Fatalf("byte %d: %q came back as %q (err %v)", b, s, out, err)
			}
		}
	}
}

func TestDevKickoff(t *testing.T) {
	want := "source relay kickoff: developer of run demo\n" +
		"\n" +
		"You are the developer of the monet:source run demo. Read /s/reference/rules.md and /s/reference/developer.md with the Read tool first, and follow them.\n" +
		"The run folder is /r/demo, and your folder is /r/demo/dev. Run each engine command as /r/demo/bin/monet-source with --holder <your session id>, and start with dev hello.\n"
	got := devKickoff("demo", "/r/demo", "/s")
	if got != want {
		t.Fatalf("devKickoff:\ngot  %q\nwant %q", got, want)
	}
	if !strings.HasPrefix(got, kickoffMarker+" ") {
		t.Fatalf("first line %q does not start with %q", strings.SplitN(got, "\n", 2)[0], kickoffMarker)
	}
}

func TestDevStartCommand(t *testing.T) {
	want := "(cd '/r/demo/dev' && claude --bg --name 'source demo dev' --effort medium --permission-mode acceptEdits " +
		"'source relay kickoff: developer of run demo. Read /r/demo/handoff/dev-x.md with the Read tool first, and follow it.')"
	if got := devStartCommand("demo", "/r/demo", "/r/demo/handoff/dev-x.md"); got != want {
		t.Fatalf("devStartCommand:\ngot  %q\nwant %q", got, want)
	}
	want = `(cd '/r/it'\''s/dev' && claude --bg --name 'source o'\''k dev' --effort medium --permission-mode acceptEdits ` +
		`'source relay kickoff: developer of run o'\''k. Read /k'\''x with the Read tool first, and follow it.')`
	if got := devStartCommand("o'k", "/r/it's", "/k'x"); got != want {
		t.Fatalf("devStartCommand with quotes:\ngot  %q\nwant %q", got, want)
	}
}

func TestRelayKickoff(t *testing.T) {
	for _, c := range []struct{ role, title, file, first string }{
		{"brain", "Brain", "brain.md", "lock acquire brain"},
		{"dev", "developer", "developer.md", "dev takeover --from s1"},
	} {
		line := relayKickoffLine(c.role, "demo", "source demo brain", "s1")
		if want := kickoffMarker + " " + c.role + " of run demo, from session source demo brain (s1)."; line != want {
			t.Fatalf("relayKickoffLine %s:\ngot  %q\nwant %q", c.role, line, want)
		}
		want := line + "\n" +
			"\n" +
			"The handoff relay started this session, and the session named above has stopped working. This session takes over as the " + c.title + " of the monet:source run demo. Read /s/reference/rules.md and /s/reference/" + c.file + " with the Read tool first, and follow them.\n" +
			"Then read the handoff at /r/demo/handoff/brain-x.md. The run folder is /r/demo, and the words of the owner are in /r/demo/owner-words.md.\n" +
			"Run each engine command as /r/demo/bin/monet-source with --holder <your session id>, and start with " + c.first + ".\n"
		got := relayKickoff(c.role, "demo", "source demo brain", "s1", "/r/demo", "/s", "/r/demo/handoff/brain-x.md")
		if got != want {
			t.Fatalf("relayKickoff %s:\ngot  %q\nwant %q", c.role, got, want)
		}
		if !strings.HasPrefix(got, kickoffMarker+" ") {
			t.Fatalf("first line %q does not start with %q", strings.SplitN(got, "\n", 2)[0], kickoffMarker)
		}
	}
}

func TestRelayCommand(t *testing.T) {
	line := relayKickoffLine("brain", "demo", "source demo brain", "s1")
	msg := "'source relay kickoff: brain of run demo, from session source demo brain (s1).\n\nRead /r/demo/handoff/brain-x.prompt.txt with the Read tool first, and follow it.')"
	want := "(cd '/h' && claude --bg --name 'source demo brain 2' --effort high --permission-mode acceptEdits " + msg
	got := relayCommand("/h", "source demo brain 2", []string{"--effort", "high", "--permission-mode", "acceptEdits"}, line, "/r/demo/handoff/brain-x.prompt.txt")
	if got != want {
		t.Fatalf("relayCommand:\ngot  %q\nwant %q", got, want)
	}
	want = "(cd '/h' && claude --bg --name 'source demo brain 2' " + msg
	if got := relayCommand("/h", "source demo brain 2", nil, line, "/r/demo/handoff/brain-x.prompt.txt"); got != want {
		t.Fatalf("relayCommand with no words:\ngot  %q\nwant %q", got, want)
	}
	line = relayKickoffLine("brain", "o'k", "it's", "s1")
	want = `(cd '/h'\''x' && claude --bg --name 'it'\''s 2' 'source relay kickoff: brain of run o'\''k, from session it'\''s (s1).` + "\n\n" + `Read /p'\''q with the Read tool first, and follow it.')`
	if got := relayCommand("/h'x", "it's 2", nil, line, "/p'q"); got != want {
		t.Fatalf("relayCommand with quotes:\ngot  %q\nwant %q", got, want)
	}
}
