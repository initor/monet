package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"source/internal/charter"
	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
)

// readOr returns the content of path, or "" when it does not exist.
func readOr(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

// setBrief runs persona set for id with brief on stdin and needs exit 0.
func setBrief(t *testing.T, run, id, brief string) {
	t.Helper()
	if _, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(brief)}, "persona", "set", id, "--holder", "h1"); code != exit.OK {
		t.Fatalf("persona set %s: code %d, stderr %q", id, code, e)
	}
}

// The rows run in order on one run, and the seal comes before the row that has seal set.
// A refusal writes neither state.json nor the brief. The first set stores stdin and its hash.
func TestPersonaSet(t *testing.T) {
	run := newSealableRun(t)
	d, now := store.Dir{Root: run}, testNow.Add(time.Hour)
	brief, path := "# reviewer\nCheck each claim against the code.\n", filepath.Join(d.Personas(), "reviewer.md")
	h1 := []string{"--holder", "h1"}
	for _, tc := range []struct {
		name  string
		seal  bool
		args  []string
		stdin string
		code  int
		e     string
	}{
		{"before the seal", false, append([]string{"reviewer"}, h1...), brief, exit.Refused, "charter not sealed: seal it before persona set"},
		{"no id", true, h1, brief, exit.Usage, "usage: monet-source persona set <id> --holder <session>"},
		{"two ids", false, append([]string{"reviewer", "reader"}, h1...), brief, exit.Usage, "usage: monet-source persona set <id> --holder <session>"},
		{"another holder", false, []string{"reviewer", "--holder", "h2"}, brief, exit.Held, "lock held: brain is not held by h2"},
		{"unknown id", false, append([]string{"writer"}, h1...), brief, exit.Refused, "persona writer is not in the charter"},
		{"empty stdin", false, append([]string{"reviewer"}, h1...), "", exit.Refused, "persona reviewer: the brief on stdin is empty"},
		{"blank stdin", false, append([]string{"reviewer"}, h1...), "\n \n", exit.Refused, "persona reviewer: the brief on stdin is empty"},
		{"first set", false, append([]string{"reviewer"}, h1...), brief, exit.OK, ""},
		{"second set", false, append([]string{"reviewer"}, h1...), "# reviewer\nAnother brief.\n", exit.Refused, "persona reviewer already has a brief"},
	} {
		if tc.seal {
			if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
				t.Fatalf("seal: code %d, stderr %q", code, e)
			}
		}
		before, file := readOr(t, d.State()), readOr(t, path)
		out, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(tc.stdin), Now: func() time.Time { return now }}, append([]string{"persona", "set"}, tc.args...)...)
		want := ""
		if tc.e != "" {
			want = "monet-source: " + tc.e + "\n"
		}
		if code != tc.code || out != "" || e != want {
			t.Fatalf("%s: code %d, stdout %q, stderr %q", tc.name, code, out, e)
		}
		if tc.code != exit.OK && (readOr(t, d.State()) != before || readOr(t, path) != file) {
			t.Fatalf("%s: the refusal wrote state.json or the brief", tc.name)
		}
	}
	s, err := state.Load(d)
	if got := readOr(t, path); err != nil || got != brief || len(s.Briefs) != 1 || s.Briefs["reviewer"] != charter.Hash([]byte(brief)) {
		t.Fatalf("brief %q, briefs %v, load err %v; want stdin and its hash", got, s.Briefs, err)
	}
	var raw map[string]json.RawMessage
	var key map[string]string
	if err := json.Unmarshal([]byte(readOr(t, d.State())), &raw); err != nil || json.Unmarshal(raw["briefs"], &key) != nil || len(key) != 1 || key["reviewer"] != charter.Hash([]byte(brief)) {
		t.Fatalf("state.json briefs %s, err %v; want the key briefs with the hash of the brief", raw["briefs"], err)
	}
	checkLease(t, run, "brain", now.Add(12*time.Hour))
}

// An amend that changes the description of the first card drops the brief file and the hash of
// that card, keeps the brief of the second, and lets the changed card get a new brief. A changed
// card whose brief file is already gone still amends cleanly and drops its hash. A change to the
// sources alone, with the same description, also drops the brief of that card.
func TestAmendDropsChangedBriefs(t *testing.T) {
	run := newSealableRun(t)
	d := store.Dir{Root: run}
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("seal: code %d, stderr %q", code, e)
	}
	amend := func(edits ...string) {
		t.Helper()
		writeCharter(t, run, edits...)
		if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "render", "--holder", "h1"); code != exit.OK {
			t.Fatalf("render: code %d, stderr %q", code, e)
		}
		if out, e, code := runSource(t, &Ctx{Run: run}, "charter", "amend", "--words", "w1", "--holder", "h1"); code != exit.OK || out != "" || e != "" {
			t.Fatalf("amend %q: code %d, stdout %q, stderr %q", edits, code, out, e)
		}
	}
	briefs := func(want map[string]string) {
		t.Helper()
		s, err := state.Load(d)
		if err != nil || len(s.Briefs) != len(want) {
			t.Fatalf("briefs %v, load err %v; want %v", s.Briefs, err, want)
		}
		for id, b := range want {
			if s.Briefs[id] != charter.Hash([]byte(b)) || readOr(t, filepath.Join(d.Personas(), id+".md")) != b {
				t.Fatalf("persona %s: hash %s or its file is not the brief %q", id, s.Briefs[id], b)
			}
		}
	}
	setBrief(t, run, "reviewer", "reviewer brief\n")
	setBrief(t, run, "reader", "reader brief\n")
	tests := []string{`"Checks each claim against the code."`, `"Checks each claim against the tests."`}
	amend(tests...)
	briefs(map[string]string{"reader": "reader brief\n"})
	if _, err := os.Stat(filepath.Join(d.Personas(), "reviewer.md")); !os.IsNotExist(err) {
		t.Fatalf("reviewer.md stat err %v, want it removed", err)
	}
	setBrief(t, run, "reviewer", "new reviewer brief\n")
	briefs(map[string]string{"reviewer": "new reviewer brief\n", "reader": "reader brief\n"})
	if err := os.Remove(filepath.Join(d.Personas(), "reader.md")); err != nil {
		t.Fatal(err)
	}
	amend(append(tests, `"Reads the page as a newcomer."`, `"Reads the page once."`)...)
	briefs(map[string]string{"reviewer": "new reviewer brief\n"})
	setBrief(t, run, "reader", "second reader brief\n")
	amend(append(tests, `"Reads the page as a newcomer."`, `"Reads the page once."`, `"sources": []`, `"sources": ["skills/source/cli"]`)...)
	briefs(map[string]string{"reviewer": "new reviewer brief\n"})
	if _, err := os.Stat(filepath.Join(d.Personas(), "reader.md")); !os.IsNotExist(err) {
		t.Fatalf("reader.md stat err %v, want it removed after a sources change", err)
	}
}
