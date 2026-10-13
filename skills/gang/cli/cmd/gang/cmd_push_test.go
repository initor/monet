package main

import (
	"strings"
	"testing"

	"gang/internal/testkit"
)

func TestPushCommand(t *testing.T) {
	repo := testkit.Repo(t)
	testkit.Origin(t, repo)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	tip := testkit.CommitOn(t, repo, p.Repo.Integration, "T1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	c := &Ctx{Dir: d.Root}

	out, errOut, code := runGang(t, c, "push", "--dry-run")
	if code != 0 || out != "git -C "+repo+" push origin "+tip+":refs/heads/gang-demo\n" {
		t.Fatalf("dry run: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	out, errOut, code = runGang(t, c, "push")
	if code != 0 || out != tip+"\n" {
		t.Fatalf("push: exit %d, stdout %q, stderr %q", code, out, errOut)
	}

	testkit.CommitOn(t, repo, p.Repo.Integration, "leak", map[string]string{"k.pem": "-----BEGIN " + "EC PRIVATE KEY-----\n"})
	_, errOut, code = runGang(t, c, "push")
	if code != 2 || !strings.Contains(errOut, "k.pem:1:private-key") {
		t.Fatalf("secret: exit %d, stderr %q", code, errOut)
	}
}
