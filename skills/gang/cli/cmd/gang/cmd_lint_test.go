package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/store"
)

func TestLintCleanPlan(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	out, stderr, code := runGang(t, &Ctx{Dir: dir}, "lint")
	if code != 0 || out != "" || stderr != "" {
		t.Errorf("code %d out %q stderr %q, want a silent 0", code, out, stderr)
	}
}

// The line format comes from Contract 3: rule <n>: <task or -> : <msg>.
func TestLintReportsProblems(t *testing.T) {
	p := testPlan()
	p.Tasks[1].DependsOn = []string{"Z9"}
	p.Done = ""
	dir := newTestGangDir(t, newTestRepo(t), p)
	out, stderr, code := runGang(t, &Ctx{Dir: dir}, "lint")
	want := "rule 2: - : done is empty\nrule 3: B1 : depends_on Z9 is not a task\n"
	if code != exit.User || out != want {
		t.Errorf("code %d out %q, want 1 %q", code, out, want)
	}
	if stderr != "gang: 2 lint problems\n" {
		t.Errorf("stderr %q", stderr)
	}
}

func TestLintReportsDecodeErrorsAsRule1(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	if err := os.WriteFile(store.Dir{Root: dir}.Plan(), []byte(`{"version":1,"colour":"red"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runGang(t, &Ctx{Dir: dir}, "lint")
	if code != exit.User || !strings.HasPrefix(out, "rule 1: - : ") || !strings.Contains(out, `unknown field "colour"`) {
		t.Errorf("code %d out %q", code, out)
	}
}

func TestLintAdoptSource(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	src := filepath.Join(t.TempDir(), "graph.json")
	write := func(content string) {
		if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"tasks":[{"id":"A1","deps":[]},{"id":"B1","deps":["A1"],"mergeAfter":[]}]}`)
	if out, _, code := runGang(t, &Ctx{Dir: dir}, "lint", "--adopt-source", src); code != 0 {
		t.Errorf("matching source: code %d out %q", code, out)
	}
	write(`{"tasks":[{"id":"A1","deps":[]},{"id":"B1","deps":[]}]}`)
	out, _, code := runGang(t, &Ctx{Dir: dir}, "lint", "--adopt-source", src)
	if code != exit.User || out != "rule 8: B1 : edge B1 depends_on A1 is in the plan but not in the source\n" {
		t.Errorf("edge mismatch: code %d out %q", code, out)
	}
	if _, _, code := runGang(t, &Ctx{Dir: dir}, "lint", "extra"); code != exit.Usage {
		t.Errorf("positional arg: code %d, want 64", code)
	}
}
