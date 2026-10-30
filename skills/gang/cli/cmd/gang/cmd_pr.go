package main

import (
	"fmt"

	"gang/internal/prq"
)

// prSubs holds the gang pr subcommands. Each later verb adds itself from its own file.
var prSubs = map[string]func(c *Ctx, args []string) error{"open": cmdPROpen}

func init() { commands["pr"] = subcommands("pr", prSubs) }

func cmdPROpen(c *Ctx, args []string) error {
	fs := newFlagSet("pr open")
	holder := fs.String("holder", "", "the driver lease holder")
	dry := fs.Bool("dry-run", false, "print the push and the PR instead of opening it")
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
	out, err := prq.Publish(c.Repo, d, p, pos[0], c.Now(), *dry)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, out)
	return nil
}
