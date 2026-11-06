package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

// oneByteEdits returns s, each deletion of one byte, and each substitution or
// insertion of one byte from 0x20 to 0x7e. It copies the helper of package round.
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

// respond runs respond as holder with stdin and returns the exit code and stderr.
func respond(t *testing.T, run, holder, stdin string) (int, string) {
	t.Helper()
	_, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(stdin)}, "respond", "--holder", holder)
	return code, e
}

// fixRound submits a new version, opens round n, has reviewer block with F1, and closes it with FIX.
func fixRound(t *testing.T, run string, n int) {
	t.Helper()
	submitFile(t, run, "a.md", fmt.Sprintf("v%d\n", n))
	openRound(t, run)
	writeVerdict(t, run, n, "reviewer", "block", round.Finding{ID: "F1", Severity: "blocker"})
	closeRound(t, run, n, "FIX")
}

// After a FIX round, two answers give 0, response.md holds each answer and its text byte for byte, and
// RESPONSE 1 goes to brain and log.tsv. A second respond on the round gives 2 and keeps the file.
func TestRespondWritesResponse(t *testing.T) {
	run := newLoopRun(t)
	d, p := store.Dir{Root: run}, round.Path(run, 1, "response.md")
	fixRound(t, run, 1)
	in := `{"answers": [{"finding": "F1", "answer": "fixed", "text": "Cites the *rename*.\n\n- store.go:44"}, {"finding": "F2", "answer": "declined", "text": "  Out of scope.\n"}]}`
	want := "# Response to round 1\n\n## F1: fixed\n\nCites the *rename*.\n\n- store.go:44\n\n## F2: declined\n\n  Out of scope.\n\n"
	if code, e := respond(t, run, "d1", in); code != exit.OK || e != "" || readOr(t, p) != want {
		t.Fatalf("respond: code %d, stderr %q, response.md %q; want 0 and %q", code, e, readOr(t, p), want)
	}
	lines, _, err := events.For(d, "brain", 0)
	if err != nil || len(lines) == 0 || lines[len(lines)-1] != "RESPONSE 1" || !strings.HasSuffix(readOr(t, d.Log()), "\tRESPONSE 1\n") {
		t.Fatalf("brain events %q (err %v); want RESPONSE 1 last, also in log.tsv", lines, err)
	}
	if code, e := respond(t, run, "d1", in); code != exit.Refused || e != "monet-source: round 1 already has a response\n" || readOr(t, p) != want {
		t.Fatalf("second respond: code %d, stderr %q; want 2 and the file kept", code, e)
	}
}

// Each one-byte edit of each answer value runs on a fresh closed round, and gives 0 exactly when the
// value is in the literal set. A refused value writes no response.md, so its round stays fresh, and
// only a written response moves the test to the next round.
func TestRespondAnswerSet(t *testing.T) {
	run := newLoopRun(t)
	d, n := store.Dir{Root: run}, 0
	valid := map[string]bool{"fixed": true, "evidence": true, "declined": true}
	for _, v := range slices.Concat(oneByteEdits("fixed"), oneByteEdits("evidence"), oneByteEdits("declined")) {
		if _, err := os.Stat(round.Path(run, n, "response.md")); n == 0 || err == nil {
			if err := state.Update(d, func(s *state.State) error { s.Round++; n = s.Round; return nil }); err != nil {
				t.Fatal(err)
			}
		}
		in, _ := json.Marshal(map[string]any{"answers": []map[string]string{{"finding": "F1", "answer": v, "text": "t"}}})
		code, e := respond(t, run, "d1", string(in))
		_, err := os.Stat(round.Path(run, n, "response.md"))
		refusal := "monet-source: answer F1: want fixed, evidence, or declined\n"
		if (code == exit.OK) != valid[v] || (code != exit.OK && (code != exit.Refused || e != refusal)) || (err == nil) != valid[v] {
			t.Fatalf("answer %q: code %d, stderr %q, response.md err %v", v, code, e, err)
		}
	}
}

// respond refuses no round, an open round, an unknown key, and trailing data with 2, and the Brain
// holder h1 with 4. No refusal writes response.md or an event.
func TestRespondRefuses(t *testing.T) {
	ok := `{"answers": [{"finding": "F1", "answer": "fixed", "text": "t"}]}`
	for _, tc := range []struct {
		name          string
		rounds        int
		holder, stdin string
		code          int
	}{
		{"no round", 0, "d1", ok, exit.Refused},
		{"open round", 2, "d1", ok, exit.Refused},
		{"unknown key", 1, "d1", strings.Replace(ok, `"t"}`, `"t", "note": "x"}`, 1), exit.Refused},
		{"trailing data", 1, "d1", ok + " {}", exit.Refused},
		{"brain holder", 1, "h1", ok, exit.Held},
	} {
		run := newLoopRun(t)
		d := store.Dir{Root: run}
		if tc.rounds > 0 {
			fixRound(t, run, 1)
		}
		if tc.rounds > 1 {
			submitFile(t, run, "a.md", "v2\n")
			openRound(t, run)
		}
		evs, files := readOr(t, d.Events()), roundFiles(t, run)
		code, e := respond(t, run, tc.holder, tc.stdin)
		if code != tc.code || e == "" || readOr(t, d.Events()) != evs || roundFiles(t, run) != files {
			t.Errorf("%s: code %d, stderr %q, or a write of an event or a file under rounds/; want %d", tc.name, code, e, tc.code)
		}
	}
}
