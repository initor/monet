// Package brief renders a task's brief (Contract 5): the plan's slice for
// one task plus the run's standing orders, owner rulings, and prior report.
package brief

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gang/internal/plan"
	"gang/internal/repair"
	"gang/internal/store"
)

const reportLine = "Return the JSON object your prompt names. Write your report with gang report."

// Render returns the brief for task id, a plan task or a synthetic repair
// task (repair.TaskFor). When run/standing.md starts with "STOP:", that
// line is also the brief's first line.
func Render(d store.Dir, p *plan.Plan, id string) (string, error) {
	t, ok := repair.TaskFor(p, id)
	if !ok {
		return "", fmt.Errorf("unknown task %s", id)
	}
	standing, err := readOptional(d.Run("standing.md"))
	if err != nil {
		return "", err
	}
	owner, err := readOptional(d.Run("tasks", id, "owner.md"))
	if err != nil {
		return "", err
	}
	prior := d.Run("tasks", id, "report.md")
	if _, err := os.Stat(prior); errors.Is(err, os.ErrNotExist) {
		prior = "none"
	} else if err != nil {
		return "", err
	}

	sources := map[string]string{}
	for _, r := range p.Requirements {
		sources[r.ID] = r.Source
	}
	reqs := make([]string, len(t.Requirements))
	for i, r := range t.Requirements {
		src, ok := sources[r]
		if !ok {
			src = "source not in plan"
		}
		reqs[i] = fmt.Sprintf("%s (%s)", r, src)
	}

	var b strings.Builder
	if first, _, _ := strings.Cut(standing, "\n"); strings.HasPrefix(first, "STOP:") {
		b.WriteString(first + "\n")
	}
	fmt.Fprintf(&b, "TASK %s: %s\nbranch: %s\nwave: %d\n", t.ID, t.Title, plan.BranchFor(t), t.Wave)
	section(&b, "GOAL", t.Goal)
	section(&b, "SCOPE", list("write", t.Scope.Write)+list("shared", t.Scope.Shared)+list("peers", t.Peers))
	section(&b, "CONTEXT", list("depends_on", t.DependsOn)+list("merge_after", t.MergeAfter)+
		list("requirements", reqs)+list("consumes", t.Interfaces.Consumes)+list("produces", t.Interfaces.Produces))
	section(&b, "ACCEPTANCE", bullets(t.Acceptance))
	section(&b, "VERIFY", list("tests_first", t.Verify.TestsFirst)+list("commands", t.Verify.Commands)+list("gate", repair.GateFor(p, t)))
	section(&b, "TIMEBOX", fmt.Sprintf("lines: %d\n%s\nminutes: %d", t.Size.Lines, budget(p.Limits, t.Size.Reason), t.TimeboxMin))
	section(&b, "FORBIDDEN", bullets(dedupe(append(append([]string{}, p.Forbidden...), t.Forbidden...))))
	section(&b, "REPORT", reportLine)
	section(&b, "STANDING", orNone(standing))
	section(&b, "OWNER RULINGS", orNone(owner))
	section(&b, "PRIOR REPORT", prior)
	return b.String(), nil
}

// budget states how many lines the task may change. A reason that is blank
// once trimmed counts as none, the rule of size.Check.
func budget(l plan.Limits, reason string) string {
	const rest = ", tests included, lock and binary files not counted"
	if strings.TrimSpace(reason) == "" {
		return fmt.Sprintf("budget: %d changed lines%s", l.TaskLines, rest)
	}
	return fmt.Sprintf("budget: up to %d changed lines (size.reason: %s)%s", l.SplitLines, strings.Join(strings.Fields(reason), " "), rest)
}

func readOptional(path string) (string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// section writes a blank line, the heading, and the body with one final newline.
func section(b *strings.Builder, heading, body string) {
	fmt.Fprintf(b, "\n%s\n%s\n", heading, strings.TrimRight(body, "\n"))
}

// list renders "label: none" or "label:" followed by one "- item" per line.
func list(label string, items []string) string {
	if len(items) == 0 {
		return label + ": none\n"
	}
	return label + ":\n" + bullets(items) + "\n"
}

func bullets(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return "- " + strings.Join(items, "\n- ")
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "none"
	}
	return s
}

func dedupe(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}
