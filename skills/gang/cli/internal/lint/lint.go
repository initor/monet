// Package lint checks a plan against the 8 rules of spec section 5.3.
package lint

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"gang/internal/lock"
	"gang/internal/plan"
)

// Problem is one lint finding. Task is "-" or empty for run-wide problems.
type Problem struct {
	Rule int
	Task string
	Msg  string
}

// String formats the problem as "rule <n>: <task or -> : <msg>".
func (p Problem) String() string {
	task := p.Task
	if task == "" {
		task = "-"
	}
	return fmt.Sprintf("rule %d: %s : %s", p.Rule, task, p.Msg)
}

// Options turns on the optional rule 8.
type Options struct{ AdoptSource string }

var (
	idRE        = regexp.MustCompile(`^[A-Z]+[0-9]+$`)
	slugRE      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	refPrefixes = []string{"py:", "cli:", "make:", "file:", "test:"}
)

// Lint returns every problem in p, ordered by rule. p must be normalized,
// as plan.Load returns it. Decode errors are the caller's rule 1 problems.
func Lint(p *plan.Plan, o Options) []Problem {
	l := &linter{p: p}
	l.shape()
	l.required()
	l.references()
	l.coverage()
	l.scopes()
	l.sizes()
	l.models()
	if o.AdoptSource != "" {
		l.adopt(o.AdoptSource)
	}
	return l.out
}

type linter struct {
	p   *plan.Plan
	out []Problem
}

func (l *linter) add(rule int, task, format string, args ...any) {
	l.out = append(l.out, Problem{Rule: rule, Task: task, Msg: fmt.Sprintf(format, args...)})
}

// maxWaveTitle is the longest wave PR title allowed, measured at wave 99.
const maxWaveTitle = 72

// shape is rule 1: values a decoder accepts but the schema does not,
// including the wave PR title template.
func (l *linter) shape() {
	p := l.p
	if p.Version != 1 {
		l.add(1, "-", "version is %d, want 1", p.Version)
	}
	if !strings.Contains(p.Repo.WaveTitle, "{k}") {
		l.add(1, "-", "repo.wave_title %q must contain {k}", p.Repo.WaveTitle)
	}
	if title := plan.WaveTitle(p, 99); utf8.RuneCountInString(title) > maxWaveTitle {
		l.add(1, "-", "repo.wave_title renders to %d characters for wave 99, want at most %d: %q",
			utf8.RuneCountInString(title), maxWaveTitle, title)
	}
	if p.Slug != "" && !slugRE.MatchString(p.Slug) {
		l.add(1, "-", "slug %q must match %s", p.Slug, slugRE)
	}
	if p.Repo.Integration != "" && p.Repo.Integration == p.Repo.Base {
		l.add(1, "-", "repo.integration must differ from repo.base %q", p.Repo.Base)
	}
	if c := p.Repo.Checks; c != plan.ChecksGitHub && c != plan.ChecksGate {
		l.add(1, "-", "repo.checks %q must be github or gate", c)
	}
	for _, name := range p.Locks {
		if !lock.ValidName(name) {
			l.add(1, "-", "lock %q must start with a letter or digit and hold only letters, digits, '.', '_', and '-'", name)
		}
	}
	for _, t := range p.Tasks {
		if t.ID != "" && !idRE.MatchString(t.ID) {
			l.add(1, t.ID, "id must match %s", idRE)
		}
		if t.Risk != "low" && t.Risk != "high" {
			l.add(1, t.ID, "risk %q must be low or high", t.Risk)
		}
		if t.Size.Lines < 0 || t.TimeboxMin < 0 || t.Wave < 0 {
			l.add(1, t.ID, "size.lines, timebox_min, and wave must not be negative")
		}
		for _, ref := range slices.Concat(t.Interfaces.Consumes, t.Interfaces.Produces) {
			if !slices.ContainsFunc(refPrefixes, func(pre string) bool { return strings.HasPrefix(ref, pre) }) {
				l.add(1, t.ID, "interface ref %q must start with one of %s", ref, strings.Join(refPrefixes, " "))
			}
		}
	}
}

// required is rule 2: every required field is present and not empty.
func (l *linter) required() {
	p := l.p
	for _, f := range []struct{ name, v string }{
		{"slug", p.Slug}, {"repo.remote", p.Repo.Remote}, {"repo.base", p.Repo.Base},
		{"repo.integration", p.Repo.Integration}, {"done", p.Done},
	} {
		if strings.TrimSpace(f.v) == "" {
			l.add(2, "-", "%s is empty", f.name)
		}
	}
	l.list("-", "commands.gate", p.Commands.Gate)
	for _, r := range p.Requirements {
		if r.ID == "" || r.Source == "" {
			l.add(2, "-", "requirement %q needs an id and a source", r.ID)
		}
	}
	for _, t := range p.Tasks {
		for _, f := range []struct{ name, v string }{{"id", t.ID}, {"title", t.Title}, {"goal", t.Goal}} {
			if strings.TrimSpace(f.v) == "" {
				l.add(2, t.ID, "%s is empty", f.name)
			}
		}
		l.list(t.ID, "scope.write", t.Scope.Write)
		l.list(t.ID, "acceptance", t.Acceptance)
		l.list(t.ID, "verify.tests_first", t.Verify.TestsFirst)
		if t.Size.Lines == 0 {
			l.add(2, t.ID, "size.lines is 0")
		}
		if t.TimeboxMin == 0 {
			l.add(2, t.ID, "timebox_min is 0")
		}
	}
}

func (l *linter) list(task, name string, vals []string) {
	if len(vals) == 0 {
		l.add(2, task, "%s is empty", name)
		return
	}
	if slices.ContainsFunc(vals, func(v string) bool { return strings.TrimSpace(v) == "" }) {
		l.add(2, task, "%s holds an empty entry", name)
	}
}

// references is rule 3: unique ids, task branches that are neither
// repo.base, repo.integration, nor another task's, resolved references, and
// no cycle across depends_on and merge_after.
func (l *linter) references() {
	p := l.p
	count := map[string]int{}
	for _, t := range p.Tasks {
		count[t.ID]++
	}
	for i, t := range p.Tasks {
		if count[t.ID] > 1 && !slices.ContainsFunc(p.Tasks[:i], func(o plan.Task) bool { return o.ID == t.ID }) {
			l.add(3, t.ID, "id is used by %d tasks", count[t.ID])
		}
	}
	owner := map[string]string{}
	for i := range p.Tasks {
		t := &p.Tasks[i]
		b := plan.BranchFor(t)
		first, taken := owner[b]
		switch {
		case b == p.Repo.Base:
			l.add(3, t.ID, "branch %s is repo.base", b)
		case b == p.Repo.Integration:
			l.add(3, t.ID, "branch %s is repo.integration", b)
		case taken && first != t.ID:
			l.add(3, t.ID, "branch %s is also the branch of %s", b, first)
		case !taken:
			owner[b] = t.ID
		}
	}
	reqs := map[string]int{}
	for _, r := range p.Requirements {
		reqs[r.ID]++
	}
	for _, r := range p.Requirements {
		if reqs[r.ID] > 1 {
			l.add(3, "-", "requirement id %s is listed %d times", r.ID, reqs[r.ID])
			reqs[r.ID] = 1
		}
	}
	for _, t := range p.Tasks {
		for _, ref := range []struct {
			field string
			ids   []string
		}{{"depends_on", t.DependsOn}, {"merge_after", t.MergeAfter}, {"peers", t.Peers}} {
			for _, id := range ref.ids {
				if count[id] == 0 {
					l.add(3, t.ID, "%s %s is not a task", ref.field, id)
				}
			}
		}
		for _, r := range t.Requirements {
			if reqs[r] == 0 {
				l.add(3, t.ID, "requirement %s is not in the run-wide list", r)
			}
		}
		for _, lk := range t.Locks {
			if !slices.Contains(p.Locks, lk) {
				l.add(3, t.ID, "lock %s is not in the run-wide locks", lk)
			}
		}
	}
	if cyc := findCycle(p); cyc != nil {
		l.add(3, cyc[0], "cycle across depends_on and merge_after: %s", strings.Join(cyc, " > "))
	}
}

// findCycle returns one cycle as a path that starts and ends on the same id,
// or nil. Edges run from a task to each of its depends_on and merge_after.
func findCycle(p *plan.Plan) []string {
	edges := map[string][]string{}
	for _, t := range p.Tasks {
		edges[t.ID] = append(edges[t.ID], slices.Concat(t.DependsOn, t.MergeAfter)...)
	}
	const (
		unseen = iota
		active
		done
	)
	state := map[string]int{}
	var stack []string
	var visit func(id string) []string
	visit = func(id string) []string {
		state[id] = active
		stack = append(stack, id)
		for _, next := range edges[id] {
			if _, known := edges[next]; !known {
				continue
			}
			switch state[next] {
			case active:
				start := slices.Index(stack, next)
				return append(slices.Clone(stack[start:]), next)
			case unseen:
				if c := visit(next); c != nil {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
		return nil
	}
	for _, t := range p.Tasks {
		if state[t.ID] == unseen {
			if c := visit(t.ID); c != nil {
				return c
			}
		}
	}
	return nil
}

// coverage is rule 4: each requirement is covered, and a requirement covered
// by more than one task is named in the notes of each of them.
func (l *linter) coverage() {
	by := map[string][]string{}
	notes := map[string]string{}
	for _, t := range l.p.Tasks {
		notes[t.ID] = t.Notes
		seen := map[string]bool{}
		for _, r := range t.Requirements {
			if !seen[r] {
				seen[r] = true
				by[r] = append(by[r], t.ID)
			}
		}
	}
	for _, r := range l.p.Requirements {
		ids := by[r.ID]
		if len(ids) == 0 {
			l.add(4, "-", "requirement %s is covered by no task", r.ID)
			continue
		}
		if len(ids) == 1 {
			continue
		}
		for _, id := range ids {
			if !strings.Contains(notes[id], r.ID) {
				l.add(4, id, "requirement %s is covered by %s: name it in notes with the reason", r.ID, strings.Join(ids, ", "))
			}
		}
	}
}

// scopes is rule 5: tasks whose scope.write globs overlap list each other in peers.
func (l *linter) scopes() {
	ts := l.p.Tasks
	for i := range ts {
		for j := i + 1; j < len(ts); j++ {
			a, b := &ts[i], &ts[j]
			if a.ID == b.ID {
				continue
			}
			ga, gb, ok := firstOverlap(a.Scope.Write, b.Scope.Write)
			if !ok {
				continue
			}
			if !slices.Contains(a.Peers, b.ID) {
				l.add(5, a.ID, "scope.write %s overlaps %s of %s: list %s in peers", ga, gb, b.ID, b.ID)
			}
			if !slices.Contains(b.Peers, a.ID) {
				l.add(5, b.ID, "scope.write %s overlaps %s of %s: list %s in peers", gb, ga, a.ID, a.ID)
			}
		}
	}
}

func firstOverlap(as, bs []string) (string, string, bool) {
	for _, a := range as {
		for _, b := range bs {
			if overlap(a, b) {
				return a, b, true
			}
		}
	}
	return "", "", false
}

// overlap reports whether two scope globs can name the same path. It compares
// the literal text before the first *, ?, or [. Equal literals overlap. A
// literal that is a prefix of the other overlaps when a glob follows it, or
// when it ends on a path segment boundary.
func overlap(a, b string) bool {
	la, ga := literal(a)
	lb, gb := literal(b)
	return covers(la, ga, lb) || covers(lb, gb, la)
}

func literal(glob string) (string, bool) {
	if i := strings.IndexAny(glob, "*?["); i >= 0 {
		return glob[:i], true
	}
	return glob, false
}

func covers(prefix string, glob bool, other string) bool {
	if !strings.HasPrefix(other, prefix) {
		return false
	}
	return glob || len(other) == len(prefix) || strings.HasSuffix(prefix, "/") || other[len(prefix)] == '/'
}

// sizes is rule 6, the size budget. A plan may lower limits.task_lines and
// limits.split_lines but not raise them above the defaults, and split_lines may
// not fall below task_lines. A task above task_lines needs a size.reason, and
// no task goes above split_lines, with a reason or without one.
func (l *linter) sizes() {
	lim, dflt := l.p.Limits, plan.DefaultLimits()
	if lim.TaskLines > dflt.TaskLines {
		l.add(6, "-", "limits.task_lines %d is above the default %d: a plan may lower it, not raise it", lim.TaskLines, dflt.TaskLines)
	}
	if lim.SplitLines > dflt.SplitLines {
		l.add(6, "-", "limits.split_lines %d is above the default %d: a plan may lower it, not raise it", lim.SplitLines, dflt.SplitLines)
	}
	if lim.SplitLines < lim.TaskLines {
		l.add(6, "-", "limits.split_lines %d is below limits.task_lines %d", lim.SplitLines, lim.TaskLines)
	}
	for _, t := range l.p.Tasks {
		switch {
		case t.Size.Lines > lim.SplitLines:
			l.add(6, t.ID, "size.lines %d is above limits.split_lines %d: split the task", t.Size.Lines, lim.SplitLines)
		case t.Size.Lines > lim.TaskLines && strings.TrimSpace(t.Size.Reason) == "":
			l.add(6, t.ID, "size.lines %d is above limits.task_lines %d: split the task, or say in size.reason why it cannot be split", t.Size.Lines, lim.TaskLines)
		}
	}
}

// models is rule 7: role keys, model aliases, and efforts in every layer.
func (l *linter) models() {
	l.roles("-", "models", l.p.Models)
	for _, t := range l.p.Tasks {
		l.roles(t.ID, "model", t.Model)
	}
}

func (l *linter) roles(task, field string, m map[string]plan.Role) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, role := range keys {
		r := m[role]
		if !slices.Contains(plan.Roles, role) {
			l.add(7, task, "%s: unknown role %q", field, role)
		}
		if r.Model != "" && !slices.Contains(plan.Aliases, r.Model) {
			l.add(7, task, "%s.%s.model %q is not one of %s", field, role, r.Model, strings.Join(plan.Aliases, ", "))
		}
		if r.Effort != "" && !slices.Contains(plan.Efforts, r.Effort) {
			l.add(7, task, "%s.%s.effort %q is not one of %s", field, role, r.Effort, strings.Join(plan.Efforts, ", "))
		}
	}
}

// adopt is rule 8: the plan has the source plan's tasks and edges, no more
// and no fewer. A task with a source stands for that source task, so the
// parts of a split task count once and an edge between two parts drops out.
func (l *linter) adopt(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		l.add(8, "-", "read adopt source: %v", err)
		return
	}
	var src struct {
		Tasks []map[string]json.RawMessage `json:"tasks"`
	}
	if err := json.Unmarshal(data, &src); err != nil {
		l.add(8, "-", "adopt source %s: %v", path, err)
		return
	}
	srcIDs, srcEdges := map[string]bool{}, map[string]bool{}
	for i, raw := range src.Tasks {
		var id string
		if err := json.Unmarshal(raw["id"], &id); err != nil || id == "" {
			l.add(8, "-", "adopt source task %d has no string id", i)
			continue
		}
		srcIDs[id] = true
		for _, kind := range []struct {
			name string
			keys []string
		}{{"depends_on", []string{"depends_on", "deps", "dependsOn"}}, {"merge_after", []string{"merge_after", "mergeAfter"}}} {
			targets, err := firstList(raw, kind.keys)
			if err != nil {
				l.add(8, id, "adopt source %s: %v", kind.name, err)
			}
			for _, to := range targets {
				srcEdges[id+" "+kind.name+" "+to] = true
			}
		}
	}
	ids, edges := map[string]bool{}, map[string]bool{}
	edge := func(from, kind, to string) {
		from, to = l.group(from), l.group(to)
		if from != to {
			edges[from+" "+kind+" "+to] = true
		}
	}
	for _, t := range l.p.Tasks {
		ids[l.group(t.ID)] = true
		for _, to := range t.DependsOn {
			edge(t.ID, "depends_on", to)
		}
		for _, to := range t.MergeAfter {
			edge(t.ID, "merge_after", to)
		}
	}
	if len(ids) != len(srcIDs) {
		l.add(8, "-", "task count %d differs from the source's %d", len(ids), len(srcIDs))
	}
	for _, id := range missing(srcIDs, ids) {
		l.add(8, id, "task is in the source but not in the plan")
	}
	for _, id := range missing(ids, srcIDs) {
		l.add(8, id, "task is in the plan but not in the source")
	}
	for _, e := range missing(srcEdges, edges) {
		l.add(8, strings.Fields(e)[0], "edge %s is in the source but not in the plan", e)
	}
	for _, e := range missing(edges, srcEdges) {
		l.add(8, strings.Fields(e)[0], "edge %s is in the plan but not in the source", e)
	}
}

// group returns the source task that the plan task id stands for: its source,
// or id itself when it has none or is not a plan task.
func (l *linter) group(id string) string {
	for _, t := range l.p.Tasks {
		if t.ID == id && t.Source != "" {
			return t.Source
		}
	}
	return id
}

// firstList decodes the first of keys present in raw as a string list.
func firstList(raw map[string]json.RawMessage, keys []string) ([]string, error) {
	for _, k := range keys {
		v, ok := raw[k]
		if !ok || string(v) == "null" {
			continue
		}
		var out []string
		if err := json.Unmarshal(v, &out); err != nil {
			return nil, fmt.Errorf("%s: %v", k, err)
		}
		return out, nil
	}
	return nil, nil
}

// missing returns the sorted keys of a that b lacks.
func missing(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
