package main

import (
	"strings"
	"testing"
)

const rsSHA = "3333333333333333333333333333333333333333"

func TestLedgerRecordAndCheck(t *testing.T) {
	high := rsTask("H3")
	high.Risk = "high"
	repo, dir := rsSetup(t, rsPlan(rsTask("A1"), high))
	c := rsCtx(dir, repo)

	out, _, code := runGang(t, c, "ledger", "record", "A1", rsSHA, "--lens", "review", "--blockers", "0", "--majors", "0", "--survivors", "0", "--gate-ran", "--mutation-checked", "--evidence", "review-1.md")
	if code != 0 || out != "CLEAN\n" {
		t.Fatalf("record clean: code=%d out=%q", code, out)
	}
	out, _, code = runGang(t, c, "ledger", "check", "A1", rsSHA)
	if code != 0 || out != "CLEAN\n" {
		t.Fatalf("check A1: code=%d out=%q", code, out)
	}

	out, _, code = runGang(t, c, "ledger", "record", "H3", rsSHA, "--lens", "review", "--blockers", "0", "--majors", "1", "--survivors", "0", "--gate-ran", "--mutation-checked", "--evidence", "r.md")
	if code != 0 || out != "CHANGES\n" {
		t.Fatalf("record changes: code=%d out=%q", code, out)
	}
	out, _, code = runGang(t, c, "ledger", "check", "H3", rsSHA)
	if code != 2 || out != "NOT-VERIFIED review,adversarial\n" {
		t.Fatalf("check H3: code=%d out=%q", code, out)
	}

	out, _, code = runGang(t, c, "ledger", "record", "H3", rsSHA, "--lens", "adversarial", "--blockers", "0", "--majors", "0", "--survivors", "0", "--evidence", "a.md")
	if code != 0 || out != "INCOMPLETE\n" {
		t.Fatalf("record incomplete: code=%d out=%q", code, out)
	}
	// A repair id outside the plan needs only the review lens (risk low).
	if out, _, code = runGang(t, c, "ledger", "check", "F01", rsSHA); code != 2 || out != "NOT-VERIFIED review\n" {
		t.Fatalf("check F01: code=%d out=%q", code, out)
	}
	out, _, code = runGang(t, c, "ledger", "summary")
	if code != 0 || out != "CLEAN 1\nCHANGES 1\nINCOMPLETE 1\n" {
		t.Fatalf("summary: code=%d out=%q", code, out)
	}

	// A surviving mutant with everything else clean is CHANGES (Contract 4).
	out, _, code = runGang(t, c, "ledger", "record", "A1", rsSHA, "--lens", "review", "--blockers", "0", "--majors", "0", "--survivors", "1", "--gate-ran", "--mutation-checked", "--evidence", "review-2.md")
	if code != 0 || out != "CHANGES\n" {
		t.Fatalf("record survivor: code=%d out=%q", code, out)
	}
	if out, _, code = runGang(t, c, "ledger", "check", "A1", rsSHA); code != 2 || out != "NOT-VERIFIED review\n" {
		t.Fatalf("check A1 after a survivor: code=%d out=%q", code, out)
	}
}

func TestLedgerErrors(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"unknown task on check", []string{"ledger", "check", "Z9", rsSHA}, 2},
		{"unknown task on record", []string{"ledger", "record", "Z9", rsSHA, "--lens", "review", "--blockers", "0", "--majors", "0", "--survivors", "0", "--evidence", "e"}, 1},
		{"short sha", []string{"ledger", "record", "A1", "abc123", "--lens", "review", "--blockers", "0", "--majors", "0", "--survivors", "0", "--evidence", "e"}, 1},
		{"bad lens", []string{"ledger", "record", "A1", rsSHA, "--lens", "style", "--blockers", "0", "--majors", "0", "--survivors", "0", "--evidence", "e"}, 1},
		{"no evidence", []string{"ledger", "record", "A1", rsSHA, "--lens", "review", "--blockers", "0", "--majors", "0"}, 64},
		{"no blockers", []string{"ledger", "record", "A1", rsSHA, "--lens", "review", "--majors", "0", "--survivors", "0", "--evidence", "e"}, 64},
		{"no survivors", []string{"ledger", "record", "A1", rsSHA, "--lens", "review", "--blockers", "0", "--majors", "0", "--evidence", "e"}, 64},
		{"no verb", []string{"ledger"}, 64},
		{"bad verb", []string{"ledger", "erase"}, 64},
	}
	for _, tc := range cases {
		if _, stderr, code := runGang(t, c, tc.args...); code != tc.code {
			t.Errorf("%s: code=%d want %d (stderr %q)", tc.name, code, tc.code, stderr)
		}
	}
	if out, _, _ := runGang(t, c, "ledger", "summary"); !strings.HasPrefix(out, "CLEAN 0\n") {
		t.Fatalf("a rejected record reached the ledger: %q", out)
	}
}

func TestLedgerRecordTakesFlagsAnywhere(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	out, _, code := runGang(t, c, "ledger", "record", "--lens", "review", "A1", "--blockers", "0", rsSHA, "--majors", "0", "--survivors", "0", "--gate-ran", "--mutation-checked", "--evidence", "e")
	if code != 0 || out != "CLEAN\n" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}
