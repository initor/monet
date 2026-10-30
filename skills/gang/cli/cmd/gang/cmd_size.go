package main

import (
	"fmt"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/repair"
	"gang/internal/size"
)

func init() { commands["size"] = cmdSize }

// cmdSize prints how many lines the task tip (or --sha) would change on the
// integration branch. It exits 2 over budget or without a tip, and 3 on a conflict.
func cmdSize(c *Ctx, args []string) error {
	fs := newFlagSet("size")
	sha := fs.String("sha", "", "measure this commit, not the task branch tip")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return exit.New(exit.Usage, "usage: gang size <id> [--sha <sha>]")
	}
	_, p, err := openRun(c)
	if err != nil {
		return err
	}
	t, ok := repair.TaskFor(p, pos[0])
	if !ok {
		return exit.New(exit.User, "unknown task %s", pos[0])
	}
	tip, err := gitx.RevParse(c.Repo, firstSet(*sha, "refs/heads/"+plan.BranchFor(t)))
	if err != nil {
		return exit.New(exit.Refused, "size %s: %v", t.ID, err)
	}
	trunk, err := gitx.RevParse(c.Repo, "refs/heads/"+p.Repo.Integration)
	if err != nil {
		return err
	}
	tree, conflicts, err := gitx.MergeTree(c.Repo, trunk, tip)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return exit.New(exit.Conflict, "size %s: the tip conflicts with %s in %v", t.ID, p.Repo.Integration, conflicts)
	}
	n, msg, err := size.Measure(c.Repo, trunk, tree, p.Limits, t.Size.Reason)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "%s: %d changed lines (limit %d, ceiling %d)\n", t.ID, n, p.Limits.TaskLines, p.Limits.SplitLines)
	if msg != "" {
		return exit.New(exit.Refused, "%s", msg)
	}
	return nil
}
