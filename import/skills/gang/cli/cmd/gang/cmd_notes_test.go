package main

import (
	"os"
	"strings"
	"testing"

	"gang/internal/store"
)

func TestReportCommand(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	d := store.Dir{Root: dir}
	c := rsCtx(dir, repo)

	c.Stdin = strings.NewReader("did the thing\n")
	out, _, code := runGang(t, c, "report", "A1")
	if want := d.Run("tasks", "A1", "report.md") + "\n"; code != 0 || out != want {
		t.Fatalf("report: code=%d out=%q want %q", code, out, want)
	}
	c.Stdin = strings.NewReader("fixed it\n")
	if _, _, code := runGang(t, c, "report", "A1", "--heading", "fix 1"); code != 0 {
		t.Fatalf("report --heading: code=%d", code)
	}
	b, _ := os.ReadFile(d.Run("tasks", "A1", "report.md"))
	if want := "## report\n\ndid the thing\n\n## fix 1\n\nfixed it\n"; string(b) != want {
		t.Fatalf("report.md = %q, want %q", b, want)
	}

	c.Stdin = strings.NewReader("findings\n")
	out, _, code = runGang(t, c, "report", "A1", "--name", "review-r1-1.md")
	if want := d.Run("tasks", "A1", "review-r1-1.md") + "\n"; code != 0 || out != want {
		t.Fatalf("report --name: code=%d out=%q", code, out)
	}

	for _, args := range [][]string{
		{"report", "A1", "--name", "owner.md"},
		{"report", "A1", "--name", "../x.md"},
		{"report", "Z9"},
	} {
		c.Stdin = strings.NewReader("x\n")
		if _, _, code := runGang(t, c, args...); code != 1 {
			t.Errorf("%v: code=%d, want 1", args, code)
		}
	}
	c.Stdin = strings.NewReader("repaired\n")
	if out, _, code := runGang(t, c, "report", "F01"); code != 0 || out != d.Run("tasks", "F01", "report.md")+"\n" {
		t.Errorf("report F01: code=%d out=%q", code, out)
	}
	c.Stdin = strings.NewReader("")
	if _, _, code := runGang(t, c, "report", "A1"); code != 1 {
		t.Errorf("empty stdin: code=%d, want 1", code)
	}
	if _, _, code := runGang(t, c, "report"); code != 64 {
		t.Errorf("no id: code=%d, want 64", code)
	}
}

func TestGatesCommand(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1"), rsTask("B2")))
	c := rsCtx(dir, repo)
	out, _, code := runGang(t, c, "gates", "park", "A1", "--question", "Which port?", "--default", "8080")
	if code != 0 || out != "G1\n" {
		t.Fatalf("park: code=%d out=%q", code, out)
	}
	if out, _, _ = runGang(t, c, "gates", "park", "B2", "--question", "Keep it?", "--default", "yes"); out != "G2\n" {
		t.Fatalf("second park: %q", out)
	}
	if out, _, _ = runGang(t, c, "gates", "park", "F01", "--question", "Revert?", "--default", "no"); out != "G3\n" {
		t.Fatalf("repair park: %q", out)
	}
	if _, _, code = runGang(t, c, "gates", "answer", "G3", "--answer", "no"); code != 0 {
		t.Fatalf("answer G3: code=%d", code)
	}
	if _, _, code = runGang(t, c, "gates", "answer", "G1", "--answer", "9090"); code != 0 {
		t.Fatalf("answer: code=%d", code)
	}
	out, _, code = runGang(t, c, "gates", "list")
	if want := "## G2 B2 open\nQuestion: Keep it?\nDefault: yes\n"; code != 0 || out != want {
		t.Fatalf("list: code=%d out=%q want %q", code, out, want)
	}
	cases := []struct {
		args []string
		code int
	}{
		{[]string{"gates", "answer", "G9", "--answer", "x"}, 2},
		{[]string{"gates", "park", "Z9", "--question", "q", "--default", "d"}, 2},
		{[]string{"gates", "park", "A1", "--question", "q"}, 64},
		{[]string{"gates", "answer", "G2"}, 64},
		{[]string{"gates", "close", "G2"}, 64},
	}
	for _, tc := range cases {
		if _, _, code := runGang(t, c, tc.args...); code != tc.code {
			t.Errorf("%v: code=%d, want %d", tc.args, code, tc.code)
		}
	}
}

func TestStandingCommand(t *testing.T) {
	repo, dir := rsSetup(t, rsPlan(rsTask("A1")))
	c := rsCtx(dir, repo)
	steps := []struct {
		args []string
		out  string
	}{
		{[]string{"standing", "show"}, ""},
		{[]string{"standing", "add", "Run", "make check."}, "1. Run make check.\n"},
		{[]string{"standing", "stop", "PR_RED", "wave 1"}, "STOP: PR_RED wave 1\n1. Run make check.\n"},
		{[]string{"standing", "show"}, "STOP: PR_RED wave 1\n1. Run make check.\n"},
		{[]string{"standing", "clear-stop"}, "1. Run make check.\n"},
	}
	for _, s := range steps {
		out, _, code := runGang(t, c, s.args...)
		if code != 0 || out != s.out {
			t.Fatalf("%v: code=%d out=%q want %q", s.args, code, out, s.out)
		}
	}
	for _, args := range [][]string{{"standing"}, {"standing", "add"}, {"standing", "stop"}, {"standing", "wipe"}} {
		if _, _, code := runGang(t, c, args...); code != 64 {
			t.Errorf("%v: code=%d, want 64", args, code)
		}
	}
}
