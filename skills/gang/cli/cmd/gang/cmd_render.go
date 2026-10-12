package main

import (
	"fmt"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/render"
	"gang/internal/schedule"
	"gang/internal/store"
)

func init() { commands["render"] = cmdRender }

// cmdRender writes plan/plan.md and prints its path.
func cmdRender(c *Ctx, args []string) error {
	pos, err := parseFlags(newFlagSet("render"), args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return exit.New(exit.Usage, "render takes no arguments")
	}
	p, err := loadPlan(c)
	if err != nil {
		return err
	}
	path, err := writePlanMD(c, p)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, path)
	return nil
}

// writePlanMD renders p into plan/plan.md with the default schedule (cap 4)
// and returns the path. A graph that cannot be scheduled still renders,
// without the chain and the range, and the reason goes to stderr.
func writePlanMD(c *Ctx, p *plan.Plan) (string, error) {
	s, err := schedule.Run(p, schedule.Options{})
	if err != nil {
		fmt.Fprintf(c.Stderr, "gang: render without schedule: %v\n", err)
		s = nil
	}
	path := store.Dir{Root: c.Dir}.PlanMD()
	if err := store.WriteFileAtomic(path, []byte(render.Markdown(p, s)), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
