package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/store"
)

// demoRef is the plan ref for testPlan, from Contract 0 item 3.
const demoRef = "refs/gang/demo/plan"

// checkoutState is everything plan commit must leave alone: the status, HEAD,
// the current branch, the index, and every branch and tag.
func checkoutState(t *testing.T, repo string) string {
	t.Helper()
	return strings.Join([]string{
		gitT(t, repo, "status", "--porcelain", "--untracked-files=all"),
		gitT(t, repo, "rev-parse", "HEAD"),
		gitT(t, repo, "branch", "--show-current"),
		gitT(t, repo, "ls-files", "--stage"),
		gitT(t, repo, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads", "refs/tags"),
	}, "\n")
}

func TestPlanCommitWritesThePrivateRef(t *testing.T) {
	repo := newTestRepo(t)
	gitT(t, repo, "branch", "gang-demo") // the integration branch exists and must not move
	dir := newTestGangDir(t, repo, testPlan())
	// Render first so the work tree already holds plan.md: the state check
	// below then sees any change plan commit makes to the checkout.
	if _, _, code := runGang(t, &Ctx{Dir: dir}, "render"); code != 0 {
		t.Fatal("render failed")
	}
	before := checkoutState(t, repo)

	out, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "commit")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	sha := strings.TrimSpace(out)
	if got := gitT(t, repo, "rev-parse", demoRef); got != sha {
		t.Errorf("%s at %s, printed %s", demoRef, got, sha)
	}
	if got := gitT(t, repo, "rev-list", "--parents", "-n", "1", sha); got != sha {
		t.Errorf("first plan commit has parents: %q", got)
	}
	if got := gitT(t, repo, "log", "-1", "--format=%s", sha); got != "plan(demo): update plan" {
		t.Errorf("subject %q", got)
	}
	if got := gitT(t, repo, "ls-tree", "--name-only", sha); got != "plan.json\nplan.md" {
		t.Errorf("tree holds %q, want only plan.json and plan.md", got)
	}
	d := store.Dir{Root: dir}
	for src, path := range map[string]string{d.Plan(): "plan.json", d.PlanMD(): "plan.md"} {
		want, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := gitx.Show(repo, sha, path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s in the plan commit differs from %s", path, src)
		}
	}
	if after := checkoutState(t, repo); after != before {
		t.Errorf("main checkout or a branch changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestPlanCommitIsIdempotentAndRerenders(t *testing.T) {
	repo := newTestRepo(t)
	p := testPlan()
	dir := newTestGangDir(t, repo, p)
	first, _, code := runGang(t, &Ctx{Dir: dir}, "plan", "commit")
	if code != 0 {
		t.Fatal("first commit failed")
	}
	again, _, code := runGang(t, &Ctx{Dir: dir}, "plan", "commit")
	if code != 0 || again != first {
		t.Errorf("unchanged plan: code %d printed %q, want %q", code, again, first)
	}
	if got := gitT(t, repo, "rev-parse", demoRef); got+"\n" != first {
		t.Errorf("unchanged plan moved the ref to %s", got)
	}

	p.Tasks[1].Title = "add beta twice"
	if err := plan.Save(store.Dir{Root: dir}.Plan(), p); err != nil {
		t.Fatal(err)
	}
	out, _, code := runGang(t, &Ctx{Dir: dir}, "plan", "commit", "--message", "retitle B1")
	if code != 0 {
		t.Fatal("second commit failed")
	}
	sha := strings.TrimSpace(out)
	if got := gitT(t, repo, "rev-parse", sha+"^"); got+"\n" != first {
		t.Errorf("parent %s, want the previous plan commit %s", got, first)
	}
	if got := gitT(t, repo, "log", "-1", "--format=%s", sha); got != "plan(demo): retitle B1" {
		t.Errorf("subject %q", got)
	}
	md, err := gitx.Show(repo, sha, "plan.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "## B1: add beta twice\n") {
		t.Error("plan commit did not re-render plan.md from the edited plan.json")
	}
}

func TestPlanCommitLostRaceExits3(t *testing.T) {
	repo := newTestRepo(t)
	p := testPlan()
	dir := newTestGangDir(t, repo, p)
	if _, _, code := runGang(t, &Ctx{Dir: dir}, "plan", "commit"); code != 0 {
		t.Fatal("first commit failed")
	}
	p.Tasks[0].Title = "add alpha again"
	if err := plan.Save(store.Dir{Root: dir}.Plan(), p); err != nil {
		t.Fatal(err)
	}
	moved := gitT(t, repo, "rev-parse", "main")
	old := beforePlanCAS
	t.Cleanup(func() { beforePlanCAS = old })
	beforePlanCAS = func() { gitT(t, repo, "update-ref", demoRef, moved) }

	_, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "commit")
	if code != exit.Conflict || !strings.Contains(stderr, "moved") {
		t.Errorf("code %d stderr %q, want 3", code, stderr)
	}
	if got := gitT(t, repo, "rev-parse", demoRef); got != moved {
		t.Errorf("a lost race overwrote the ref: %s, want %s", got, moved)
	}
}

// Contract 0 item 3: the target repo git-ignores .planning/gang/. The plan
// files still reach the ref, because they go in by blob id, not git add.
func TestPlanCommitWorksWhenGangDirIsIgnored(t *testing.T) {
	repo := newTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".git", "info", "exclude"), []byte(".planning/gang/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := newTestGangDir(t, repo, testPlan())
	out, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "commit")
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	if got := gitT(t, repo, "ls-tree", "--name-only", strings.TrimSpace(out)); got != "plan.json\nplan.md" {
		t.Errorf("tree holds %q", got)
	}
}

func TestPlanCommitRefusals(t *testing.T) {
	t.Run("lint problem", func(t *testing.T) {
		repo := newTestRepo(t)
		p := testPlan()
		p.Done = ""
		dir := newTestGangDir(t, repo, p)
		_, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "commit")
		if code != exit.User || !strings.Contains(stderr, "rule 2: - : done is empty") {
			t.Errorf("code %d stderr %q", code, stderr)
		}
		if _, err := gitx.RevParse(repo, demoRef); !errors.Is(err, gitx.ErrNotFound) {
			t.Errorf("a plan with lint problems created the ref: %v", err)
		}
	})
	t.Run("slug mismatch", func(t *testing.T) {
		repo := newTestRepo(t)
		dir := filepath.Join(repo, ".planning", "gang", "other")
		if err := plan.Save(store.Dir{Root: dir}.Plan(), testPlan()); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "commit"); code != exit.User || !strings.Contains(stderr, "does not match plan slug") {
			t.Errorf("code %d stderr %q", code, stderr)
		}
	})
	t.Run("no subcommand", func(t *testing.T) {
		dir := newTestGangDir(t, newTestRepo(t), testPlan())
		if _, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan"); code != exit.Usage || !strings.Contains(stderr, "commit") {
			t.Errorf("code %d stderr %q", code, stderr)
		}
	})
}
