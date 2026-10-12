package main

import (
	"fmt"
	"slices"

	"gang/internal/exit"
	"gang/internal/gitx"
)

func init() { commands["done"] = cmdDone }

// cmdDone prints the ids of the whole-line Task: <id> commits reachable from
// the integration branch (Contract 0 rule 1), sorted, one per line. A
// missing branch means none.
func cmdDone(c *Ctx, args []string) error {
	if len(args) != 0 {
		return exit.New(exit.Usage, "usage: gang done")
	}
	_, p, err := openRun(c)
	if err != nil {
		return err
	}
	ids, err := gitx.TaskIDs(c.Repo, "refs/heads/"+p.Repo.Integration)
	if err != nil {
		return err
	}
	ids = slices.Clone(ids)
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		fmt.Fprintln(c.Stdout, id)
	}
	return nil
}
