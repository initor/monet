package repair

import (
	"reflect"
	"testing"

	"gang/internal/plan"
)

func fixture() *plan.Plan {
	p := &plan.Plan{
		Slug:     "toy",
		Commands: plan.Commands{Gate: []string{"make check"}, Health: []string{"make check", "make e2e"}},
		Locks:    []string{"kind"},
		Tasks:    []plan.Task{{ID: "A1", Title: "Task A1"}, {ID: "F02", Title: "Planned fix", DependsOn: []string{"A1"}}},
	}
	p.Normalize()
	return p
}

// Contract 0 rule 16: F<nn> ids missing from plan.json are synthetic repair
// tasks. Every other unknown id stays unknown.
func TestTaskForIDs(t *testing.T) {
	p := fixture()
	for _, id := range []string{"A1", "F01", "F02", "F123"} {
		if _, ok := TaskFor(p, id); !ok {
			t.Errorf("%s: not found", id)
		}
	}
	for _, id := range []string{"Z9", "F1", "F", "X01", "F01a", "f01", ""} {
		if _, ok := TaskFor(p, id); ok {
			t.Errorf("%s: found, want unknown", id)
		}
	}
}

func TestSyntheticRepairTask(t *testing.T) {
	p := fixture()
	r, _ := TaskFor(p, "F01")
	if r.ID != "F01" || plan.BranchFor(r) != "gang/F01" || r.Risk != "low" {
		t.Fatalf("id %s branch %s risk %s", r.ID, plan.BranchFor(r), r.Risk)
	}
	if len(r.DependsOn) != 0 || len(r.MergeAfter) != 0 || r.DependsOn == nil || r.MergeAfter == nil {
		t.Fatalf("deps %v merge_after %v, want empty lists", r.DependsOn, r.MergeAfter)
	}
	if r.Goal != "make every health command pass on the integration branch" {
		t.Fatalf("goal %q", r.Goal)
	}
	if !reflect.DeepEqual(r.Scope.Write, []string{"**"}) || !reflect.DeepEqual(r.Locks, p.Locks) {
		t.Fatalf("scope %v locks %v", r.Scope.Write, r.Locks)
	}
	if _, inPlan := p.Task("F01"); inPlan {
		t.Fatal("TaskFor added F01 to the plan")
	}
}

func TestPlannedFixKeepsItsPlanEntry(t *testing.T) {
	p := fixture()
	f, _ := TaskFor(p, "F02")
	if f.Title != "Planned fix" || !reflect.DeepEqual(f.DependsOn, []string{"A1"}) {
		t.Fatalf("F02 in plan.json was replaced: %+v", f)
	}
}

func TestGateFor(t *testing.T) {
	p := fixture()
	a1, _ := TaskFor(p, "A1")
	f01, _ := TaskFor(p, "F01")
	f02, _ := TaskFor(p, "F02")
	if got := GateFor(p, f01); !reflect.DeepEqual(got, p.Commands.Health) {
		t.Errorf("F01 gate %v, want health %v", got, p.Commands.Health)
	}
	for _, task := range []*plan.Task{a1, f02} {
		if got := GateFor(p, task); !reflect.DeepEqual(got, p.Commands.Gate) {
			t.Errorf("%s gate %v, want %v", task.ID, got, p.Commands.Gate)
		}
	}
}
