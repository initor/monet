package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/repair"
)

func init() {
	commands["claim"] = cmdClaim
	commands["release"] = cmdRelease
}

// worktreeTop returns the real path of the linked worktree that holds the
// cwd. It refuses with NO-WORKTREE (exit 2) unless that worktree belongs to
// c.Repo and sits under <repo>/.claude/worktrees/. Commands that switch
// HEAD call it first, so they never move the main checkout.
func worktreeTop(c *Ctx) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	refuse := exit.New(exit.Refused, "NO-WORKTREE: %s is not in a linked worktree under %s", cwd, filepath.Join(c.Repo, ".claude", "worktrees"))
	top, err := gitx.Run(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", refuse
	}
	realTop, err := filepath.EvalSymlinks(strings.TrimSpace(top))
	if err != nil {
		return "", err
	}
	realRepo, err := filepath.EvalSymlinks(c.Repo)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(realTop, filepath.Join(realRepo, ".claude", "worktrees")+string(filepath.Separator)) {
		return "", refuse
	}
	owner, err := gitx.MainRepo(realTop)
	if err != nil {
		return "", refuse
	}
	if realOwner, err := filepath.EvalSymlinks(owner); err != nil || realOwner != realRepo {
		return "", refuse
	}
	return realTop, nil
}

// checkedOutElsewhere returns the worktree other than top that has ref
// checked out, or "".
func checkedOutElsewhere(top, ref string) (string, error) {
	out, err := gitx.Run(top, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	var path string
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			path = p
			continue
		}
		if line == "branch "+ref {
			if rp, err := filepath.EvalSymlinks(path); err != nil || rp != top {
				return path, nil
			}
		}
	}
	return "", nil
}

// writeOnce writes data to path atomically, unless path already exists.
// os.Link fails when the target exists, so the first writer wins.
func writeOnce(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Link(f.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}

// cmdClaim puts the task branch in this worktree and prints its tip. An
// existing branch is switched to. A new one starts at the integration tip.
// The first claim writes run/tasks/<id>/claimed_at (Contract 0 rule 11).
func cmdClaim(c *Ctx, args []string) error {
	if len(args) != 1 {
		return exit.New(exit.Usage, "usage: gang claim <id>")
	}
	top, err := worktreeTop(c)
	if err != nil {
		return err
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	t, ok := repair.TaskFor(p, args[0])
	if !ok {
		return exit.New(exit.Refused, "unknown task %s", args[0])
	}
	branch := plan.BranchFor(t)
	ref := "refs/heads/" + branch
	have, err := gitx.Run(top, "for-each-ref", "--format=%(objectname)", ref)
	if err != nil {
		return err
	}
	if strings.TrimSpace(have) == "" {
		if _, err := gitx.Run(top, "switch", "-c", branch, "refs/heads/"+p.Repo.Integration); err != nil {
			return err
		}
	} else {
		other, err := checkedOutElsewhere(top, ref)
		if err != nil {
			return err
		}
		if other != "" {
			return exit.New(exit.Held, "%s is checked out in %s", branch, other)
		}
		cur, err := gitx.Run(top, "branch", "--show-current")
		if err != nil {
			return err
		}
		if strings.TrimSpace(cur) != branch {
			if _, err := gitx.Run(top, "switch", branch); err != nil {
				return err
			}
		}
	}
	stamp := c.Now().UTC().Format(time.RFC3339) + "\n"
	if err := writeOnce(d.Run("tasks", t.ID, "claimed_at"), []byte(stamp)); err != nil {
		return err
	}
	tip, err := gitx.RevParse(top, "HEAD")
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, strings.TrimSpace(tip))
	return nil
}

// cmdRelease detaches HEAD so another worktree can claim the branch.
func cmdRelease(c *Ctx, args []string) error {
	if len(args) != 0 {
		return exit.New(exit.Usage, "usage: gang release")
	}
	top, err := worktreeTop(c)
	if err != nil {
		return err
	}
	_, err = gitx.Run(top, "switch", "--detach")
	return err
}
