package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"gang/internal/plan"
	"gang/internal/store"
	"gang/internal/testkit"
)

var now = time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)

type pfFixture struct {
	repo, gh, bin string
	d             store.Dir
	p             *plan.Plan
}

// newPreflight builds a repo where every check passes: cwd at the root, an SSH
// origin that matches repo.remote, the plan committed on refs/gang/demo/plan,
// .planning/gang/ ignored, and a PATH holding only git and fakes of go, node, gh.
func newPreflight(t *testing.T) pfFixture {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	repo := testkit.Repo(t)
	testkit.Git(t, repo, "remote", "add", "origin", "git@github.com:acme/widget.git")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".planning/gang/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testkit.Git(t, repo, "add", ".gitignore")
	testkit.Git(t, repo, "commit", "-q", "-m", "ignore gang state")
	testkit.Git(t, repo, "branch", "gang-demo", "main")
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	commitPlan(t, repo, d, p)

	state, bin := testkit.FakeGH(t)
	testkit.Answer(t, state, "login", "acme-bot\n")
	if err := os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"go", "node"} {
		if err := os.WriteFile(filepath.Join(bin, tool), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Chdir(repo)
	old := freeBytes
	freeBytes = func(string) (uint64, error) { return 100 << 30, nil }
	t.Cleanup(func() { freeBytes = old })
	return pfFixture{repo: repo, gh: state, bin: bin, d: d, p: p}
}

// commitPlan saves p and commits plan.json to the plan ref, as gang plan
// commit does (Contract 0.3).
func commitPlan(t *testing.T, repo string, d store.Dir, p *plan.Plan) {
	t.Helper()
	if err := plan.Save(d.Plan(), p); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(d.Plan())
	if err != nil {
		t.Fatal(err)
	}
	testkit.CommitRef(t, repo, plan.Ref(p.Slug), "plan", map[string]string{"plan.json": string(b)})
}

func run(t *testing.T, f pfFixture) map[string]Check {
	t.Helper()
	checks, err := Run(f.repo, f.d, f.p, "s1", now)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]Check{}
	var names []string
	for _, c := range checks {
		m[c.Name] = c
		names = append(names, c.Name)
	}
	if got := strings.Join(names, ","); got != "repo,driver,tools,gh,clean checkout,integration,ignored,plan,disk,worktrees" {
		t.Fatalf("check order = %s", got)
	}
	return m
}

func TestPreflightAllPass(t *testing.T) {
	f := newPreflight(t)
	for name, c := range run(t, f) {
		if !c.OK {
			t.Errorf("%s failed: %s", name, c.Detail)
		}
	}
	b, err := os.ReadFile(f.d.Run("locks", "driver.json"))
	if err != nil || !strings.Contains(string(b), `"holder":"s1"`) || !strings.Contains(string(b), `"pid":0`) {
		t.Fatalf("driver lease not at run/locks/driver.json with pid 0: %v %s", err, b)
	}
	if want := "held by s1 until " + now.Add(12*time.Hour).Format(time.RFC3339); run(t, f)["driver"].Detail != want {
		t.Fatalf("driver detail = %q, want %q (a 12 h lease)", run(t, f)["driver"].Detail, want)
	}
	capText, err := os.ReadFile(f.d.Run("cap"))
	if err != nil || strings.TrimSpace(string(capText)) != strconv.Itoa(max(1, runtime.NumCPU()-2)) {
		t.Fatalf("run/cap = %q, %v", capText, err)
	}
	// The same holder passes again: the lease is this session's.
	if c := run(t, f)["driver"]; !c.OK {
		t.Fatalf("re-run by the holder: %s", c.Detail)
	}
}

func TestPreflightFailures(t *testing.T) {
	cases := []struct {
		check   string
		detail  string
		breakIt func(t *testing.T, f pfFixture)
	}{
		{"repo", "is not the repo root", func(t *testing.T, f pfFixture) { t.Chdir(f.d.Root) }},
		{"repo", "does not match repo.remote", func(t *testing.T, f pfFixture) {
			testkit.Git(t, f.repo, "remote", "set-url", "origin", "https://github.com/acme/widget-old.git")
		}},
		{"driver", "held by another session", func(t *testing.T, f pfFixture) {
			lease := fmt.Sprintf(`{"name":"driver","holder":"other","pid":1,"host":"h","expires":%q}`, now.Add(time.Hour).Format(time.RFC3339))
			if err := os.MkdirAll(f.d.Run("locks"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(f.d.Run("locks", "driver.json"), []byte(lease), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"tools", "not on PATH: node", func(t *testing.T, f pfFixture) {
			if err := os.Remove(filepath.Join(f.bin, "node")); err != nil {
				t.Fatal(err)
			}
		}},
		{"gh", `logged in as "someone"`, func(t *testing.T, f pfFixture) { testkit.Answer(t, f.gh, "login", "someone\n") }},
		{"clean checkout", "uncommitted changes: M README.md", func(t *testing.T, f pfFixture) {
			if err := os.WriteFile(filepath.Join(f.repo, "README.md"), []byte("changed\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"integration", "neither gang-demo nor base trunk exists", func(t *testing.T, f pfFixture) {
			testkit.Git(t, f.repo, "update-ref", "-d", "refs/heads/gang-demo")
			f.p.Repo.Base = "trunk"
		}},
		{"ignored", ".planning/gang is not git-ignored", func(t *testing.T, f pfFixture) {
			// Ignoring run/ alone is not enough: plan/ must stay out of git too.
			if err := os.WriteFile(filepath.Join(f.repo, ".gitignore"), []byte(".planning/gang/*/run/\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			testkit.Git(t, f.repo, "commit", "-q", "-am", "narrow ignore")
		}},
		{"plan", "lint problems", func(t *testing.T, f pfFixture) {
			f.p.Tasks[0].Goal = ""
			commitPlan(t, f.repo, f.d, f.p)
		}},
		{"plan", "differs from refs/gang/demo/plan", func(t *testing.T, f pfFixture) {
			f.p.Done = "edited after commit"
			if err := plan.Save(f.d.Plan(), f.p); err != nil {
				t.Fatal(err)
			}
		}},
		{"disk", "1 GB free, need 10", func(t *testing.T, f pfFixture) {
			freeBytes = func(string) (uint64, error) { return 1 << 30, nil }
		}},
		{"worktrees", "1 worktrees, limit 1", func(t *testing.T, f pfFixture) { f.p.Limits.MaxWorktrees = 1 }},
	}
	for _, c := range cases {
		t.Run(c.check+"/"+c.detail, func(t *testing.T) {
			f := newPreflight(t)
			c.breakIt(t, f)
			got := run(t, f)[c.check]
			if got.OK || !strings.Contains(got.Detail, c.detail) {
				t.Fatalf("%s = %+v, want a failure containing %q", c.check, got, c.detail)
			}
		})
	}
}

func TestPreflightCreatesIntegrationFromBase(t *testing.T) {
	f := newPreflight(t)
	testkit.Git(t, f.repo, "update-ref", "-d", "refs/heads/gang-demo")
	m := run(t, f)
	if c := m["integration"]; !c.OK || c.Detail != "created gang-demo from main" {
		t.Fatalf("integration = %+v", c)
	}
	if testkit.Git(t, f.repo, "rev-parse", "gang-demo") != testkit.Git(t, f.repo, "rev-parse", "main") {
		t.Fatal("gang-demo not created at main")
	}
}

// gang merge moves the integration ref with update-ref, so a checkout of that
// branch, main or linked, would go stale.
func TestPreflightFailsWhenIntegrationIsCheckedOut(t *testing.T) {
	f := newPreflight(t)
	testkit.Git(t, f.repo, "switch", "-q", "gang-demo")
	want := "gang-demo is checked out in " + f.repo + ": switch that checkout to another branch"
	if c := run(t, f)["integration"]; c.OK || c.Detail != want {
		t.Fatalf("integration = %+v, want FAIL %q", c, want)
	}

	testkit.Git(t, f.repo, "switch", "-q", "main")
	linked, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	linked = filepath.Join(linked, "wt")
	testkit.Git(t, f.repo, "worktree", "add", "-q", linked, "gang-demo")
	want = "gang-demo is checked out in " + linked + ": switch that checkout to another branch"
	if c := run(t, f)["integration"]; c.OK || c.Detail != want {
		t.Fatalf("integration = %+v, want FAIL %q", c, want)
	}
}

// Contract 0.9: no account means no gh at all, and a local-path origin.
func TestPreflightWithoutGitHub(t *testing.T) {
	f := newPreflight(t)
	if err := os.Remove(filepath.Join(f.bin, "gh")); err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(t.TempDir(), "origin.git")
	testkit.Git(t, f.repo, "remote", "set-url", "origin", bare)
	f.p.Repo.GHAccount = ""
	f.p.Repo.Remote = bare
	m := run(t, f)
	if c := m["gh"]; !c.OK || !c.Skip {
		t.Fatalf("gh = %+v, want a skip", c)
	}
	for _, name := range []string{"repo", "tools"} {
		if !m[name].OK {
			t.Errorf("%s failed: %s", name, m[name].Detail)
		}
	}
}

func TestRemoteMatches(t *testing.T) {
	for _, origin := range []string{
		"github.com/acme/widget",
		"https://github.com/acme/widget.git",
		"https://user@github.com/acme/widget/",
		"git@github.com:acme/widget.git",
		"ssh://git@github.com:22/acme/widget.git",
	} {
		if !remoteMatches("/repo", origin, "github.com/acme/widget") {
			t.Errorf("%q must match github.com/acme/widget", origin)
		}
	}
	for _, c := range []struct{ origin, remote string }{
		{"git@github.com:acme/widget-old.git", "github.com/acme/widget"},
		{"https://github.com/acme/widget-old", "acme/widget"},
		{"https://github.com/acme/widget.git", "acme/widget"},
		{"git@github.com:acme/other.git", "github.com/acme/widget"},
	} {
		if remoteMatches("/repo", c.origin, c.remote) {
			t.Errorf("%q must not match %q", c.origin, c.remote)
		}
	}
	if !remoteMatches("/work/repo", "/srv/git/widget.git", "/srv/git/widget.git") || !remoteMatches("/work/repo", "../origin.git", "/work/origin.git") {
		t.Error("a local path compares as an absolute path")
	}
	if remoteMatches("/work/repo", "/srv/git/widget-old.git", "/srv/git/widget.git") {
		t.Error("a different local path must not match")
	}
}
