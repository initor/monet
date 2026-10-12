package main

import (
	"strings"
	"testing"
)

func TestBriefCommand(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	if _, _, code := runGang(t, c, "standing", "stop", "hold"); code != 0 {
		t.Fatal("standing stop failed")
	}
	out, _, code := runGang(t, c, "brief", "A1")
	if code != 0 || !strings.HasPrefix(out, "STOP: hold\nTASK A1: Task A1\nbranch: gang/A1\nwave: 1\n") {
		t.Fatalf("code=%d brief:\n%s", code, out)
	}
	if out, _, code := runGang(t, c, "brief", "F01"); code != 0 || !strings.Contains(out, "\nTASK F01: Repair the integration branch\nbranch: gang/F01\n") {
		t.Fatalf("repair brief: code=%d\n%s", code, out)
	}
	for _, id := range []string{"Z9", "F1"} {
		if _, _, code := runGang(t, c, "brief", id); code != 2 {
			t.Fatalf("unknown task %s: code=%d, want 2", id, code)
		}
	}
	if _, _, code := runGang(t, c, "brief"); code != 64 {
		t.Fatalf("no id: code=%d, want 64", code)
	}
}
