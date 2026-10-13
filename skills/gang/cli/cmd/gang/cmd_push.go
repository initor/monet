package main

import (
	"fmt"

	"gang/internal/wave"
)

func init() { commands["push"] = cmdPush }

// cmdPush is gang push [--dry-run].
func cmdPush(c *Ctx, args []string) error {
	fs := newFlagSet("push")
	dry := fs.Bool("dry-run", false, "scan, then print the push command instead of pushing")
	if _, err := owArgs(fs, args, 0); err != nil {
		return err
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	out, err := wave.Push(c.Repo, d, p, *dry)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, out)
	return nil
}
