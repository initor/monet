package main

import (
	"strings"
	"testing"

	"gang/internal/testkit"
)

func TestSizePrintsTheChangedLines(t *testing.T) {
	repo := newTestRepo(t)
	gitT(t, repo, "branch", "gang-demo")
	testkit.CommitOn(t, repo, "gang/A1", "a1\n\nTask: A1", map[string]string{"alpha/a.txt": "1\n2\n3\n"})
	out, stderr, code := runGang(t, &Ctx{Dir: newTestGangDir(t, repo, testPlan())}, "size", "A1")
	if want := "A1: 3 changed lines (limit 100, ceiling 300)\n"; code != 0 || out != want {
		t.Errorf("code %d out %q stderr %q, want 0 and %q", code, out, stderr, want)
	}
}

func TestSizeOverBudgetExits2(t *testing.T) {
	repo := newTestRepo(t)
	gitT(t, repo, "branch", "gang-demo")
	p := testPlan()
	p.Tasks[0].Size.Reason = ""
	testkit.CommitOn(t, repo, "gang/A1", "a1\n\nTask: A1", map[string]string{"alpha/a.txt": strings.Repeat("x\n", 101)})
	_, stderr, code := runGang(t, &Ctx{Dir: newTestGangDir(t, repo, p)}, "size", "A1")
	if want := "size 101 is above limits.task_lines 100"; code != 2 || !strings.Contains(stderr, want) {
		t.Errorf("code %d stderr %q, want 2 and %q", code, stderr, want)
	}
}

func TestSizeShaMeasuresThatCommit(t *testing.T) {
	repo := newTestRepo(t)
	testkit.CommitOn(t, repo, "gang-demo", "trunk", map[string]string{"trunk-only.txt": "t\n"})
	first := testkit.CommitOn(t, repo, "gang/A1", "a1\n\nTask: A1", map[string]string{"alpha/a.txt": "1\n2\n3\n"})
	testkit.CommitOn(t, repo, "gang/A1", "a1 more\n\nTask: A1", map[string]string{"alpha/b.txt": "1\n2\n3\n4\n5\n"})
	out, stderr, code := runGang(t, &Ctx{Dir: newTestGangDir(t, repo, testPlan())}, "size", "A1", "--sha", first)
	if want := "A1: 3 changed lines (limit 100, ceiling 300)\n"; code != 0 || out != want {
		t.Errorf("code %d out %q stderr %q, want 0 and %q", code, out, stderr, want)
	}
}
