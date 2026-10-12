// Package repair resolves task ids for every command that takes one. An id
// in plan.json is that task. An id matching ^F[0-9]{2,}$ that the plan lacks
// is a synthetic repair task the engine created (Contract 0 rule 16).
package repair

import (
	"regexp"

	"gang/internal/plan"
)

// Goal is the synthetic repair task's goal.
const Goal = "make every health command pass on the integration branch"

var idRE = regexp.MustCompile(`^F[0-9]{2,}$`)

// TaskFor returns the plan task with id, or a synthetic repair task for an
// F<nn> id missing from the plan. The synthetic task has branch gang/<id>,
// no deps, risk low, the whole repo as scope, and the run-wide locks that
// gate run --health also takes.
func TaskFor(p *plan.Plan, id string) (*plan.Task, bool) {
	if t, ok := p.Task(id); ok {
		return t, true
	}
	if !idRE.MatchString(id) {
		return nil, false
	}
	t := &plan.Task{
		ID:           id,
		Title:        "Repair the integration branch",
		Goal:         Goal,
		Scope:        plan.Scope{Write: []string{"**"}, Shared: []string{}},
		Peers:        []string{},
		DependsOn:    []string{},
		MergeAfter:   []string{},
		Requirements: []string{},
		Interfaces:   plan.Interfaces{Consumes: []string{}, Produces: []string{}},
		Acceptance:   []string{"Every command in plan.commands.health passes on the integration branch."},
		Verify:       plan.Verify{TestsFirst: []string{}, Commands: []string{}},
		Size:         plan.Size{Lines: p.Limits.TaskLines},
		TimeboxMin:   p.Limits.StallMin,
		Forbidden:    []string{},
		Locks:        append([]string{}, p.Locks...),
		Risk:         "low",
	}
	return t, true
}

// GateFor returns the run gate commands for t: plan.commands.health for a
// synthetic repair task, plan.commands.gate otherwise.
func GateFor(p *plan.Plan, t *plan.Task) []string {
	if _, inPlan := p.Task(t.ID); !inPlan && idRE.MatchString(t.ID) {
		return p.Commands.Health
	}
	return p.Commands.Gate
}
