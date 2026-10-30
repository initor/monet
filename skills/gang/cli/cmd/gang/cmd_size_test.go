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

// TestSizeExitPaths runs gang size on one repository. gang-demo holds a trunk
// commit, gang/A1 and gang/F01 are tips, and the other rows measure a commit
// named with --sha. A1 keeps the size.reason that testPlan gives it. The
// expected stdout is exact, the stderr text a fragment.
func TestSizeExitPaths(t *testing.T) {
	repo := newTestRepo(t)
	commit := func(branch string, files map[string]string) string {
		return testkit.CommitOn(t, repo, branch, branch, files)
	}
	commit("gang-demo", map[string]string{"README.md": "trunk\n", "dup.txt": "1\n2\n3\n"})
	commit("gang/A1", map[string]string{"alpha/a.txt": strings.Repeat("x\n", 150)})
	commit("gang/F01", map[string]string{"f.txt": "1\n"})
	huge := commit("huge", map[string]string{"h.txt": strings.Repeat("x\n", 301)})
	clash := commit("clash", map[string]string{"README.md": "tip\n"})
	locks := commit("locks", map[string]string{"go.sum": strings.Repeat("s\n", 40), "s.txt": "1\n2\n3\n"})
	dup := commit("dup", map[string]string{"dup.txt": "1\n2\n3\n"})
	line := func(id, n string) string { return id + ": " + n + " changed lines (limit 100, ceiling 300)\n" }
	dir := newTestGangDir(t, repo, testPlan())
	cases := []struct {
		name        string
		args        []string
		code        int
		out, errHas string
	}{
		{"unknown task", []string{"Z9"}, 1, "", "unknown task Z9"},
		{"repair task", []string{"F01"}, 0, line("F01", "1"), ""},
		{"no task branch", []string{"B1"}, 2, "", "size B1: refs/heads/gang/B1"},
		{"unknown sha", []string{"A1", "--sha", "deadbeef"}, 2, "", "deadbeef"},
		{"no id", nil, 64, "", "usage: gang size"},
		{"two ids", []string{"A1", "B1"}, 64, "", "usage: gang size"},
		{"unknown flag", []string{"A1", "--nope"}, 64, "", "flag provided but not defined"},
		{"conflict", []string{"A1", "--sha", clash}, 3, "", "README.md"},
		{"reason honored", []string{"A1"}, 0, line("A1", "150"), ""},
		{"ceiling beats reason", []string{"A1", "--sha", huge}, 2, line("A1", "301"), "split_lines 300"},
		{"lock files", []string{"A1", "--sha", locks}, 0, line("A1", "3"), ""},
		{"trunk has the change", []string{"A1", "--sha", dup}, 0, line("A1", "0"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, stderr, code := runGang(t, &Ctx{Dir: dir}, append([]string{"size"}, c.args...)...)
			if code != c.code || out != c.out || !strings.Contains(stderr, c.errHas) {
				t.Errorf("code %d out %q stderr %q, want %d, %q and stderr with %q", code, out, stderr, c.code, c.out, c.errHas)
			}
		})
	}
}

func TestSizeWithoutTheIntegrationBranchExits1(t *testing.T) {
	repo := newTestRepo(t)
	testkit.CommitOn(t, repo, "gang/A1", "a1\n\nTask: A1", map[string]string{"alpha/a.txt": "1\n"})
	_, stderr, code := runGang(t, &Ctx{Dir: newTestGangDir(t, repo, testPlan())}, "size", "A1")
	if want := "refs/heads/gang-demo"; code != 1 || !strings.Contains(stderr, want) {
		t.Errorf("code %d stderr %q, want 1 and %q", code, stderr, want)
	}
}
