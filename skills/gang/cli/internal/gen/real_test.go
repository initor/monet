package gen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gang/internal/plan"
	"gang/internal/testkit"
)

// simPlan is a 16-task plan shaped like the simulator's own graph: 4 lanes of
// 4, each task after the one above it, some cross-lane deps, some high-risk
// tasks, and 4 merge_after edges, each to its own target.
func simPlan() *plan.Plan {
	p := testkit.Plan()
	p.Tasks = nil
	id := func(row, lane int) string { return fmt.Sprintf("T%02d", row*4+lane+1) }
	for row := 0; row < 4; row++ {
		for lane := 0; lane < 4; lane++ {
			t := plan.Task{ID: id(row, lane), Title: "task " + id(row, lane), Wave: row + 1,
				Size: plan.Size{Lines: 100 + 37*((row*4+lane)%9)}}
			if row > 0 {
				t.DependsOn = []string{id(row-1, lane)}
				if lane > 0 && (row+lane)%3 == 0 {
					t.DependsOn = append(t.DependsOn, id(row-1, lane-1))
				}
			}
			if (row*4+lane)%5 == 2 {
				t.Risk = "high"
			}
			p.Tasks = append(p.Tasks, t)
		}
	}
	for i, w := range [][2]int{{3, 0}, {2, 1}, {3, 1}, {2, 0}} {
		waiter := &p.Tasks[w[0]*4+w[1]]
		waiter.MergeAfter = []string{id(1+i/2, 2+i%2)}
	}
	p.Normalize()
	return p
}

// The real template, prompts, and simulator (plan 04) must work with a
// generated engine. roles.json lands in plan 05, so testdata supplies it.
func TestGenerateWithRealTemplatePromptsAndSim(t *testing.T) {
	tpl, err := os.ReadFile("../../../engine/engine.template.js")
	if err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile("../../../reference/roles.md")
	if err != nil {
		t.Fatal(err)
	}
	vars, err := ParseVars(tpl)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, vs := range vars {
		for _, v := range vs {
			names[v] = true
		}
	}
	if len(names) != 24 {
		t.Errorf("VARS names %d variables, Contract 0.19 says 24", len(names))
	}

	f := newGenFixture(t)
	repo := testkit.Repo(t)
	p := simPlan()
	d := testkit.GangDir(t, repo, p)
	testkit.CommitOn(t, repo, p.Repo.Integration, "start", map[string]string{"start.txt": "x\n"})
	testkit.WriteSkill(t, f.skill, string(tpl), string(md), testkit.RolesJSON(t))
	path, _, err := Generate(repo, d, p, f.skill, filepath.Join(d.Root, "run", "bin", "gang"), f.opts("real"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "/*@@") {
		t.Fatal("generated engine still holds a /*@@ placeholder")
	}

	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH")
	}
	sim, err := exec.Command("node", "../../../sim/sim.mjs", "--engine", path).CombinedOutput()
	if err != nil {
		t.Fatalf("sim.mjs --engine failed: %v\n%s", err, sim)
	}
}

// A risk-high task has one reviewer, the adversarial one, so its prompt holds
// the checks of the full normal review word for word: the map of requirements
// to tests, and the task commit with its Task line.
func TestAdversarialPromptHoldsTheFullReviewChecks(t *testing.T) {
	md, err := os.ReadFile("../../../reference/roles.md")
	if err != nil {
		t.Fatal(err)
	}
	prompts, err := ParsePrompts(md)
	if err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Requirements:", "The commits:"} {
		want := stepText(t, "review_full", prompts["review_full"], label)
		if got := stepText(t, "adversarial", prompts["adversarial"], label); got != want {
			t.Errorf("adversarial step %q =\n%s\nwant the review_full step\n%s", label, got, want)
		}
	}
	if strings.Contains(prompts["adversarial"], "another reviewer") {
		t.Error("the adversarial prompt still says that another reviewer checks the task")
	}
}

var stepRe = regexp.MustCompile(`^[0-9]+\. (.*)$`)

// stepText returns the numbered step of prompt that starts with label, without its number.
func stepText(t *testing.T, key, prompt, label string) string {
	t.Helper()
	for _, line := range strings.Split(prompt, "\n") {
		if m := stepRe.FindStringSubmatch(line); m != nil && strings.HasPrefix(m[1], label) {
			return m[1]
		}
	}
	t.Fatalf("%s has no numbered step that starts with %q", key, label)
	return ""
}
