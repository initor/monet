package main

import (
	"os"
	"strings"
	"testing"

	"gang/internal/testkit"
)

func TestGenCommandWritesEngine(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	testkit.CommitOn(t, repo, p.Repo.Integration, "T1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	c := &Ctx{Dir: d.Root, SkillDir: testkit.SkillDir(t)}

	out, errOut, code := runGang(t, c, "gen", "--tag", "r1", "--only", "T2", "--max-wave", "2")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || lines[0] != d.Run("engine.js") {
		t.Fatalf("stdout = %q", out)
	}
	if !strings.HasPrefix(lines[1], "tag r1 trunk ") || !strings.Contains(lines[1], " done 1 only T2 max-wave 2 ") {
		t.Errorf("summary = %q", lines[1])
	}
	js, err := os.ReadFile(lines[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"gang":"`+d.Run("bin", "gang")+`"`) {
		t.Error("CFG.gang is not the absolute path of run/bin/gang")
	}
}

func TestGenCommandExitCodes(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	testkit.CommitOn(t, repo, p.Repo.Integration, "start", map[string]string{"x": "y"})
	skill := testkit.SkillDir(t)
	cases := []struct {
		args []string
		want int
	}{
		{[]string{"gen"}, 64},
		{[]string{"gen", "--tag", "a b"}, 64},
		{[]string{"gen", "--tag", "R1"}, 64}, // Contract 0.18: lower case only
		{[]string{"gen", "--tag", "r1", "extra"}, 64},
		{[]string{"gen", "--tag", "r1", "--max-wave", "-1"}, 64},
		{[]string{"gen", "--tag", "r1", "--only", "T9"}, 1},
	}
	for _, tc := range cases {
		c := &Ctx{Dir: d.Root, SkillDir: skill}
		if _, errOut, code := runGang(t, c, tc.args...); code != tc.want {
			t.Errorf("%v: exit %d, want %d (%s)", tc.args, code, tc.want, errOut)
		}
	}
	t.Setenv("GANG_SKILL", "")
	if _, _, code := runGang(t, &Ctx{Dir: d.Root}, "gen", "--tag", "r1"); code != 64 {
		t.Errorf("no skill dir: exit %d, want 64", code)
	}
}
