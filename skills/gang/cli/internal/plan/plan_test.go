package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadIsStrict(t *testing.T) {
	cases := map[string]string{
		"unknown top field":  `{"version":1,"bogus":true}`,
		"unknown task field": `{"version":1,"tasks":[{"id":"A1","color":"red"}]}`,
		"wrong type":         `{"version":"1"}`,
		"trailing object":    `{"version":1} {}`,
		"trailing garbage":   `{"version":1} x`,
	}
	for name, content := range cases {
		if _, err := Load(writeFile(t, content)); err == nil {
			t.Errorf("%s: Load accepted %s", name, content)
		}
	}
	if _, err := Load(writeFile(t, "{\"version\":1}\n\n")); err != nil {
		t.Errorf("trailing whitespace: %v", err)
	}
}

// The defaults come from the Normalize row of Contract 1.
func TestLoadNormalizes(t *testing.T) {
	p, err := Load(writeFile(t, `{"version":1,"limits":{"task_lines":800},"tasks":[{"id":"A1"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := Limits{TaskLines: 800, SplitLines: 300, FixRounds: 3, RepairTries: 2, BlockedStop: 3,
		NullStop: 6, Retries: 2, GateSlots: 3, GateTimeoutMin: 60, StallMin: 60, MaxWorktrees: 400, ChecksGraceMin: 10}
	if p.Limits != want {
		t.Errorf("limits %+v, want %+v", p.Limits, want)
	}
	if p.Repo.WaveTitle != "chore(gang): land wave {k} of {slug}" {
		t.Errorf("wave_title %q, want the Contract 1 default", p.Repo.WaveTitle)
	}
	if p.Repo.Checks != "github" {
		t.Errorf("checks %q, want github", p.Repo.Checks)
	}
	if g, err := Load(writeFile(t, `{"version":1,"repo":{"checks":"gate"},"limits":{"checks_grace_min":3}}`)); err != nil {
		t.Error(err)
	} else if g.Repo.Checks != "gate" || g.Limits.ChecksGraceMin != 3 {
		t.Errorf("explicit checks %q and grace %d, want gate and 3", g.Repo.Checks, g.Limits.ChecksGraceMin)
	}
	tk := p.Tasks[0]
	if tk.Risk != "low" {
		t.Errorf("risk %q, want low", tk.Risk)
	}
	if tk.DependsOn == nil || tk.Scope.Write == nil || tk.Verify.TestsFirst == nil || tk.Locks == nil {
		t.Error("task slices left nil")
	}
	if p.Locks == nil || p.Requirements == nil || p.Commands.Gate == nil || p.Env == nil || p.Models == nil {
		t.Error("plan slices or maps left nil")
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), ":null") {
		t.Errorf("normalized plan still marshals a null: %s", data)
	}
}

// DefaultLimits is what Normalize fills into a plan that sets no limits.
func TestDefaultLimitsMatchNormalize(t *testing.T) {
	var p Plan
	p.Normalize()
	if DefaultLimits() != p.Limits {
		t.Errorf("DefaultLimits() %+v, want %+v", DefaultLimits(), p.Limits)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	p, err := Load(writeFile(t, `{"version":1,"slug":"demo","tasks":[{"id":"A1","depends_on":[],"model":{"review":{"model":"sonnet","effort":""}}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "out", "plan.json")
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(want)+"\n" {
		t.Errorf("Save wrote other bytes than MarshalIndent plus a newline:\n%s", data)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, p) {
		t.Errorf("round trip changed the plan:\n%+v\n%+v", back, p)
	}
}

// size.reason survives Save and Load, and omitempty keeps an empty one out of
// the file.
func TestSizeReasonRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	p := &Plan{Version: 1, Tasks: []Task{
		{ID: "A1", Size: Size{Lines: 150, Reason: "generated table"}},
		{ID: "B1", Size: Size{Lines: 40}},
	}}
	if err := Save(path, p); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), `"reason":`); n != 1 {
		t.Errorf("file has %d reason keys, want 1:\n%s", n, data)
	}
	back, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Tasks) != 2 {
		t.Fatalf("Load returned %d tasks, want 2", len(back.Tasks))
	}
	if got, want := back.Tasks[0].Size, (Size{Lines: 150, Reason: "generated table"}); got != want {
		t.Errorf("A1 size %+v, want %+v", got, want)
	}
	if got, want := back.Tasks[1].Size, (Size{Lines: 40}); got != want {
		t.Errorf("B1 size %+v, want %+v", got, want)
	}
}

func TestWaveTitle(t *testing.T) {
	p := &Plan{Slug: "demo-phase2"}
	if got := WaveTitle(p, 3); got != "chore(gang): land wave 3 of demo-phase2" {
		t.Errorf("default title %q", got)
	}
	p.Repo.WaveTitle = "feat({slug}): wave {k}"
	if got := WaveTitle(p, 12); got != "feat(demo-phase2): wave 12" {
		t.Errorf("custom title %q", got)
	}
}

// The ref name comes from Contract 0 item 3.
func TestRef(t *testing.T) {
	if got := Ref("demo-phase2"); got != "refs/gang/demo-phase2/plan" {
		t.Errorf("Ref = %s", got)
	}
}

func TestTaskAndBranchFor(t *testing.T) {
	p := &Plan{Tasks: []Task{{ID: "A1"}, {ID: "B2", Branch: "feat/b2"}}}
	a, ok := p.Task("A1")
	if !ok || a.ID != "A1" {
		t.Fatalf("Task(A1) = %v %v", a, ok)
	}
	a.Title = "edited"
	if p.Tasks[0].Title != "edited" {
		t.Error("Task returned a copy, want a pointer into the plan")
	}
	if _, ok := p.Task("Z9"); ok {
		t.Error("Task(Z9) found a task")
	}
	b, _ := p.Task("B2")
	if got := BranchFor(a); got != "gang/A1" {
		t.Errorf("BranchFor(A1) = %s, want gang/A1", got)
	}
	if got := BranchFor(b); got != "feat/b2" {
		t.Errorf("BranchFor(B2) = %s, want feat/b2", got)
	}
}

func TestResolveRoleFallsThroughFieldByField(t *testing.T) {
	defaults := map[string]Role{"implement": {Model: "opus", Effort: "high"}, "merge": {Model: "opus", Effort: "low"}}
	p := &Plan{Models: map[string]Role{"implement": {Effort: "xhigh"}}}
	tk := &Task{Model: map[string]Role{"implement": {Model: "sonnet"}}}
	cases := []struct {
		name string
		p    *Plan
		t    *Task
		role string
		want Role
	}{
		{"all three layers", p, tk, "implement", Role{Model: "sonnet", Effort: "xhigh"}},
		{"run level", p, nil, "implement", Role{Model: "opus", Effort: "xhigh"}},
		{"defaults only", p, tk, "merge", Role{Model: "opus", Effort: "low"}},
		{"no plan", nil, tk, "implement", Role{Model: "sonnet", Effort: "high"}},
		{"unknown role", p, tk, "review", Role{}},
	}
	for _, c := range cases {
		if got := ResolveRole(c.p, c.t, c.role, defaults); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
}

// The sets come from Contract 1.
func TestClosedSets(t *testing.T) {
	if want := []string{"implement", "implement_hard", "review", "review_adversarial", "fix", "sync", "merge", "health", "repair", "plan_draft", "plan_review"}; !reflect.DeepEqual(Roles, want) {
		t.Errorf("Roles = %v", Roles)
	}
	if want := []string{"opus", "sonnet", "haiku", "fable"}; !reflect.DeepEqual(Aliases, want) {
		t.Errorf("Aliases = %v", Aliases)
	}
	if want := []string{"low", "medium", "high", "xhigh", "max"}; !reflect.DeepEqual(Efforts, want) {
		t.Errorf("Efforts = %v", Efforts)
	}
}
