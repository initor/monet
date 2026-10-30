// Package render turns a plan into plan.md, the human view of plan.json.
package render

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"gang/internal/plan"
	"gang/internal/schedule"
)

// Markdown renders p. The waves table groups tasks by their stored wave.
// When s is not nil, the critical chain and the wall-clock range follow it.
func Markdown(p *plan.Plan, s *schedule.Result) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# Plan: %s\n\n", p.Slug)
	w("Rendered from plan.json by `gang render`. Do not edit by hand.\n\n")

	w("## Run config\n\n| Field | Value |\n|---|---|\n")
	for _, row := range [][2]string{
		{"Remote", code(p.Repo.Remote)},
		{"Base", code(p.Repo.Base)},
		{"Integration", code(p.Repo.Integration)},
		{"GitHub account", code(p.Repo.GHAccount)},
		{"Wave title", code(p.Repo.WaveTitle)},
		{"Checks", code(p.Repo.Checks)},
		{"Gate", codes(p.Commands.Gate)},
		{"Health", codes(p.Commands.Health)},
		{"Wave gate", codes(p.Commands.WaveGate)},
		{"Env", env(p.Env)},
		{"Locks", codes(p.Locks)},
		{"Limits", limits(p.Limits)},
		{"Models", roles(p.Models)},
		{"Forbidden", codes(p.Forbidden)},
		{"Done", orNone(flat(p.Done))},
	} {
		w("| %s | %s |\n", row[0], cell(row[1]))
	}

	w("\n## Waves\n\n| Wave | Tasks |\n|---|---|\n")
	byWave := map[int][]string{}
	for _, t := range p.Tasks {
		byWave[t.Wave] = append(byWave[t.Wave], t.ID)
	}
	nums := make([]int, 0, len(byWave))
	for n := range byWave {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	for _, n := range nums {
		label := fmt.Sprint(n)
		if n == 0 {
			label = "unscheduled"
		}
		w("| %s | %s |\n", label, cell(strings.Join(byWave[n], ", ")))
	}
	if s != nil {
		w("\nCritical chain: %s\n\nEstimated range: %d-%d min\n",
			orNone(strings.Join(s.Chain, " > ")), int(math.Round(s.LowMin)), int(math.Round(s.HighMin)))
	}

	sources := map[string]string{}
	for _, r := range p.Requirements {
		sources[r.ID] = r.Source
	}
	for _, t := range p.Tasks {
		w("\n## %s: %s\n\n", t.ID, flat(t.Title))
		w("- Goal: %s\n", flat(t.Goal))
		w("- Branch: %s\n", code(plan.BranchFor(&t)))
		w("- Wave: %d\n", t.Wave)
		w("- Risk: %s\n", t.Risk)
		size := fmt.Sprintf("%d lines", t.Size.Lines)
		if r := strings.Join(strings.Fields(t.Size.Reason), " "); r != "" {
			size += fmt.Sprintf(" (size.reason: %s)", r)
		}
		w("- Size: %s, timebox %d min\n", size, t.TimeboxMin)
		w("- Scope write: %s\n", codes(t.Scope.Write))
		w("- Scope shared: %s\n", codes(t.Scope.Shared))
		w("- Peers: %s\n", ids(t.Peers))
		w("- Depends on: %s\n", ids(t.DependsOn))
		w("- Merge after: %s\n", ids(t.MergeAfter))
		reqs := make([]string, 0, len(t.Requirements))
		for _, r := range t.Requirements {
			src := "unknown source"
			if v, ok := sources[r]; ok {
				src = code(v)
			}
			reqs = append(reqs, fmt.Sprintf("%s (%s)", r, src))
		}
		w("- Requirements: %s\n", orNone(strings.Join(reqs, ", ")))
		w("- Consumes: %s\n", codes(t.Interfaces.Consumes))
		w("- Produces: %s\n", codes(t.Interfaces.Produces))
		w("- Verify commands: %s\n", codes(t.Verify.Commands))
		w("- Forbidden: %s\n", codes(t.Forbidden))
		w("- Locks: %s\n", codes(t.Locks))
		w("- Model: %s\n", roles(t.Model))
		if t.Notes != "" {
			w("- Notes: %s\n", flat(t.Notes))
		}
		w("\nAcceptance:\n\n")
		for _, a := range t.Acceptance {
			w("- %s\n", flat(a))
		}
		w("\nTests first:\n\n")
		for _, tf := range t.Verify.TestsFirst {
			w("- %s\n", code(tf))
		}
	}
	return b.String()
}

// flat puts s on one line.
func flat(s string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
}

// cell escapes a table cell. GFM needs \| even inside a code span.
func cell(s string) string { return strings.ReplaceAll(flat(s), "|", `\|`) }

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// code wraps s in a code span, with double backticks when s holds one.
func code(s string) string {
	s = flat(s)
	switch {
	case s == "":
		return "none"
	case strings.Contains(s, "`"):
		return "`` " + s + " ``"
	default:
		return "`" + s + "`"
	}
}

func codes(vals []string) string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = code(v)
	}
	return orNone(strings.Join(out, ", "))
}

func ids(vals []string) string { return orNone(strings.Join(vals, ", ")) }

func env(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = code(k + "=" + m[k])
	}
	return orNone(strings.Join(out, ", "))
}

func limits(l plan.Limits) string {
	return fmt.Sprintf("task_lines %d, split_lines %d, fix_rounds %d, repair_tries %d, blocked_stop %d, null_stop %d, retries %d, gate_slots %d, gate_timeout_min %d, stall_min %d, max_worktrees %d, checks_grace_min %d",
		l.TaskLines, l.SplitLines, l.FixRounds, l.RepairTries, l.BlockedStop, l.NullStop, l.Retries, l.GateSlots, l.GateTimeoutMin, l.StallMin, l.MaxWorktrees, l.ChecksGraceMin)
}

// roles lists role overrides as role=model/effort in plan.Roles order, then
// any unknown role names sorted, with "-" for a field left to a lower layer.
func roles(m map[string]plan.Role) string {
	var keys []string
	for _, r := range plan.Roles {
		if _, ok := m[r]; ok {
			keys = append(keys, r)
		}
	}
	var extra []string
	for k := range m {
		if !slices.Contains(plan.Roles, k) {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	keys = append(keys, extra...)
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = fmt.Sprintf("%s=%s/%s", k, dash(m[k].Model), dash(m[k].Effort))
	}
	return orNone(strings.Join(out, ", "))
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
