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
