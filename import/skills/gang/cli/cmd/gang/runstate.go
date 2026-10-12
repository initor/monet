package main

import (
	"gang/internal/plan"
	"gang/internal/store"
)

// openRun loads the plan with loadPlan (exit 1 on failure) and returns the
// gang dir beside it, for the run-state commands of plan 02.
func openRun(c *Ctx) (store.Dir, *plan.Plan, error) {
	p, err := loadPlan(c)
	return store.Dir{Root: c.Dir}, p, err
}
