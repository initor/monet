package main

import (
	"fmt"

	"gang/internal/exit"
	"gang/internal/lint"
	"gang/internal/plan"
	"gang/internal/store"
)

func init() { commands["lint"] = cmdLint }

// cmdLint prints one problem per line and exits 1 when there is any. A plan
// that does not decode is a rule 1 problem.
func cmdLint(c *Ctx, args []string) error {
	fs := newFlagSet("lint")
	adopt := fs.String("adopt-source", "", "source plan to cross-check task ids and edges against")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return exit.New(exit.Usage, "lint takes no arguments")
	}
	var probs []lint.Problem
	p, err := plan.Load(store.Dir{Root: c.Dir}.Plan())
	if err != nil {
		probs = []lint.Problem{{Rule: 1, Task: "-", Msg: err.Error()}}
	} else {
		probs = lint.Lint(p, lint.Options{AdoptSource: *adopt})
	}
	for _, pr := range probs {
		fmt.Fprintln(c.Stdout, pr.String())
	}
	if len(probs) > 0 {
		return exit.New(exit.User, "%d lint problems", len(probs))
	}
	return nil
}
