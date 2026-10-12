package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gang/internal/gitx"
)

func init() { commands["worktrees"] = cmdWorktrees }

var shellSafeRe = regexp.MustCompile(`^[A-Za-z0-9_./~@%+=:,-]+$`)

// cmdWorktrees prints a remove command for each task worktree under
// <repo>/.claude/worktrees/ that is clean, unlocked, and merged into the
// integration branch. It never removes anything itself.
func cmdWorktrees(c *Ctx, args []string) error {
	if _, err := owArgs(newFlagSet("worktrees"), args, 0); err != nil {
		return err
	}
	_, p, err := openRun(c)
	if err != nil {
		return err
	}
	root, err := filepath.EvalSymlinks(filepath.Join(c.Repo, ".claude", "worktrees"))
	if err != nil {
		return nil // no task worktrees yet
	}
	out, err := gitx.Run(c.Repo, "worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	integ := "refs/heads/" + p.Repo.Integration
	for _, wt := range parseWorktrees(out) {
		if wt.locked || wt.head == "" {
			continue
		}
		real, err := filepath.EvalSymlinks(wt.path)
		if err != nil || !strings.HasPrefix(real, root+string(filepath.Separator)) {
			continue
		}
		status, err := gitx.Run(wt.path, "status", "--porcelain")
		if err != nil || strings.TrimSpace(status) != "" {
			continue
		}
		if merged, err := gitx.IsAncestor(c.Repo, wt.head, integ); err != nil || !merged {
			continue
		}
		fmt.Fprintf(c.Stdout, "git -C %s worktree remove %s\n", shellQuote(c.Repo), shellQuote(wt.path))
	}
	return nil
}

type worktree struct {
	path, head string
	locked     bool
}

// parseWorktrees reads git worktree list --porcelain.
func parseWorktrees(out string) []worktree {
	var wts []worktree
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "worktree "):
			wts = append(wts, worktree{path: strings.TrimPrefix(l, "worktree ")})
		case len(wts) == 0:
		case strings.HasPrefix(l, "HEAD "):
			wts[len(wts)-1].head = strings.TrimPrefix(l, "HEAD ")
		case l == "locked" || strings.HasPrefix(l, "locked "):
			wts[len(wts)-1].locked = true
		}
	}
	return wts
}

func shellQuote(s string) string {
	if shellSafeRe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
