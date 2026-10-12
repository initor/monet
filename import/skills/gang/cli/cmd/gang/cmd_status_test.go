package main

import (
	"os"
	"testing"
	"time"

	"gang/internal/testkit"
)

func TestStatusCommand(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	testkit.CommitOn(t, repo, p.Repo.Integration, "T1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	c := &Ctx{Dir: d.Root, Now: func() time.Time { return time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC) }}

	out, errOut, code := runGang(t, c, "status")
	want := "merged 1/2 tasks\nledger CLEAN 0 CHANGES 0 INCOMPLETE 0\ngates open 0\nwave PR none\ndriver free\n"
	if code != 0 || out != want {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if _, err := os.Stat(d.Run("status.md")); err != nil {
		t.Fatal(err)
	}
}
