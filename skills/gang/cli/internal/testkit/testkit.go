// Package testkit holds fixtures shared by the gang tests. Only _test.go files import it.
package testkit

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/plan"
	"gang/internal/store"
)

// Git runs git -C dir and returns trimmed stdout. It fails the test on error.
func Git(t testing.TB, dir string, args ...string) string {
	t.Helper()
	return gitEnv(t, dir, nil, args...)
}

func gitEnv(t testing.TB, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

// Repo makes a git repo on branch main with one commit, under a temp dir with
// symlinks resolved. It isolates git from the user's config, including the
// default excludes file (~/.config/git/ignore), which may ignore .planning/.
func Repo(t *testing.T) string {
	t.Helper()
	global := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(global, []byte("[core]\n\texcludesFile = /dev/null\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL": global, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME": "gang test", "GIT_AUTHOR_EMAIL": "gang@example.com",
		"GIT_COMMITTER_NAME": "gang test", "GIT_COMMITTER_EMAIL": "gang@example.com",
	} {
		t.Setenv(k, v)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	Git(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	Git(t, dir, "add", "README.md")
	Git(t, dir, "commit", "-q", "-m", "init")
	return dir
}

// CommitOn commits files onto branch without touching the checkout, and
// returns the new SHA. A missing branch starts from main.
func CommitOn(t *testing.T, repo, branch, msg string, files map[string]string) string {
	t.Helper()
	parent := "refs/heads/" + branch
	if exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", parent).Run() != nil {
		parent = "refs/heads/main"
	}
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	gitEnv(t, repo, env, "read-tree", parent)
	for name, body := range files {
		blob := filepath.Join(t.TempDir(), "blob")
		if err := os.WriteFile(blob, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		sha := Git(t, repo, "hash-object", "-w", blob)
		gitEnv(t, repo, env, "update-index", "--add", "--cacheinfo", "100644,"+sha+","+name)
	}
	tree := gitEnv(t, repo, env, "write-tree")
	sha := Git(t, repo, "commit-tree", tree, "-p", parent, "-m", msg)
	Git(t, repo, "update-ref", "refs/heads/"+branch, sha)
	return sha
}

// Plan is a two-task plan that lints clean: T1 (wave 1) and T2 (wave 2, after T1).
func Plan() *plan.Plan {
	p := &plan.Plan{
		Version:      1,
		Slug:         "demo",
		Repo:         plan.Repo{Remote: "github.com/acme/widget", Base: "main", Integration: "gang-demo", GHAccount: "acme-bot"},
		Commands:     plan.Commands{Gate: []string{"true"}, Health: []string{"true"}, WaveGate: []string{"true"}},
		Forbidden:    []string{"git stash"},
		Done:         "every task is reachable from origin/main",
		Requirements: []plan.Requirement{{ID: "R1", Source: "spec.md:1"}, {ID: "R2", Source: "spec.md:2"}},
		Tasks: []plan.Task{
			task("T1", 1, nil, "R1", "a"),
			task("T2", 2, []string{"T1"}, "R2", "b"),
		},
	}
	p.Normalize()
	return p
}

func task(id string, wave int, deps []string, req, dir string) plan.Task {
	return plan.Task{
		ID:           id,
		Title:        "Build " + id,
		Goal:         "Make " + id + " pass its test.",
		Scope:        plan.Scope{Write: []string{dir + "/**"}},
		DependsOn:    deps,
		Requirements: []string{req},
		Interfaces:   plan.Interfaces{Produces: []string{"file:" + dir + "/" + id + ".go"}},
		Acceptance:   []string{id + " works"},
		Verify:       plan.Verify{TestsFirst: []string{"test:" + id}, Commands: []string{"true"}},
		Size:         plan.Size{Lines: 100},
		TimeboxMin:   30,
		Forbidden:    []string{"git rebase"},
		Wave:         wave,
	}
}

// GangDir writes p to <repo>/.planning/gang/<slug>/plan/plan.json and makes run/.
func GangDir(t *testing.T, repo string, p *plan.Plan) store.Dir {
	t.Helper()
	d := store.Dir{Root: filepath.Join(repo, ".planning", "gang", p.Slug)}
	for _, dir := range []string{filepath.Dir(d.Plan()), d.Run()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := plan.Save(d.Plan(), p); err != nil {
		t.Fatal(err)
	}
	return d
}
