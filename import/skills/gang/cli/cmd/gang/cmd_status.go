package main

import (
	"fmt"

	"gang/internal/status"
)

func init() { commands["status"] = cmdStatus }

// cmdStatus is gang status: writes run/status.md and prints at most 5 lines.
func cmdStatus(c *Ctx, args []string) error {
	if _, err := owArgs(newFlagSet("status"), args, 0); err != nil {
		return err
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	lines, err := status.Write(c.Repo, d, p, c.Now())
	if err != nil {
		return err
	}
	for _, l := range lines {
		fmt.Fprintln(c.Stdout, l)
	}
	return nil
}
