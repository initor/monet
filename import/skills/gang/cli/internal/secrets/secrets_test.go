package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gang/internal/testkit"
)

// samples are built by concatenation so no source file holds a whole token.
func samples() map[string]string {
	return map[string]string{
		"private-key":    "-----BEGIN " + "RSA PRIVATE" + " KEY-----",
		"aws-access-key": "AK" + "IA" + strings.Repeat("Q", 16),
		"github-token":   "gh" + "p_" + strings.Repeat("a1", 18),
		"github-pat":     "github" + "_pat_" + strings.Repeat("b2", 25),
		"slack-token":    "xo" + "xb-" + strings.Repeat("3", 12),
		"anthropic-key":  "sk-" + "ant-" + strings.Repeat("c", 24),
		"generic-sk":     "sk" + "-" + strings.Repeat("D4", 16),
		"google-api-key": "AI" + "za" + strings.Repeat("e", 35),
		"openai-key":     "sk-" + "proj-" + strings.Repeat("F6_", 8),
		"stripe-key":     "sk_" + "live_" + strings.Repeat("G7", 12),
		"gitlab-pat":     "gl" + "pat-" + strings.Repeat("H8-", 7),
		"npm-token":      "np" + "m_" + strings.Repeat("I9", 18),
	}
}

// wantOne checks that sample, added as line 1 of x.env, matches only rule.
func wantOne(t *testing.T, rule, sample string) {
	t.Helper()
	diff := "diff --git a/x.env b/x.env\n--- a/x.env\n+++ b/x.env\n@@ -0,0 +1 @@\n+KEY=" + sample + "\n"
	got := ScanDiff(diff)
	want := []Finding{{File: "x.env", Line: 1, Rule: rule}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %+v, want %+v", rule, got, want)
	}
}

func TestEachRuleFiresOnItsSample(t *testing.T) {
	for rule, sample := range samples() {
		wantOne(t, rule, sample)
	}
}

// Other shapes of the same credentials fire their rule too.
func TestRuleVariantsFire(t *testing.T) {
	for _, c := range []struct{ rule, sample string }{
		{"private-key", "-----BEGIN " + "PGP PRIVATE" + " KEY BLOCK-----"},
		{"aws-access-key", "AS" + "IA" + strings.Repeat("R", 16)},
		{"slack-token", "xa" + "pp-" + "1-" + strings.Repeat("A5", 8)},
		{"openai-key", "sk-" + "svcacct-" + strings.Repeat("j", 20)},
		{"openai-key", "sk-" + "admin-" + strings.Repeat("k", 20)},
		{"stripe-key", "rk_" + "live_" + strings.Repeat("L", 20)},
	} {
		wantOne(t, c.rule, c.sample)
	}
}

func TestNearMissesAndRemovedLinesAreClean(t *testing.T) {
	added := []string{
		"AK" + "IA" + strings.Repeat("Q", 15), // one character short
		"sk-" + strings.Repeat("x", 31),
		"const pattern = `AKIA[0-9A-Z]{16}`",
		"-----BEGIN " + "PGP PUBLIC" + " KEY BLOCK-----",
		"sk-" + "proj-" + strings.Repeat("f", 19), // one character short
		"sk_" + "test_" + strings.Repeat("g", 24), // a test-mode key
		"gl" + "pat-" + strings.Repeat("h", 19),   // one character short
		"np" + "m_" + strings.Repeat("i", 35),     // one character short
	}
	lines := []string{
		"diff --git a/a.go b/a.go",
		"--- a/a.go",
		"+++ b/a.go",
		fmt.Sprintf("@@ -3,2 +3,%d @@", len(added)),
		"-old " + samples()["aws-access-key"], // removed lines are not scanned
		"-gone",
	}
	for _, a := range added {
		lines = append(lines, "+"+a)
	}
	if got := ScanDiff(strings.Join(lines, "\n") + "\n"); len(got) != 0 {
		t.Fatalf("got %+v, want none", got)
	}
}

// Hunk counts decide what is content: "+++ b/x" inside a hunk is an added line.
func TestAddedLineThatLooksLikeAHeader(t *testing.T) {
	diff := "diff --git a/n.txt b/n.txt\n--- a/n.txt\n+++ b/n.txt\n@@ -0,0 +1,2 @@\n+first\n+++ " + samples()["slack-token"] + "\n"
	want := []Finding{{File: "n.txt", Line: 2, Rule: "slack-token"}}
	if got := ScanDiff(diff); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Line numbers must agree with the file git wrote, across files and hunks.
func TestLineNumbersMatchGit(t *testing.T) {
	repo := testkit.Repo(t)
	base := make([]string, 30)
	for i := range base {
		base[i] = "line"
	}
	write := func(name string, lines []string) {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", base)
	write("b.txt", base)
	testkit.Git(t, repo, "add", ".")
	testkit.Git(t, repo, "commit", "-q", "-m", "base")

	s := samples()
	a := append([]string{}, base...)
	a[4] = "token " + s["github-token"]
	a = append(a[:20], append([]string{"new", "key " + s["google-api-key"]}, a[20:]...)...)
	b := append([]string{"top " + s["private-key"]}, base[1:]...)
	write("a.txt", a)
	write("b.txt", b)
	testkit.Git(t, repo, "commit", "-q", "-am", "secrets")

	diff := testkit.Git(t, repo, "diff", "--unified=0", "HEAD~1", "HEAD")
	got := ScanDiff(diff)
	want := []Finding{
		{File: "a.txt", Line: lineOf(t, a, s["github-token"]), Rule: "github-token"},
		{File: "a.txt", Line: lineOf(t, a, s["google-api-key"]), Rule: "google-api-key"},
		{File: "b.txt", Line: lineOf(t, b, s["private-key"]), Rule: "private-key"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func lineOf(t *testing.T, lines []string, needle string) int {
	t.Helper()
	for i, l := range lines {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	t.Fatalf("%q not found", needle)
	return 0
}
