package main

import (
	"fmt"
	"gang/internal/prq"
)

func init() { prSubs["fix"] = cmdPRFix }

func cmdPRFix(c *Ctx, args []string) error {
	fs := newFlagSet("pr fix")
	holder := fs.String("holder", "", "the driver lease holder")
	reason := fs.String("reason", "", "why the PR may pass limits.task_lines, recorded as a ruling")
	pos, err := owArgs(fs, args, 2)
	if err != nil {
		return err
	}
	if err := requireDriver(c, *holder); err != nil {
		return err
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	rows, _ := prq.Read(d) // a read error fails Fix too, with no pushed row
	old, _ := prq.Current(rows)
	r, err := prq.Fix(c.Repo, d, p, pos[0], pos[1], *reason)
	if err != nil {
		if r.HeadSHA != "" && r.HeadSHA != old.HeadSHA {
			return fmt.Errorf("pushed %.7s to %s, but prs.tsv still holds %.7s: %w", r.HeadSHA, r.Branch, old.HeadSHA, err)
		}
		return err
	}
	fmt.Fprintf(c.Stdout, "PR #%d head %.7s, fix %d of 2\n", r.PR, r.HeadSHA, r.Fixes)
	if *reason != "" {
		fmt.Fprintf(c.Stdout, "reason: %s\n", *reason)
	}
	return nil
}
