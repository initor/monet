package testkit

import (
	"path/filepath"
	"testing"
)

// Origin makes a bare repo, adds it to repo as origin, and pushes main.
func Origin(t *testing.T, repo string) string {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "origin.git")
	Git(t, repo, "init", "-q", "--bare", bare)
	Git(t, repo, "remote", "add", "origin", bare)
	Git(t, repo, "push", "-q", "origin", "main")
	Git(t, repo, "fetch", "-q", "origin")
	return bare
}
