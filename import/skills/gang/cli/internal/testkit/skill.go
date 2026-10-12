package testkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PromptKeys is the Contract 8 key list, kept apart from package gen so the
// fixtures pin gen against the contract.
var PromptKeys = []string{
	"common", "implement_new", "implement_resume", "implement_fix", "implement_sync",
	"review_full", "review_fix", "review_merge", "adversarial", "merge", "health", "repair",
}

// FixtureVars are the variables of every fixture prompt. gang prompts fills
// all but n.
var FixtureVars = []string{"gang", "repo", "id", "branch", "integration", "runTag", "n"}

// Template is a minimal engine template with each Contract 8 placeholder once
// and a Contract 0.19 VARS block. Run in node, it checks the checksum and the
// count as the real engine does.
var Template = `// fixture engine template
const TASKS_JSON = /*@@TASKS_JSON@@*/null
const TASKSUM = /*@@TASKSUM@@*/null
const DONE = /*@@DONE@@*/null
const CFG = /*@@CFG@@*/null
const PROMPTS = /*@@PROMPTS@@*/null
` + varsBlock() + `
function fnv(s) { let h = 0x811c9dc5; for (let i = 0; i < s.length; i++) { h ^= s.charCodeAt(i); h = Math.imul(h, 0x01000193) >>> 0 } return h }
if (typeof TASKS_JSON !== 'string' || fnv(TASKS_JSON) !== TASKSUM) throw new Error('task table checksum does not match')
const TASKS = JSON.parse(TASKS_JSON)
if (TASKS.length !== CFG.count) throw new Error('task count does not match')
console.log('ok ' + TASKS.length)
`

// varsBlock is the template's VARS block: every key gets FixtureVars.
func varsBlock() string {
	quoted := `"` + strings.Join(FixtureVars, `", "`) + `"`
	var b strings.Builder
	b.WriteString("const VARS = {\n")
	for i, k := range PromptKeys {
		sep := ","
		if i == len(PromptKeys)-1 {
			sep = ""
		}
		b.WriteString(`  "` + k + `": [` + quoted + "]" + sep + "\n")
	}
	b.WriteString("}")
	return b.String()
}

// RolesMD is a roles.md with one text block per prompt key.
func RolesMD() string {
	fence := strings.Repeat("`", 3)
	var b strings.Builder
	b.WriteString("# Agent prompts\n")
	for _, k := range PromptKeys {
		b.WriteString("\n## " + k + "\n\n" + fence + "text\n")
		b.WriteString(k + ": {{id}} on {{branch}} into {{integration}}\n")
		b.WriteString("run {{gang}} in {{repo}} tag {{runTag}} round {{n}}\n")
		b.WriteString(fence + "\n")
	}
	return b.String()
}

// RolesJSON reads the fixture roles.json (plan 05 ships the real one). Every
// caller is a test two levels under cli/, so the path is fixed.
func RolesJSON(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "roles.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// SkillDir writes the fixture template, roles.md, and roles.json into a temp
// skill dir and returns it.
func SkillDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	WriteSkill(t, dir, Template, RolesMD(), RolesJSON(t))
	return dir
}

// WriteSkill writes a template, roles.md, and roles.json under dir.
func WriteSkill(t *testing.T, dir, template, rolesMD, rolesJSON string) {
	t.Helper()
	files := map[string]string{
		filepath.Join("engine", "engine.template.js"): template,
		filepath.Join("reference", "roles.md"):        rolesMD,
		"roles.json":                                  rolesJSON,
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
