package main

import (
	"fmt"

	"gang/internal/gitx"
	"gang/internal/prq"
)

func init() { prSubs["merge"] = cmdPRMerge }

// cmdPRMerge fetches origin first: prq.Merge reads the local origin ref, and a
// stale one makes it refuse the PR (exit 2), which is a refusal and not a retry.
func cmdPRMerge(c *Ctx, args []string) error {
	fs := newFlagSet("pr merge")
	holder := fs.String("holder", "", "the driver lease holder")
	pos, err := owArgs(fs, args, 1)
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
	if _, err := gitx.Run(c.Repo, "fetch", "--quiet", "origin"); err != nil {
		return fmt.Errorf("fetch origin: %w", err)
	}
	subject, err := prq.Merge(c.Repo, d, p, pos[0])
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, subject)
	if _, err := gitx.Run(c.Repo, "fetch", "--quiet", "origin"); err != nil { // the merge is done: a failed fetch only warns
		fmt.Fprintf(c.Stderr, "warning: fetch origin after the merge: %v\n", err)
	}
	return nil
}
