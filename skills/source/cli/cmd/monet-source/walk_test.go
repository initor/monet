package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/round"
	"source/internal/store"
	"source/internal/words"
)

// TestWalkRun drives one step 1 run through the CLI, from dev start to deliver: research with a FIX and a second
// check, design with a decision item and a wall, a send-back from acceptance, and the final check. Every call must
// give its exit code, and events.tsv must hold the whole walk in order (source.md:363).
func TestWalkRun(t *testing.T) {
	run := newSealableRun(t)
	d := store.Dir{Root: run}
	step := func(want int, stdin string, args ...string) string {
		t.Helper()
		out, e, code := runSource(t, &Ctx{Run: run, SkillDir: "/s", Stdin: strings.NewReader(stdin)}, args...)
		if code != want {
			t.Fatalf("%q: code %d, stdout %q, stderr %q; want %d", args, code, out, e, want)
		}
		return out
	}
	is := func(got, want, what string) {
		t.Helper()
		if got != want {
			t.Fatalf("%s: %q; want %q", what, got, want)
		}
	}
	submit := func(body string) {
		t.Helper()
		p := filepath.Join(run, "dev", "a.md")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		step(exit.OK, "", "submit", "--path", p, "--holder", "d2")
	}
	say := func(id, text string) {
		t.Helper()
		if err := words.Append(d.Words(), words.Entry{ID: id, At: "2026-11-01T12:30:00Z", Text: text}); err != nil {
			t.Fatal(err)
		}
	}
	h1, f1 := []string{"--holder", "h1"}, round.Finding{ID: "F1", Severity: "blocker", Problem: "No rename."}
	open := func() string { return step(exit.OK, "", append([]string{"round", "open"}, h1...)...) }
	closed := func(want string) {
		is(step(exit.OK, "", append([]string{"round", "close"}, h1...)...), want, "round close")
	}
	next := func() { step(exit.OK, "", append([]string{"phase", "next"}, h1...)...) }

	// Step 1: the Brain seals and starts the developer, d1 says hello, and d2 takes over.
	step(exit.OK, "", "charter", "seal", "--words", "w1", "--holder", "h1")
	setBrief(t, run, "reviewer", "# reviewer\nCheck each claim against the code.\n")
	setBrief(t, run, "reader", "# reader\nRead the page as a newcomer.\n")
	step(exit.OK, "", "dev", "start", "--holder", "h1")
	step(exit.OK, "", "dev", "hello", "--holder", "d1")
	step(exit.OK, "", "dev", "takeover", "--from", "d1", "--holder", "d2")

	// Step 2, research: a FIX and its answer, then a second check at max that accepts.
	submit("research one\n")
	open()
	writeVerdict(t, run, 1, "reviewer", "block", f1)
	closed("FIX\n")
	step(exit.OK, `{"answers": [{"finding": "F1", "answer": "fixed", "text": "fixed by v2"}]}`, "respond", "--holder", "d2")
	submit("research two\n")
	if out := open(); !strings.HasPrefix(out, "check 2-reviewer reviewer max ") {
		t.Fatalf("round 2 open: %q; want the reviewer check at max", out)
	}
	writeVerdict(t, run, 2, "reviewer", "accept")
	closed("ACCEPT\n")
	next()

	// Step 3, design: reviewer blocks at v1 and v2, so D1 is a draft. The Brain opens D1, d2 opens W1, the
	// owner clears it, and both personas accept v3.
	for n, body := range []string{"design one\n", "design two\n"} {
		submit(body)
		open()
		writeVerdict(t, run, n+3, "reviewer", "block", f1)
		writeVerdict(t, run, n+3, "reader", "accept")
		closed([]string{"FIX\n", "ACCEPT\nitem D1 draft for reviewer\n"}[n])
	}
	step(exit.OK, `{"default": "keep v1", "way_back": "phase back design", "affects": "acceptance"}`, append([]string{"item", "open", "D1"}, h1...)...)
	is(step(exit.OK, `{"question": "Keep the v1 layout?", "default": "keep it", "blocks": "design"}`, "wall", "open", "--holder", "d2"), "W1\n", "wall open")
	say("w2", "Clear it.")
	step(exit.OK, "", append([]string{"wall", "clear", "W1", "--words", "w2"}, h1...)...)
	submit("design three\n")
	open()
	writeVerdict(t, run, 5, "reviewer", "accept")
	writeVerdict(t, run, 5, "reader", "accept")
	closed("ACCEPT\n")
	next()

	// Step 4: the owner picks other on D1, which sends the run back to design, and v4 passes a round.
	say("w3", "Take the reviewer side.")
	step(exit.OK, "", append([]string{"item", "rule", "D1", "--pick", "other", "--words", "w3"}, h1...)...)
	submit("design four\n")
	open()
	writeVerdict(t, run, 6, "reviewer", "accept")
	writeVerdict(t, run, 6, "reader", "accept")
	closed("ACCEPT\n")
	next()

	// Step 5: the final check of reviewer accepts, and the run is delivered with the accepted versions.
	if out := step(exit.OK, "", append([]string{"accept"}, h1...)...); !strings.HasPrefix(out, "check 7-reviewer reviewer max ") || strings.Count(out, "\n") != 1 {
		t.Fatalf("accept: %q; want one reviewer check at max", out)
	}
	writeVerdict(t, run, 7, "reviewer", "accept")
	closed("ACCEPT\n")
	art := filepath.Join(run, "artifacts")
	is(step(exit.OK, "", append([]string{"deliver"}, h1...)...), "deliverable "+filepath.Join(art, "research", "v2")+"\ndeliverable "+filepath.Join(art, "design", "v4")+"\n", "deliver")

	// Step 6: the whole event list in order, and the last status line.
	rows, err := store.ReadTSV(d.Events(), events.Header)
	var got []string
	for _, r := range rows {
		got = append(got, r[0])
	}
	want := []string{"DEV_RELAYED d1 d2",
		"VERSION research v1", "FINDINGS 1 FIX", "RESPONSE 1", "VERSION research v2", "FINDINGS 2 ACCEPT", "PHASE 1 design",
		"VERSION design v1", "FINDINGS 3 FIX", "VERSION design v2", "FINDINGS 4 ACCEPT", "ITEM D1", "WALL W1", "CLEARED W1 w2",
		"VERSION design v3", "FINDINGS 5 ACCEPT", "PHASE 2 acceptance",
		"RULING D1 other w3", "PHASE 3 design", "VERSION design v4", "FINDINGS 6 ACCEPT", "PHASE 4 acceptance",
		"FINDINGS 7 ACCEPT", "DELIVERED 7"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("events.tsv (err %v)\n%q\nwant\n%q", err, got, want)
	}
	lines := strings.Split(strings.TrimSuffix(step(exit.OK, "", "status"), "\n"), "\n")
	is(lines[len(lines)-1], "events 24, last DELIVERED 7", "the last status line")
}
