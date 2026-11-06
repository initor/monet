package transcript

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"source/internal/testkit"
)

// assistant returns testkit.Assistant(1, 0, 0) with the given model and sidechain mark.
func assistant(model string, side bool) map[string]any {
	e := testkit.Assistant(1, 0, 0)
	e["isSidechain"] = side
	e["message"].(map[string]any)["model"] = model
	return e
}

func TestNewestFields(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name    string
		entries []map[string]any
		want    Fields
	}{
		{"plan case", []map[string]any{{"type": "agent-name", "agentName": "source demo dev"}, testkit.Typed("u1", "p0", "hi"),
			{"type": "user", "permissionMode": "plan"}, testkit.Assistant(1, 0, 0), assistant("claude-haiku-4-5", true),
			assistant("<synthetic>", false), {"type": "user", "agentName": ""}},
			Fields{AgentName: "source demo dev", PermissionMode: "plan", Model: "claude-opus-5-5"}},
		{"newest wins", []map[string]any{{"agentName": "a1", "customTitle": "t1", "permissionMode": "default"},
			assistant("m1", false), {"agentName": "a2", "customTitle": "t2"}, assistant("m2", false), {"customTitle": ""}, {"type": "assistant"}},
			Fields{AgentName: "a2", CustomTitle: "t2", PermissionMode: "default", Model: "m2"}},
		{"none", []map[string]any{{"type": "user"}}, Fields{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testkit.Transcript(t, filepath.Join(dir, tc.name), "s1", tc.entries...)
			if got, err := Newest(p); got != tc.want || err != nil {
				t.Errorf("Newest = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
	if _, err := Newest(filepath.Join(dir, "gone.jsonl")); err == nil {
		t.Error("Newest of a missing file gave no error")
	}
}

// TestNewestModelRule checks the model rule against a plain reference over every type, chain, and model shape.
func TestNewestModelRule(t *testing.T) {
	dir := t.TempDir()
	for _, typ := range []string{"assistant", "user", "attachment", ""} {
		for _, side := range []bool{false, true} {
			for _, model := range []string{"", "<", "<synthetic>", "m<", "claude-x", " <x"} {
				last := assistant(model, side)
				last["type"] = typ
				name := fmt.Sprintf("%s-%t-%x", typ, side, model)
				p := testkit.Transcript(t, filepath.Join(dir, name), "s1", assistant("base", false), last)
				want := "base"
				if typ == "assistant" && !side && model != "" && !strings.HasPrefix(model, "<") {
					want = model
				}
				if got, err := Newest(p); got.Model != want || err != nil {
					t.Errorf("%s: Model = %q, %v; want %q", name, got.Model, err, want)
				}
			}
		}
	}
}
