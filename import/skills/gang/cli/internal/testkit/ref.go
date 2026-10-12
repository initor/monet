package testkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// CommitRef commits files at the tree root of any ref, without touching the
// checkout. A missing ref starts from an empty tree with no parent.
func CommitRef(t *testing.T, repo, ref, msg string, files map[string]string) string {
	t.Helper()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	args := []string{"commit-tree"}
	if exec.Command("git", "-C", repo, "rev-parse", "--verify", "--quiet", ref).Run() == nil {
		gitEnv(t, repo, env, "read-tree", ref)
		args = append(args, "-p", ref)
	}
	for name, body := range files {
		blob := filepath.Join(t.TempDir(), "blob")
		if err := os.WriteFile(blob, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		sha := Git(t, repo, "hash-object", "-w", blob)
		gitEnv(t, repo, env, "update-index", "--add", "--cacheinfo", "100644,"+sha+","+name)
	}
	tree := gitEnv(t, repo, env, "write-tree")
	sha := Git(t, repo, append(args, tree, "-m", msg)...)
	Git(t, repo, "update-ref", ref, sha)
	return sha
}
