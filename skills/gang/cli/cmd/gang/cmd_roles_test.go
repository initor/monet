package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/plan"
)

// specRoles is the default table of spec section 6, plus repair from Contract 9.
const specRoles = `{
  "implement": {"model": "opus", "effort": "high"},
  "implement_hard": {"model": "opus", "effort": "xhigh"},
  "review": {"model": "opus", "effort": "xhigh"},
  "review_adversarial": {"model": "opus", "effort": "xhigh"},
  "fix": {"model": "opus", "effort": "high"},
  "sync": {"model": "opus", "effort": "high"},
  "merge": {"model": "opus", "effort": "low"},
  "health": {"model": "opus", "effort": "medium"},
  "repair": {"model": "opus", "effort": "xhigh"},
  "plan_draft": {"model": "opus", "effort": "high"},
  "plan_review": {"model": "opus", "effort": "high"}
}`

func writeSkill(t *testing.T, roles string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "roles.json"), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRolesResolvesAtRunLevel(t *testing.T) {
	p := testPlan()
	p.Models = map[string]plan.Role{"review": {Model: "sonnet"}, "merge": {Effort: "medium"}}
	p.Tasks[0].Model = map[string]plan.Role{"implement": {Model: "haiku"}} // task layer: not run level
	dir := newTestGangDir(t, newTestRepo(t), p)
	out, stderr, code := runGang(t, &Ctx{Dir: dir, SkillDir: writeSkill(t, specRoles)}, "roles")
	want := "implement opus high\n" +
		"implement_hard opus xhigh\n" +
		"review sonnet xhigh\n" +
		"review_adversarial opus xhigh\n" +
		"fix opus high\n" +
		"sync opus high\n" +
		"merge opus medium\n" +
		"health opus medium\n" +
		"repair opus xhigh\n" +
		"plan_draft opus high\n" +
		"plan_review opus high\n"
	if code != 0 || out != want {
		t.Errorf("code %d stderr %q out:\n%s\nwant:\n%s", code, stderr, out, want)
	}
}

func TestRolesErrors(t *testing.T) {
	t.Setenv("GANG_SKILL", "")
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	if _, stderr, code := runGang(t, &Ctx{Dir: dir}, "roles"); code != exit.Usage || !strings.Contains(stderr, "no skill dir") {
		t.Errorf("no skill: code %d stderr %q, want 64", code, stderr)
	}
	cases := map[string]string{
		"missing role":  strings.Replace(specRoles, `"repair": {"model": "opus", "effort": "xhigh"},`, "", 1),
		"unknown role":  strings.Replace(specRoles, `"fix":`, `"fixer":`, 1),
		"raw model id":  strings.Replace(specRoles, `"merge": {"model": "opus"`, `"merge": {"model": "claude-opus-4-1"`, 1),
		"bad effort":    strings.Replace(specRoles, `"effort": "low"`, `"effort": "lowest"`, 1),
		"unknown field": strings.Replace(specRoles, `"effort": "low"`, `"effort": "low", "temp": 1`, 1),
		"not json":      `{"implement":`,
		"missing roles": "",
	}
	for name, roles := range cases {
		skill := writeSkill(t, roles)
		if name == "missing roles" {
			skill = t.TempDir()
		}
		if _, stderr, code := runGang(t, &Ctx{Dir: dir, SkillDir: skill}, "roles"); code != exit.User {
			t.Errorf("%s: code %d stderr %q, want 1", name, code, stderr)
		}
	}
}
