package main

import (
	"fmt"
	"gang/internal/lock"
	"gang/internal/prq"
	"gang/internal/store"
	"gang/internal/testkit"
	"os"
	"strings"
	"testing"
)

// prFixOpen opens the PR of T1 as holder "me" and adds a reviewed fix F01 for it.
func prFixOpen(t *testing.T) (c *Ctx, repo, ghDir string, d store.Dir, head string) {
	c, repo, ghDir = prRun(t, nil)
	lease(t, c, "me")
	testkit.Answer(t, ghDir, "pr-create", "https://github.com/acme/widget/pull/7\n")
	if _, errOut, code := runGang(t, c, "pr", "open", "T1", "--holder", "me"); code != 0 {
		t.Fatalf("pr open: exit %d, stderr %q", code, errOut)
	}
	d = store.Dir{Root: c.Dir}
	rows, _ := prq.Read(d)
	testkit.LedgerRow(t, d, "F01", testkit.CommitOn(t, repo, "gang-demo", "fix: tidy a\n\nTask: F01\nFixes: T1", map[string]string{"a/T1.go": "package a // tidy\n"}), "review", "CLEAN")
	testkit.Answer(t, ghDir, "pr-view.json", fmt.Sprintf(`{"state":"OPEN","headRefOid":%q}`, rows[0].HeadSHA))
	return c, repo, ghDir, d, rows[0].HeadSHA
}

func TestPRFixNeedsTheDriverLease(t *testing.T) {
	c, _, ghDir, d, head := prFixOpen(t)
	calls := len(testkit.Calls(t, ghDir))
	if _, errOut, code := runGang(t, c, "pr", "fix", "T1", "F01"); code != 64 {
		t.Fatalf("no --holder: exit %d, stderr %q, want 64", code, errOut)
	}
	swap := func(from, to string) {
		lock.Release(d.Run("locks"), "driver", from) // lease fails below if it is still held
		lease(t, c, to)
	}
	swap("me", "other")
	if _, errOut, code := runGang(t, c, "pr", "fix", "T1", "F01", "--holder", "me"); code != 4 || len(testkit.Calls(t, ghDir)) != calls {
		t.Fatalf("other holds the lease: exit %d, stderr %q, gh calls %q, want 4 and no new call", code, errOut, testkit.Calls(t, ghDir))
	}
	swap("other", "me")
	out, errOut, code := runGang(t, c, "pr", "fix", "T1", "F01", "--holder", "me", "--reason", "tidy")
	rows, _ := prq.Read(d)
	if want := fmt.Sprintf("PR #7 head %.7s, fix 1 of 2\nreason: tidy\n", rows[0].HeadSHA); code != 0 || out != want || rows[0].Fixes != 1 || rows[0].HeadSHA == head {
		t.Fatalf("pr fix: exit %d, stdout %q, stderr %q, row %+v, want stdout %q and one fix", code, out, errOut, rows[0], want)
	}
}

func TestPRFixSaysWhenThePushLandedButPrsTsvDidNot(t *testing.T) {
	c, repo, _, d, head := prFixOpen(t)
	_, refused, _ := runGang(t, c, "pr", "fix", "T1", "F99", "--holder", "me")
	hook := strings.TrimSpace(testkit.Git(t, repo, "remote", "get-url", "origin")) + "/hooks/post-receive"
	os.WriteFile(hook, []byte("#!/bin/sh\nchmod a-w '"+d.Run()+"'\n"), 0o755) // a missing hook fails the push notice check below
	t.Cleanup(func() { os.Chmod(d.Run(), 0o755) })
	_, errOut, code := runGang(t, c, "pr", "fix", "T1", "F01", "--holder", "me")
	if want := fmt.Sprintf("but prs.tsv still holds %.7s: ", head); code == 0 || strings.Contains(refused, "pushed") || !strings.Contains(errOut, "pushed ") || !strings.Contains(errOut, " to gang/demo/T1, "+want) {
		t.Fatalf("pr fix: exit %d, stderr %q, want a push notice ending in %q", code, errOut, want)
	}
}
