package gen

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// varsEntries are the lines inside a template's VARS block.
const varsEntries = `  "common": ["gang", "repo"], "implement_new": ["gang", "id"], "implement_resume": [], "implement_fix": [],
  "implement_sync": [], "review_full": ["tip", "n"], "review_fix": [], "review_merge": [], "adversarial": [],
  "merge": ["gang", "id", "tip"], "health": [], "repair": []`

// template wraps VARS entries the way engine.template.js holds them.
func template(entries string) string {
	return "// engine\nconst PROMPTS = /*@@PROMPTS@@*/null\nconst VARS = {\n" + entries + "\n}\nconst ROLES = []\n"
}

// rolesMD builds a roles.md with an intro and one section per key. body(k) returns the section text after the heading.
func rolesMD(intro string, keys []string, body func(k string) string) string {
	var b strings.Builder
	b.WriteString("# Agent prompts\n\nIntro prose is allowed.\n\n" + intro + "\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", k, body(k))
	}
	return b.String()
}

func textBlock(k string) string { return "```text\nprompt " + k + " {{id}}\n  indented line\n```" }

func TestParsePromptsReadsEveryKeyVerbatim(t *testing.T) {
	md := rolesMD("", PromptKeys, textBlock)
	got, err := ParsePrompts([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(PromptKeys) {
		t.Fatalf("got %d keys, want %d", len(got), len(PromptKeys))
	}
	for _, k := range PromptKeys {
		if want := "prompt " + k + " {{id}}\n  indented line"; got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
}

func TestParsePromptsLongFenceHoldsInnerFence(t *testing.T) {
	md := rolesMD("", PromptKeys, func(k string) string {
		if k == "review_full" {
			return "````text\nshow this:\n```\ncode\n```\n````"
		}
		return textBlock(k)
	})
	got, err := ParsePrompts([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if want := "show this:\n```\ncode\n```"; got["review_full"] != want {
		t.Fatalf("review_full = %q, want %q", got["review_full"], want)
	}
}

func TestParsePromptsErrors(t *testing.T) {
	with := func(key, section string) func(string) string {
		return func(k string) string {
			if k == key {
				return section
			}
			return textBlock(k)
		}
	}
	all := append([]string{}, PromptKeys...)
	cases := []struct {
		name, md, want string
	}{
		{"missing key", rolesMD("", PromptKeys[1:], textBlock), "missing prompt keys: common"},
		{"unknown key", rolesMD("", append(all, "planner"), textBlock), `unknown prompt key "planner"`},
		{"second block", rolesMD("", PromptKeys, with("merge", textBlock("merge")+"\n\n"+textBlock("merge"))), "## merge has a second block"},
		{"no block", rolesMD("", PromptKeys, with("health", "prose only")), "## health has no text block"},
		{"wrong type", rolesMD("", PromptKeys, with("repair", "```md\nx\n```")), `## repair block is "md", want text`},
		{"unclosed", rolesMD("", PromptKeys, with("repair", "```text\nx")), "fenced block is not closed"},
		{"duplicate heading", rolesMD("", append(all, "common"), textBlock), "## common appears twice"},
		{"block before the first heading", rolesMD("```json\n{}\n```", PromptKeys, textBlock), "fenced block before the first prompt heading"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParsePrompts([]byte(c.md))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

func TestParseVars(t *testing.T) {
	got, err := ParseVars([]byte(template(varsEntries)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got["merge"], []string{"gang", "id", "tip"}) || len(got) != len(PromptKeys) {
		t.Fatalf("got %v", got)
	}
	cases := []struct {
		name, tpl, want string
	}{
		{"no block", "const PROMPTS = null\n", `no line "const VARS = {"`},
		{"not closed", "const VARS = {\n" + varsEntries + "\n", `no closing "}" line`},
		{"missing key", template(strings.Replace(varsEntries, `"repair": []`, `"other": []`, 1)), "no entry for repair"},
		{"unknown key", template(varsEntries + `, "planner": []`), `unknown prompt key "planner"`},
		{"bad name", template(strings.Replace(varsEntries, `"tip", "n"`, `"tip", "n x"`, 1)), `bad variable name "n x"`},
		{"not strict", template(varsEntries + ","), "engine template VARS"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseVars([]byte(c.tpl))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}
