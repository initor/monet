package transcript

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"source/internal/testkit"
)

// decode marshals m and unmarshals it into an Entry.
func decode(t *testing.T, m map[string]any) Entry {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var e Entry
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestFromUserRule(t *testing.T) {
	for _, typ := range []string{"user", "attachment", "assistant"} {
		for _, side := range []bool{false, true} {
			for _, meta := range []bool{false, true} {
				for _, kind := range []string{"", "human", "task-notification"} {
					for _, src := range []string{"", "typed", "other"} {
						for _, atype := range []string{"queued_command", "other"} {
							a := map[string]any{"type": atype, "prompt": "hi"}
							e := map[string]any{"type": typ, "isSidechain": side, "isMeta": meta,
								"message": map[string]any{"content": "hi"}, "attachment": a}
							if kind != "" {
								e["origin"], a["origin"] = map[string]any{"kind": kind}, map[string]any{"kind": kind}
							}
							if src != "" {
								e["promptSource"] = src
							}
							want := !side && !meta && ((typ == "user" && (kind == "human" || src == "typed")) ||
								(typ == "attachment" && atype == "queued_command" && kind == "human"))
							if got, ok := fromUser(decode(t, e)); ok != want || (ok && got != "hi") {
								t.Fatalf("%v: got %q %v, want %v", e, got, ok, want)
							}
						}
					}
				}
			}
		}
	}
}

func TestFromUserTakesItsOwnText(t *testing.T) {
	user := decode(t, map[string]any{"type": "user", "origin": map[string]any{"kind": "human"},
		"message": map[string]any{"content": "m"}, "attachment": map[string]any{"prompt": "p"}})
	queued := decode(t, map[string]any{"type": "attachment", "message": map[string]any{"content": "m"},
		"attachment": map[string]any{"type": "queued_command", "prompt": "p", "origin": map[string]any{"kind": "human"}}})
	if got, ok := fromUser(user); !ok || got != "m" {
		t.Fatalf("user: got %q %v, want m", got, ok)
	}
	if got, ok := fromUser(queued); !ok || got != "p" {
		t.Fatalf("queued: got %q %v, want p", got, ok)
	}
	for _, e := range []Entry{{Type: "user", PromptSource: "typed"}, {Type: "attachment"}} {
		if got, ok := fromUser(e); ok {
			t.Fatalf("%+v: got %q, want false", e, got)
		}
	}
}

func TestTextOf(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want string
		ok   bool
	}{
		{`"a"`, "a", true},
		{`[{"type":"text","text":"a"},{"type":"text","text":"b"}]`, "a\n\nb", true},
		{`[{"type":"text","text":"a"},{"type":"text","text":"b"},{"type":"tool_result"}]`, "", false},
		{`[{"type":"image","text":"x"},{"type":"text","text":"a"}]`, "a", true},
		{`[{"type":"text","text":"a"},1e999]`, "a", true},
		{`[{"type":"image"}]`, "", false},
		{`[{"type":"text","text":5}]`, "", false},
		{`5`, "", false},
		{`null`, "", false},
	} {
		if got, ok := textOf(json.RawMessage(c.raw)); ok != c.ok || got != c.want {
			t.Errorf("%s: got %q %v, want %q %v", c.raw, got, ok, c.want, c.ok)
		}
	}
}

// oneByteEdits returns s, each deletion of one byte, and each substitution or
// insertion of one byte from 0x20 to 0x7e (a copy of the helper of package round).
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

// wordsDiff describes the first message where got and want differ, or gives "".
func wordsDiff(got, want []Words) string {
	for i := range max(len(got), len(want)) {
		var g, w Words
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if i >= len(got) || i >= len(want) || g != w {
			return fmt.Sprintf("message %d (got %d, want %d): got %+v, want %+v", i, len(got), len(want), g, w)
		}
	}
	return ""
}

func TestWordsSkipsMarkerAndCommands(t *testing.T) {
	var entries []map[string]any
	var want []Words
	edits := slices.Concat(oneByteEdits("source relay kickoff: x"), oneByteEdits("<command-name>/x"), oneByteEdits("<command-message>/x"),
		// Kept: a tag on a later line, a tag after a non-ASCII blank (NBSP), an empty text, and another tag.
		// Skipped: a tag after a long run of blanks.
		[]string{"a\n<command-name>/x", "a\n<command-message>/x", string(rune(0xa0)) + "<command-name>/x", "", "<command-args>x",
			strings.Repeat(" ", 64) + "<command-name>/x"})
	for _, edit := range edits {
		for _, prefix := range []string{"", " ", "\t\n", "\v\f\r"} {
			text, id := prefix+edit, fmt.Sprintf("u%d", len(entries))
			entries = append(entries, testkit.Typed(id, "", text))
			trimmed := strings.TrimLeft(text, " \t\n\v\f\r")
			if !strings.HasPrefix(text, "source relay kickoff:") && !strings.HasPrefix(trimmed, "<command-name>") && !strings.HasPrefix(trimmed, "<command-message>") {
				want = append(want, Words{ID: id, At: "2026-01-02T03:04:05.000Z", Text: text})
			}
		}
	}
	msgs, fromHuman, err := OwnerWords(testkit.Transcript(t, t.TempDir(), "s1", entries...), "source relay kickoff:")
	if err != nil || fromHuman != len(entries) {
		t.Fatalf("got err %v and fromHuman %d, want nil and %d", err, fromHuman, len(entries))
	}
	if d := wordsDiff(msgs, want); d != "" {
		t.Fatal(d)
	}
	// The marker is the parameter, not a fixed literal.
	marked := testkit.Transcript(t, t.TempDir(), "s1", testkit.Typed("u1", "", "source relay kickoff: x"), testkit.Typed("u2", "", "gang relay kickoff: x"))
	msgs, fromHuman, err = OwnerWords(marked, "gang relay kickoff:")
	if d := wordsDiff(msgs, []Words{{"u1", "2026-01-02T03:04:05.000Z", "", "source relay kickoff: x"}}); err != nil || fromHuman != 2 || d != "" {
		t.Fatalf("marker gang relay kickoff: got err %v, fromHuman %d (want 2), %s", err, fromHuman, d)
	}
}

func TestWordsSupersedes(t *testing.T) {
	const at = "2026-01-02T03:04:05.000Z"
	noID, byPrompt, byTime := testkit.Typed("", "p1", "g"), testkit.Typed("", "", "h"), testkit.Typed("", "", "i")
	both, noTime := testkit.Typed("u6", "", "j"), testkit.Typed("u7", "", "k")
	byPrompt["promptId"], both["promptId"] = "pr1", "pr2"
	delete(noID, "timestamp")
	delete(noTime, "timestamp")
	// Chain keys from promptId (pr9) and from the timestamp (p8), and a promptId with no timestamp (pr8).
	byPromptInChain, byTimeInChain, promptNoTime := testkit.Typed("", "p9", "l"), testkit.Typed("", "p8", "n"), testkit.Typed("", "", "q")
	byPromptInChain["promptId"], promptNoTime["promptId"] = "pr9", "pr8"
	delete(promptNoTime, "timestamp")
	// Queued messages join the pq chain of q1: q2 (no timestamp, empty text) replaces q1, and u10 replaces q2.
	q2 := testkit.Queued("q2", "", "human")
	delete(q2, "timestamp")
	// The skipped u5 and noID sit in the p1 chain, so neither may become the message that the next one replaces.
	entries := []map[string]any{testkit.Typed("u1", "p1", "a"), testkit.Typed("u5", "p1", "<command-name>/x"), testkit.Typed("u2", "p1", "b"),
		noID, testkit.Typed("u3", "p1", "c"), testkit.Typed("u4", "p2", "d"), testkit.Queued("q1", "e", "human"),
		testkit.Assistant(1, 0, 0), byPrompt, byTime, both, noTime,
		byPromptInChain, testkit.Typed("u9", "p9", "m"), byTimeInChain, testkit.Typed("u8", "p8", "o"), promptNoTime,
		q2, testkit.Typed("u10", "pq", "r")}
	want := []Words{{"u1", at, "", "a"}, {"u2", at, "u1", "b"}, {"u3", at, "u2", "c"}, {"u4", at, "", "d"},
		{"q1", "2026-01-02T03:05:00.000Z", "", "e"}, {"pr1", at, "", "h"}, {at, at, "", "i"}, {"u6", at, "", "j"},
		{"u7", "unknown", "", "k"}, {"pr9", at, "", "l"}, {"u9", at, "pr9", "m"}, {at, at, "", "n"}, {"u8", at, at, "o"},
		{"pr8", "unknown", "", "q"}, {"q2", "unknown", "q1", ""}, {"u10", at, "q2", "r"}}
	msgs, fromHuman, err := OwnerWords(testkit.Transcript(t, t.TempDir(), "s1", entries...), "source relay kickoff:")
	if err != nil || fromHuman != len(entries)-1 {
		t.Fatalf("got err %v and fromHuman %d, want nil and %d", err, fromHuman, len(entries)-1)
	}
	if d := wordsDiff(msgs, want); d != "" {
		t.Fatal(d)
	}
	if _, _, err := OwnerWords(filepath.Join(t.TempDir(), "none.jsonl"), "source relay kickoff:"); err == nil {
		t.Fatal("a missing transcript gave no error")
	}
}
