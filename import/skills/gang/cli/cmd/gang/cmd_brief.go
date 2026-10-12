package main

import (
	"fmt"

	"gang/internal/brief"
	"gang/internal/exit"
	"gang/internal/repair"
)

func init() { commands["brief"] = cmdBrief }

func cmdBrief(c *Ctx, args []string) error {
	if len(args) != 1 {
		return exit.New(exit.Usage, "usage: gang brief <id>")
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	if _, ok := repair.TaskFor(p, args[0]); !ok {
		return exit.New(exit.Refused, "unknown task %s", args[0])
	}
	text, err := brief.Render(d, p, args[0])
	if err != nil {
		return err
	}
	fmt.Fprint(c.Stdout, text)
	return nil
}
