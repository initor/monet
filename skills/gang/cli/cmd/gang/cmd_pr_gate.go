package main

import (
	"fmt"

	"gang/internal/exit"
	"gang/internal/gate"
	"gang/internal/gh"
	"gang/internal/plan"
	"gang/internal/prq"
)

func init() { prSubs["gate"] = cmdPRGate }
func cmdPRGate(c *Ctx, args []string) error {
	fs := newFlagSet("pr gate")
	holder := fs.String("holder", "", "the driver lease holder")
	pos, err := owArgs(fs, args, 2)
	if err == nil {
		err = requireDriver(c, *holder)
	}
	if err != nil {
		return err
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	if p.Repo.Checks != plan.ChecksGate {
		return exit.New(exit.Refused, "repo.checks is %s: gang posts gang/gate only for repo.checks gate", p.Repo.Checks)
	}
	r, _, err := prq.Live(d, p, pos[0])
	if err != nil {
		return err
	}
	state, err := gate.PRStatus(d, pos[1], r.Task, r.HeadSHA)
	if err == nil {
		err = gh.PostStatus(p.Repo.Remote, r.HeadSHA, state, "gang/gate", "gang gate job "+pos[1]+" "+state)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Stdout, "gang/gate %s on %.7s\n", state, r.HeadSHA)
	return nil
}
