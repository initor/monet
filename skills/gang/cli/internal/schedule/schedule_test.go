package schedule

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"gang/internal/plan"
)

// fixedGraph is the graph every expected value below was worked out on by
// hand. Estimates (20 + lines/15): T1 30, T2 40, T3 30, T4 50, T5 25.
// Edges: T4 depends_on T3, T5 depends_on T2, T1 merge_after T4.
// T2 and T3 share lock db.
func fixedGraph() *plan.Plan {
	p := &plan.Plan{Locks: []string{"db"}, Tasks: []plan.Task{
		{ID: "T1", Size: plan.Size{Lines: 150}, MergeAfter: []string{"T4"}},
		{ID: "T2", Size: plan.Size{Lines: 300}, Locks: []string{"db"}},
		{ID: "T3", Size: plan.Size{Lines: 150}, Locks: []string{"db"}},
		{ID: "T4", Size: plan.Size{Lines: 450}, DependsOn: []string{"T3"}},
		{ID: "T5", Size: plan.Size{Lines: 75}, DependsOn: []string{"T2"}},
	}}
	p.Normalize()
	return p
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestEstimateMinutes(t *testing.T) {
	if got := EstimateMinutes(&plan.Task{Size: plan.Size{Lines: 300}}); got != 40 {
		t.Errorf("EstimateMinutes(300 lines) = %v, want 40", got)
	}
}

// With cap 2: t=0 T3 starts (db) and T1 starts, T2 waits on db. t=30 T3
// ends, T1's work ends but it waits for T4, T4 and T2 start. t=70 T2 ends,
// T5 starts. t=80 T4 ends, so T1 finishes. t=95 T5 ends.
func TestRunFixedGraph(t *testing.T) {
	r, err := Run(fixedGraph(), Options{Cap: 2, Waves: 2})
	if err != nil {
		t.Fatal(err)
	}
	wantScore := map[string]float64{"T1": 30, "T2": 65, "T3": 110, "T4": 80, "T5": 25}
	wantFinish := map[string]float64{"T1": 80, "T2": 70, "T3": 30, "T4": 80, "T5": 95}
	for id := range wantScore {
		if !near(r.ChainScore[id], wantScore[id]) {
			t.Errorf("ChainScore[%s] = %v, want %v", id, r.ChainScore[id], wantScore[id])
		}
		if !near(r.Finish[id], wantFinish[id]) {
			t.Errorf("Finish[%s] = %v, want %v", id, r.Finish[id], wantFinish[id])
		}
	}
	if want := []string{"T3", "T4", "T1"}; !reflect.DeepEqual(r.Chain, want) {
		t.Errorf("Chain = %v, want %v", r.Chain, want)
	}
	// Finish order T3 30, T2 70, T4 80, T1 80, T5 95. T4 and T1 tie, and T4
	// goes first because T1 merges after it. Five tasks in two waves: 3 + 2.
	if want := map[string]int{"T3": 1, "T2": 1, "T4": 1, "T1": 2, "T5": 2}; !reflect.DeepEqual(r.Waves, want) {
		t.Errorf("Waves = %v, want %v", r.Waves, want)
	}
	if !near(r.LowMin, 76) || !near(r.HighMin, 142.5) || r.Scale != 1 {
		t.Errorf("range %v-%v scale %v, want 76-142.5 scale 1", r.LowMin, r.HighMin, r.Scale)
	}
}

// With T3 merged it takes 0 minutes, so the chain left is T4 > T1. Cap 2 on
// the work left: t=0 T4 and T2 start, t=40 T2 ends and T1 starts, t=50 T4
// ends and T5 starts, t=70 T1 finishes, t=75 T5 ends. Waves, Finish, and
// ChainScore cover every task, as in TestRunFixedGraph.
func TestRunDoneLeavesChainAndRange(t *testing.T) {
	full, err := Run(fixedGraph(), Options{Cap: 2, Waves: 2})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Run(fixedGraph(), Options{Cap: 2, Waves: 2, Done: map[string]bool{"T3": true, "Z9": true}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"T4", "T1"}; !reflect.DeepEqual(r.Chain, want) {
		t.Errorf("Chain = %v, want %v", r.Chain, want)
	}
	if !near(r.LowMin, 60) || !near(r.HighMin, 112.5) {
		t.Errorf("range %v-%v, want 60-112.5", r.LowMin, r.HighMin)
	}
	if !reflect.DeepEqual(r.Waves, full.Waves) || !reflect.DeepEqual(r.Finish, full.Finish) || !reflect.DeepEqual(r.ChainScore, full.ChainScore) {
		t.Errorf("waves %v finish %v score %v, want %v %v %v", r.Waves, r.Finish, r.ChainScore, full.Waves, full.Finish, full.ChainScore)
	}

	all := map[string]bool{"T1": true, "T2": true, "T3": true, "T4": true, "T5": true}
	r, err = Run(fixedGraph(), Options{Cap: 2, Waves: 2, Done: all})
	if err != nil {
		t.Fatal(err)
	}
	if r.Chain == nil || len(r.Chain) != 0 || r.LowMin != 0 || r.HighMin != 0 {
		t.Errorf("all done: chain %#v range %v-%v, want empty and 0-0", r.Chain, r.LowMin, r.HighMin)
	}
	if !reflect.DeepEqual(r.Waves, full.Waves) {
		t.Errorf("all done: waves %v, want %v", r.Waves, full.Waves)
	}
}

// A merged task needs no agent: with cap 3, the merged D1, D2, and D3
// settle at t=0 before R1 and R2 take slots, so N2 and N3 start at once.
// Makespan 100 (N2, N3), not 130 (N3 waiting for R1).
func TestRunDoneTasksNeedNoAgent(t *testing.T) {
	p := &plan.Plan{Tasks: []plan.Task{
		{ID: "D1", Size: plan.Size{Lines: 150}},
		{ID: "D2", Size: plan.Size{Lines: 150}, DependsOn: []string{"D1"}},
		{ID: "D3", Size: plan.Size{Lines: 150}, DependsOn: []string{"D1"}},
		{ID: "N2", Size: plan.Size{Lines: 1200}, DependsOn: []string{"D2"}},
		{ID: "N3", Size: plan.Size{Lines: 1200}, DependsOn: []string{"D3"}},
		{ID: "R1", Size: plan.Size{Lines: 150}},
		{ID: "R2", Size: plan.Size{Lines: 150}},
	}}
	p.Normalize()
	r, err := Run(p, Options{Cap: 3, Done: map[string]bool{"D1": true, "D2": true, "D3": true}})
	if err != nil {
		t.Fatal(err)
	}
	if !near(r.LowMin, 80) || !near(r.HighMin, 150) {
		t.Errorf("range %v-%v, want 80-150", r.LowMin, r.HighMin)
	}
}

func TestRunLockSerializesHolders(t *testing.T) {
	p := fixedGraph()
	for i := range p.Tasks {
		p.Tasks[i].Locks = []string{}
	}
	r, err := Run(p, Options{Cap: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !near(r.Finish["T2"], 40) {
		t.Errorf("without the lock T2 finishes at %v, want 40", r.Finish["T2"])
	}
}

// With cap 1 the order is T3, T4, T2, T1, T5: makespan 175.
func TestRunCapAndDefaults(t *testing.T) {
	r, err := Run(fixedGraph(), Options{Cap: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !near(r.LowMin, 140) || !near(r.HighMin, 262.5) {
		t.Errorf("cap 1 range %v-%v, want 140-262.5", r.LowMin, r.HighMin)
	}
	r, err = Run(fixedGraph(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for id, w := range r.Waves {
		if w != 1 {
			t.Errorf("default waves: %s in wave %d, want 1 (ceil(5/15) = 1)", id, w)
		}
	}
	if !near(r.LowMin, 76) {
		t.Errorf("default cap 4 range starts at %v, want 76", r.LowMin)
	}
}

func TestRunCalibrateScalesByMedian(t *testing.T) {
	cases := []struct {
		real      map[string]float64
		scale     float64
		low, high float64
	}{
		{map[string]float64{"T3": 60, "T2": 20}, 1.25, 95, 178.125},
		{map[string]float64{"T3": 60, "T2": 20, "T5": 50}, 2, 152, 285},
		{map[string]float64{"T3": 60, "F01": 5}, 2, 152, 285}, // F01 is not in the plan
	}
	for _, c := range cases {
		r, err := Run(fixedGraph(), Options{Cap: 2, Calibrate: c.real})
		if err != nil {
			t.Fatal(err)
		}
		if !near(r.Scale, c.scale) || !near(r.LowMin, c.low) || !near(r.HighMin, c.high) {
			t.Errorf("%v: scale %v range %v-%v, want %v %v-%v", c.real, r.Scale, r.LowMin, r.HighMin, c.scale, c.low, c.high)
		}
	}
	if _, err := Run(fixedGraph(), Options{Calibrate: map[string]float64{"T1": 0}}); err == nil {
		t.Error("zero calibrate minutes were accepted")
	}
}

func TestRunRejectsBadGraphs(t *testing.T) {
	cyc := fixedGraph()
	cyc.Tasks[2].MergeAfter = []string{"T1"} // T3 waits on T1, which waits on T4, which needs T3
	_, err := Run(cyc, Options{})
	if err == nil || !strings.Contains(err.Error(), "cycle") || !strings.Contains(err.Error(), "T1, T3, T4") {
		t.Errorf("cycle: err %v", err)
	}
	unknown := fixedGraph()
	unknown.Tasks[0].DependsOn = []string{"Z9"}
	if _, err := Run(unknown, Options{}); err == nil || !strings.Contains(err.Error(), "Z9") {
		t.Errorf("unknown dep: err %v", err)
	}
}

func TestRunEmptyPlan(t *testing.T) {
	r, err := Run(&plan.Plan{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Waves) != 0 || len(r.Chain) != 0 || r.LowMin != 0 || r.HighMin != 0 || r.Scale != 1 {
		t.Errorf("empty plan result %+v", r)
	}
}
