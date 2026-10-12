package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/testkit"
)

func TestPromptsFillsKnownVariablesAndListsTheRest(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	skill := testkit.SkillDir(t)
	c := &Ctx{Dir: d.Root, SkillDir: skill}

	out, errOut, code := runGang(t, c, "prompts", "--task", "T2")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	var want strings.Builder
	for _, k := range testkit.PromptKeys {
		fmt.Fprintf(&want, "===== %s =====\n", k)
		fmt.Fprintf(&want, "%s: T2 on gang/T2 into gang-demo\n", k)
		fmt.Fprintf(&want, "run %s in %s tag preview round {{n}}\n", d.Run("bin", "gang"), repo)
		fmt.Fprintf(&want, "----- engine fills: n\n")
	}
	if out != want.String() {
		t.Fatalf("stdout =\n%s\nwant\n%s", out, want.String())
	}

	out, _, code = runGang(t, c, "prompts")
	if code != 0 || !strings.Contains(out, "common: T1 on gang/T1 into gang-demo\n") {
		t.Fatalf("default task: exit %d\n%s", code, out)
	}
	out, _, code = runGang(t, c, "prompts", "--task", "F04")
	if code != 0 || !strings.Contains(out, "common: F04 on gang/F04 into gang-demo\n") {
		t.Fatalf("repair task: exit %d\n%s", code, out)
	}
	if _, _, code = runGang(t, c, "prompts", "--task", "T9"); code != 1 {
		t.Fatalf("unknown task: exit %d, want 1", code)
	}
}

func TestPromptsRefusesAVariableOutsideVARS(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	skill := testkit.SkillDir(t)
	md := strings.Replace(testkit.RolesMD(), "merge: {{id}}", "merge: {{id}} {{title}}", 1)
	if err := os.WriteFile(filepath.Join(skill, "reference", "roles.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runGang(t, &Ctx{Dir: d.Root, SkillDir: skill}, "prompts")
	if code != 1 || !strings.Contains(errOut, "merge uses {{title}}") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}
