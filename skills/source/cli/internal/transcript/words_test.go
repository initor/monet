package transcript

import (
	"encoding/json"
	"testing"
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
