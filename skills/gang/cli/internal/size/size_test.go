package size

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/testkit"
)

func TestCountAddsAndDeletes(t *testing.T) {
	repo := testkit.Repo(t)
	base := testkit.CommitOn(t, repo, "main", "base", map[string]string{"a.txt": "1\n2\n3\n", "b.txt": "x\ny\n"})
	tip := testkit.CommitOn(t, repo, "task", "change", map[string]string{"a.txt": "1\nX\n3\n4\n5\n", "b.txt": ""})
	// a.txt: 3 added, 1 deleted. b.txt: 2 deleted. Total 6.
	if n, err := Count(repo, base, tip, DefaultExclude); err != nil || n != 6 {
		t.Errorf("Count = %d, %v; want 6", n, err)
	}
}

func TestCountSkipsBinaryAndLockFiles(t *testing.T) {
	repo := testkit.Repo(t)
	base := testkit.Git(t, repo, "rev-parse", "HEAD")
	tip := testkit.CommitOn(t, repo, "task", "add", map[string]string{
		"src/main.go":        "package main\n\nfunc main() {}\n",
		"go.sum":             "a v1 h1:x\nb v1 h1:y\n",
		"web/pnpm-lock.yaml": "lockfileVersion: 9\n",
		"Cargo.lock":         "[[package]]\n",
		"logo.png":           "\x89PNG\x00\x01\x02\n",
	})
	if n, err := Count(repo, base, tip, DefaultExclude); err != nil || n != 3 {
		t.Errorf("Count = %d, %v; want 3, from src/main.go only", n, err)
	}
}

func TestCountRenameIsSmall(t *testing.T) {
	repo := testkit.Repo(t)
	base := testkit.CommitOn(t, repo, "main", "add", map[string]string{"old.txt": strings.Repeat("line\n", 50)})
	testkit.Git(t, repo, "reset", "-q", "--hard", base)
	testkit.Git(t, repo, "mv", "old.txt", "new.txt")
	testkit.Git(t, repo, "commit", "-q", "-m", "rename")
	tip := testkit.Git(t, repo, "rev-parse", "HEAD")
	if n, err := Count(repo, base, tip, DefaultExclude); err != nil || n != 0 {
		t.Errorf("Count = %d, %v; want 0 for a pure rename", n, err)
	}
}

func TestCountTakesRevisionsLiterally(t *testing.T) {
	repo := testkit.Repo(t)
	base := testkit.Git(t, repo, "rev-parse", "HEAD")
	testkit.CommitOn(t, repo, "task", "add", map[string]string{"x.txt": "1\n"})
	// A work tree file named like a branch must not make git read the branch as a path.
	if err := os.WriteFile(filepath.Join(repo, "task"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := Count(repo, base, "task", nil); err != nil || n != 1 {
		t.Errorf("Count with a file named task = %d, %v; want 1", n, err)
	}
	// A revision that starts with a dash must not reach git as an option.
	out := filepath.Join(t.TempDir(), "out.txt")
	if _, err := Count(repo, "--output="+out, base, nil); err == nil {
		t.Error("a revision that looks like an option: no error")
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("git wrote %s: it read the revision as an option", out)
	}
}

func TestCountNumstat(t *testing.T) {
	for _, c := range []struct {
		name, out string
		want      int
		wantErr   bool
	}{
		{"an empty diff", "", 0, false},
		{"the final NUL is missing", "1\t2\ta.txt", 3, false},
		{"a path with a tab", "1\t0\ttab\there.txt\x00", 1, false},
		{"a rename, then a record", "1\t1\t\x00old.txt\x00new.txt\x00" + "2\t0\tb.txt\x00", 4, false},
		{"a binary rename, then a record", "-\t-\t\x00old.bin\x00new.bin\x00" + "2\t0\tb.txt\x00", 2, false},
		{"a rename to an excluded name, then a record", "5\t5\t\x00a.txt\x00a.lock\x00" + "1\t0\tb.txt\x00", 1, false},
		{"a rename that stops after the old path", "1\t1\t\x00old.txt\x00", 0, true},
		{"a record with no counts", "a.txt\x00", 0, true},
		{"counts that are not numbers", "x\ty\ta.txt\x00", 0, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			n, err := countNumstat(c.out, DefaultExclude)
			if (err != nil) != c.wantErr || n != c.want {
				t.Errorf("countNumstat(%q) = %d, %v; want %d, error %v", c.out, n, err, c.want, c.wantErr)
			}
		})
	}
}

func TestExcluded(t *testing.T) {
	for p, want := range map[string]bool{"go.sum": true, "tools/go.sum": true, "Cargo.lock": true,
		"web/package-lock.json": true, "main.go": false, "lock.go": false} {
		if got := Excluded(p, DefaultExclude); got != want {
			t.Errorf("Excluded(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestExcludedMatchesTheWholePath(t *testing.T) {
	// The first glob is malformed: it matches nothing and must not hide the second.
	globs := []string{"[", "gen/*.go"}
	for p, want := range map[string]bool{
		"gen/api.go": true, "src/gen/api.go": false, "gen/deep/api.go": false, "api.go": false} {
		if got := Excluded(p, globs); got != want {
			t.Errorf("Excluded(%q, %q) = %v, want %v", p, globs, got, want)
		}
	}
}
