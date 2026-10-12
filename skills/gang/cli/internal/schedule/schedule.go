// Package schedule simulates a plan on a capped fleet of agents. It sets each
// task's wave and estimates the critical chain and the wall-clock range.
package schedule

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"gang/internal/plan"
)

// EstimateMinutes is a task's estimated duration: 20 + size.lines/15.
func EstimateMinutes(t *plan.Task) float64 { return 20 + float64(t.Size.Lines)/15 }

// Options tunes Run. Cap 0 means 4 agents. Waves 0 means ceil(tasks/15).
// Calibrate maps task id to actual minutes (run/durations.tsv). Done holds
// the ids of merged tasks. Ids in either that are not in the plan, such as
// engine repair tasks, are skipped.
type Options struct {
	Cap, Waves int
	Calibrate  map[string]float64
	Done       map[string]bool
}

// Result is one simulation. ChainScore and Finish are in estimated minutes,
// before Scale, over every task. Chain and the scaled wall-clock range
// LowMin-HighMin cover the work left: a Done task takes 0 minutes there and
// is never in Chain.
type Result struct {
	Waves           map[string]int
	Chain           []string
	ChainScore      map[string]float64
	Finish          map[string]float64
	LowMin, HighMin float64
	Scale           float64
}

// Run list-schedules p. A task starts when every depends_on task has
// finished, in ChainScore order, while fewer than Cap tasks run and no
// running task holds one of its locks. A merge_after task finishes no
// earlier than its merge_after targets. A cycle or an unknown id is an error.
func Run(p *plan.Plan, o Options) (*Result, error) {
	g, err := newGraph(p)
	if err != nil {
		return nil, err
	}
	scale, err := calibrate(g, o.Calibrate)
	if err != nil {
		return nil, err
	}
	r := &Result{Scale: scale, ChainScore: chainScores(g, nil)}
	slots := o.Cap
	if slots <= 0 {
		slots = 4
	}
	if r.Finish, err = simulate(g, r.ChainScore, slots, nil); err != nil {
		return nil, err
	}
	r.Waves = waves(g, r.Finish, o.Waves)
	left, finish := r.ChainScore, r.Finish
	if len(o.Done) > 0 {
		left = chainScores(g, o.Done)
		if finish, err = simulate(g, left, slots, o.Done); err != nil {
			return nil, err
		}
	}
	r.Chain = chain(g, left, o.Done)
	makespan := 0.0
	for _, f := range finish {
		makespan = math.Max(makespan, f)
	}
	r.LowMin, r.HighMin = makespan*0.8*scale, makespan*1.5*scale
	return r, nil
}

// chainScores returns each task's estimate, 0 when done, plus the highest
// score among its dependents.
func chainScores(g *graph, done map[string]bool) map[string]float64 {
	score := map[string]float64{}
	for i := len(g.topo) - 1; i >= 0; i-- {
		id := g.topo[i]
		best := 0.0
		for _, s := range g.succ[id] {
			best = math.Max(best, score[s])
		}
		score[id] = best
		if !done[id] {
			score[id] += EstimateMinutes(g.byID[id])
		}
	}
	return score
}

type graph struct {
	byID map[string]*plan.Task
	succ map[string][]string // prerequisite to dependents, over both edge kinds
	topo []string            // Kahn order, ready ties broken by id
}

func prereqs(t *plan.Task) []string { return slices.Concat(t.DependsOn, t.MergeAfter) }

func newGraph(p *plan.Plan) (*graph, error) {
	g := &graph{byID: map[string]*plan.Task{}, succ: map[string][]string{}}
	for i := range p.Tasks {
		t := &p.Tasks[i]
		if _, dup := g.byID[t.ID]; dup {
			return nil, fmt.Errorf("duplicate task id %s", t.ID)
		}
		g.byID[t.ID] = t
	}
	indeg := map[string]int{}
	for i := range p.Tasks {
		t := &p.Tasks[i]
		for _, pre := range prereqs(t) {
			if _, ok := g.byID[pre]; !ok {
				return nil, fmt.Errorf("task %s: unknown task %s", t.ID, pre)
			}
			g.succ[pre] = append(g.succ[pre], t.ID)
			indeg[t.ID]++
		}
	}
	var ready []string
	for i := range p.Tasks {
		if indeg[p.Tasks[i].ID] == 0 {
			ready = append(ready, p.Tasks[i].ID)
		}
	}
	for len(ready) > 0 {
		sort.Strings(ready)
		id := ready[0]
		ready = ready[1:]
		g.topo = append(g.topo, id)
		for _, s := range g.succ[id] {
			if indeg[s]--; indeg[s] == 0 {
				ready = append(ready, s)
			}
		}
	}
	if len(g.topo) != len(p.Tasks) {
		var stuck []string
		for i := range p.Tasks {
			if indeg[p.Tasks[i].ID] > 0 {
				stuck = append(stuck, p.Tasks[i].ID)
			}
		}
		sort.Strings(stuck)
		return nil, fmt.Errorf("cycle across depends_on and merge_after among %s", strings.Join(stuck, ", "))
	}
	return g, nil
}

// calibrate returns the median of actual/estimated minutes over the plan's
// tasks, or 1 with no data.
func calibrate(g *graph, actual map[string]float64) (float64, error) {
	ids := make([]string, 0, len(actual))
	for id := range actual {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var ratios []float64
	for _, id := range ids {
		mins := actual[id]
		t, ok := g.byID[id]
		if !ok {
			continue
		}
		if mins <= 0 {
			return 0, fmt.Errorf("calibrate: task %s has %v minutes, want more than 0", id, mins)
		}
		ratios = append(ratios, mins/EstimateMinutes(t))
	}
	if len(ratios) == 0 {
		return 1, nil
	}
	sort.Float64s(ratios)
	n := len(ratios)
	if n%2 == 1 {
		return ratios[n/2], nil
	}
	return (ratios[n/2-1] + ratios[n/2]) / 2, nil
}

// chain follows the highest score from the best start to a sink and leaves
// out done tasks. Ties go to the smaller id.
func chain(g *graph, score map[string]float64, done map[string]bool) []string {
	pick := func(ids []string) string {
		best := ""
		for _, id := range ids {
			if best == "" || score[id] > score[best] || (score[id] == score[best] && id < best) {
				best = id
			}
		}
		return best
	}
	out := []string{}
	for id := pick(g.topo); id != ""; id = pick(g.succ[id]) {
		if !done[id] {
			out = append(out, id)
		}
	}
	return out
}

// simulate runs the list schedule on estimated minutes and returns each
// task's finish time. A done task takes 0 minutes and no agent or lock, and
// settles before any other task starts at the same moment.
func simulate(g *graph, score map[string]float64, slots int, done map[string]bool) (map[string]float64, error) {
	order := slices.Clone(g.topo)
	sort.SliceStable(order, func(i, j int) bool {
		if score[order[i]] != score[order[j]] {
			return score[order[i]] > score[order[j]]
		}
		return order[i] < order[j]
	})
	type job struct {
		id  string
		end float64
	}
	var running []job
	started := map[string]bool{}
	workDone := map[string]float64{}
	finish := map[string]float64{}
	held := map[string]int{}
	now := 0.0
	for len(finish) < len(order) {
		settling := false
		for _, id := range order {
			if done[id] && !started[id] && allIn(g.byID[id].DependsOn, finish) {
				started[id], settling = true, true
				running = append(running, job{id, now})
			}
		}
		for _, id := range order {
			if settling || len(running) >= slots {
				break
			}
			t := g.byID[id]
			if started[id] || !allIn(t.DependsOn, finish) || slices.ContainsFunc(t.Locks, func(l string) bool { return held[l] > 0 }) {
				continue
			}
			started[id] = true
			running = append(running, job{id, now + EstimateMinutes(t)})
			for _, l := range t.Locks {
				held[l]++
			}
		}
		if len(running) == 0 {
			return nil, fmt.Errorf("schedule stuck at %.1f min with %d of %d tasks finished", now, len(finish), len(order))
		}
		now = running[0].end
		for _, j := range running[1:] {
			now = math.Min(now, j.end)
		}
		next := running[:0]
		for _, j := range running {
			if j.end > now {
				next = append(next, j)
				continue
			}
			workDone[j.id] = now
			if done[j.id] {
				continue
			}
			for _, l := range g.byID[j.id].Locks {
				held[l]--
			}
		}
		running = next
		for settled := true; settled; {
			settled = false
			for _, id := range order {
				done, ok := workDone[id]
				if _, fin := finish[id]; fin || !ok || !allIn(g.byID[id].MergeAfter, finish) {
					continue
				}
				for _, m := range g.byID[id].MergeAfter {
					done = math.Max(done, finish[m])
				}
				finish[id] = done
				settled = true
			}
		}
	}
	return finish, nil
}

func allIn(ids []string, m map[string]float64) bool {
	for _, id := range ids {
		if _, ok := m[id]; !ok {
			return false
		}
	}
	return true
}

// waves splits tasks, in finish order with ties in topological order, into
// n groups of equal size, then raises each task to at least the wave of
// every depends_on and merge_after task.
func waves(g *graph, finish map[string]float64, n int) map[string]int {
	total := len(g.topo)
	out := map[string]int{}
	if total == 0 {
		return out
	}
	if n <= 0 {
		n = (total + 14) / 15
	}
	n = min(n, total)
	pos := map[string]int{}
	for i, id := range g.topo {
		pos[id] = i
	}
	order := slices.Clone(g.topo)
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if finish[a] != finish[b] {
			return finish[a] < finish[b]
		}
		return pos[a] < pos[b]
	})
	for i, id := range order {
		out[id] = i*n/total + 1
	}
	for _, id := range g.topo {
		for _, pre := range prereqs(g.byID[id]) {
			out[id] = max(out[id], out[pre])
		}
	}
	return out
}
