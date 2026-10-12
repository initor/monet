package gitx

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := Run(dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// newRepo makes a repo on branch main with one commit. The path has its
// symlinks resolved, because git reports real paths.
func newRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "init", "--quiet", "--initial-branch=main")
	mustRun(t, dir, "config", "user.name", "Gang Test")
	mustRun(t, dir, "config", "user.email", "gang-test@example.com")
	mustRun(t, dir, "config", "commit.gpgsign", "false")
	mustRun(t, dir, "config", "core.excludesFile", os.DevNull)
	commitFile(t, dir, "README.md", "test repo\n", "initial")
	return dir
}

// commitFile writes name, commits it on the current branch, and returns the SHA.
func commitFile(t *testing.T, repo, name, content, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, repo, "add", name)
	mustRun(t, repo, "commit", "--quiet", "--no-verify", "-m", msg)
	return mustRun(t, repo, "rev-parse", "HEAD")
}

func TestRunTrimsAndReportsStderr(t *testing.T) {
	repo := newRepo(t)
	out := mustRun(t, repo, "rev-parse", "--show-toplevel")
	if out != repo {
		t.Errorf("toplevel %q, want %q with no trailing newline", out, repo)
	}
	_, err := Run(repo, "rev-parse", "--verify", "no-such-rev")
	if err == nil || !strings.Contains(err.Error(), "Needed a single revision") {
		t.Errorf("error %v does not carry git's stderr", err)
	}
}

func TestRunIgnoresInheritedRepoVariables(t *testing.T) {
	repo := newRepo(t)
	other := newRepo(t)
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(other, ".git", "index"))
	if out := mustRun(t, repo, "rev-parse", "--absolute-git-dir"); out != filepath.Join(repo, ".git") {
		t.Errorf("git used %q, want %q", out, filepath.Join(repo, ".git"))
	}
}

func TestRunEnvUsesATemporaryIndex(t *testing.T) {
	repo := newRepo(t)
	idx := filepath.Join(t.TempDir(), "index")
	env := []string{"GIT_INDEX_FILE=" + idx}
	if _, err := RunEnv(repo, env, "read-tree", "--empty"); err != nil {
		t.Fatal(err)
	}
	tree, err := RunEnv(repo, env, "write-tree")
	if err != nil {
		t.Fatal(err)
	}
	if empty := mustRun(t, repo, "hash-object", "-t", "tree", "/dev/null"); tree != empty {
		t.Errorf("temp index tree %s, want the empty tree %s", tree, empty)
	}
	if files := mustRun(t, repo, "ls-files"); files != "README.md" {
		t.Errorf("main index changed: ls-files %q", files)
	}
}

func TestRevParse(t *testing.T) {
	repo := newRepo(t)
	sha, err := RevParse(repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	if want := mustRun(t, repo, "rev-parse", "HEAD"); sha != want {
		t.Errorf("RevParse(main) = %s, want %s", sha, want)
	}
	if _, err := RevParse(repo, "refs/heads/nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing rev: err %v, want ErrNotFound", err)
	}
	if _, err := RevParse(t.TempDir(), "main"); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("not a repo: err %v, want a plain error", err)
	}
}

func TestIsAncestor(t *testing.T) {
	repo := newRepo(t)
	first := mustRun(t, repo, "rev-parse", "HEAD")
	second := commitFile(t, repo, "a.txt", "a\n", "second")
	if ok, err := IsAncestor(repo, first, second); err != nil || !ok {
		t.Errorf("first before second: %v %v, want true", ok, err)
	}
	if ok, err := IsAncestor(repo, second, first); err != nil || ok {
		t.Errorf("second before first: %v %v, want false", ok, err)
	}
	if _, err := IsAncestor(repo, "no-such-rev", first); err == nil {
		t.Error("unknown rev: no error")
	}
}

func TestMergeTree(t *testing.T) {
	repo := newRepo(t)
	base := mustRun(t, repo, "rev-parse", "HEAD")
	mustRun(t, repo, "switch", "--quiet", "-c", "side")
	side := commitFile(t, repo, "side.txt", "side\n", "side")
	commitFile(t, repo, "README.md", "side readme\n", "side readme")
	sideTip := mustRun(t, repo, "rev-parse", "HEAD")
	mustRun(t, repo, "switch", "--quiet", "main")
	mainTip := commitFile(t, repo, "main.txt", "main\n", "main")

	tree, conflicts, err := MergeTree(repo, mainTip, side)
	if err != nil || conflicts != nil {
		t.Fatalf("clean merge: conflicts %v err %v", conflicts, err)
	}
	if files := mustRun(t, repo, "ls-tree", "--name-only", tree); files != "README.md\nmain.txt\nside.txt" {
		t.Errorf("clean merge tree holds %q", files)
	}

	mustRun(t, repo, "switch", "--quiet", "-c", "rival", base)
	rival := commitFile(t, repo, "README.md", "rival readme\n", "rival readme")
	_, conflicts, err = MergeTree(repo, rival, sideTip)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(conflicts, []string{"README.md"}) {
		t.Errorf("conflicts %v, want [README.md]", conflicts)
	}
}

// A content conflict names its paths, not git's messages about them. g
// merges cleanly, so git prints "Auto-merging g" without listing g.
func TestMergeTreeContentConflictListsPathsOnly(t *testing.T) {
	repo := newRepo(t)
	commitFile(t, repo, "f", "base\n", "add f")
	commitFile(t, repo, "g", "1\n2\n3\n", "add g")
	mustRun(t, repo, "switch", "--quiet", "-c", "side")
	commitFile(t, repo, "f", "side\n", "side f")
	side := commitFile(t, repo, "g", "side\n2\n3\n", "side g")
	mustRun(t, repo, "switch", "--quiet", "main")
	commitFile(t, repo, "f", "main\n", "main f")
	mainTip := commitFile(t, repo, "g", "1\n2\nmain\n", "main g")

	_, conflicts, err := MergeTree(repo, mainTip, side)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(conflicts, []string{"f"}) {
		t.Errorf("conflicts %v, want [f]", conflicts)
	}
}

// man git-merge-tree: an empty conflicted-file list is not a clean merge,
// the exit status is. main splits dir/ into x/ and y/ while side adds dir/c,
// and git exits 1 without naming a path.
func TestMergeTreeConflictWithoutAPath(t *testing.T) {
	repo := newRepo(t)
	for _, d := range []string{"dir", "x", "y"} {
		if err := os.MkdirAll(filepath.Join(repo, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	commitFile(t, repo, "dir/a", "a\n", "add dir/a")
	commitFile(t, repo, "dir/b", "b\n", "add dir/b")
	mustRun(t, repo, "switch", "--quiet", "-c", "side")
	side := commitFile(t, repo, "dir/c", "c\n", "add dir/c")
	mustRun(t, repo, "switch", "--quiet", "main")
	mustRun(t, repo, "mv", "dir/a", "x/a")
	mustRun(t, repo, "mv", "dir/b", "y/b")
	mustRun(t, repo, "commit", "--quiet", "--no-verify", "-m", "split dir")
	mainTip := mustRun(t, repo, "rev-parse", "HEAD")

	out, code, err := git(repo, nil, []int{1}, "merge-tree", "--write-tree", "--name-only", "--no-messages", mainTip, side)
	if err != nil || code != 1 || strings.Contains(strings.TrimSpace(string(out)), "\n") {
		t.Fatalf("git no longer exits 1 with no path here (code %d, out %q, err %v), so this test pins nothing", code, out, err)
	}

	_, conflicts, err := MergeTree(repo, mainTip, side)
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 || !strings.HasPrefix(conflicts[0], "CONFLICT (directory rename split): ") {
		t.Errorf("conflicts %q, want the one directory rename split message", conflicts)
	}
}

func TestCommitTreeAndUpdateRefCAS(t *testing.T) {
	repo := newRepo(t)
	head := mustRun(t, repo, "rev-parse", "HEAD")
	tree := mustRun(t, repo, "rev-parse", "HEAD^{tree}")
	c1, err := CommitTree(repo, tree, []string{head}, "one\n\nTask: A1")
	if err != nil {
		t.Fatal(err)
	}
	if got := mustRun(t, repo, "rev-parse", c1+"^"); got != head {
		t.Errorf("parent %s, want %s", got, head)
	}
	if got := mustRun(t, repo, "log", "-1", "--format=%B", c1); got != "one\n\nTask: A1" {
		t.Errorf("message %q", got)
	}

	ref := "refs/heads/integration"
	if err := UpdateRefCAS(repo, ref, c1, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := UpdateRefCAS(repo, ref, head, ""); !errors.Is(err, ErrCASLost) {
		t.Errorf("create over an existing ref: err %v, want ErrCASLost", err)
	}
	c2, err := CommitTree(repo, tree, []string{c1}, "two")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateRefCAS(repo, ref, c2, head); !errors.Is(err, ErrCASLost) {
		t.Errorf("stale old value: err %v, want ErrCASLost", err)
	}
	if got := mustRun(t, repo, "rev-parse", ref); got != c1 {
		t.Errorf("a lost CAS moved the ref to %s", got)
	}
	if err := UpdateRefCAS(repo, ref, c2, c1); err != nil {
		t.Errorf("matching old value: %v", err)
	}
	if got := mustRun(t, repo, "rev-parse", ref); got != c2 {
		t.Errorf("ref at %s, want %s", got, c2)
	}
	if err := UpdateRefCAS(repo, "integration", c2, ""); err == nil || errors.Is(err, ErrCASLost) {
		t.Errorf("short ref name: err %v, want a plain error", err)
	}
	if err := UpdateRefCAS(repo, "refs/heads/gone", c2, c1); !errors.Is(err, ErrCASLost) {
		t.Errorf("expected ref that does not exist: err %v, want ErrCASLost", err)
	}
}

// The rule comes from Contract 0 item 1: a whole line "Task: <id>", trailer
// or not. potato writes it above a "Testing Done" paragraph, where git's
// trailer parser does not see it. A1 appears only in that form.
func TestTaskIDs(t *testing.T) {
	repo := newRepo(t)
	mustRun(t, repo, "switch", "--quiet", "-c", "work")
	commitFile(t, repo, "b.txt", "b\n", "add b\n\nTask: B2")
	commitFile(t, repo, "a.txt", "a\n", "add a\n\nTask: A1\n\nTesting Done:\n- go test ./...")
	commitFile(t, repo, "b.txt", "b2\n", "fix b\n\nTask: B2")
	commitFile(t, repo, "c.txt", "c\n", "mentions Task: C3 in the body\n\n  Task: D4\nTask: e5\nTask: F6 and more")
	if trailers := mustRun(t, repo, "log", "--format=%(trailers:key=Task,valueonly)", "-1", "HEAD~2"); trailers != "" {
		t.Fatalf("git parsed the potato-style line as a trailer (%q), so this test no longer pins the difference", trailers)
	}
	got, err := TaskIDs(repo, "work")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"A1", "B2"}) {
		t.Errorf("task ids %v, want [A1 B2]", got)
	}
	got, err = TaskIDs(repo, "no-such-branch")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("missing rev: %v %v, want an empty list and no error", got, err)
	}
}

func TestMainRepoFromWorktree(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(repo, ".claude", "worktrees", "w1")
	mustRun(t, repo, "worktree", "add", "--quiet", "-b", "w1", wt)
	sub := filepath.Join(wt, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{repo, wt, sub} {
		got, err := MainRepo(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got != repo {
			t.Errorf("MainRepo(%s) = %s, want %s", dir, got, repo)
		}
	}
}

func TestShowReturnsExactBytes(t *testing.T) {
	repo := newRepo(t)
	content := "line one\nline two\n\n"
	sha := commitFile(t, repo, "f.txt", content, "f")
	got, err := Show(repo, sha, "f.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("Show = %q, want %q", got, content)
	}
	if _, err := Show(repo, sha, "missing.txt"); err == nil {
		t.Error("missing path: no error")
	}
}
