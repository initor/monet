package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gang/internal/plan"
)

// validPlan is the smallest plan that passes every rule: two tasks, B1 after
// A1, each covering one requirement, with disjoint scopes.
func validPlan() *plan.Plan {
	p := &plan.Plan{
		Version:  1,
		Slug:     "demo",
		Repo:     plan.Repo{Remote: "github.com/example/demo", Base: "main", Integration: "gang-demo", GHAccount: "example"},
		Commands: plan.Commands{Gate: []string{"make check"}},
		Locks:    []string{"db"},
		Done:     "every task is reachable from origin/main",
		Requirements: []plan.Requirement{
			{ID: "REQ-1", Source: "spec.md:1"},
			{ID: "REQ-2", Source: "spec.md:2"},
		},
		Tasks: []plan.Task{
			{
				ID: "A1", Title: "alpha", Goal: "Add package alpha.",
				Scope:        plan.Scope{Write: []string{"alpha/**"}},
				Requirements: []string{"REQ-1"},
				Interfaces:   plan.Interfaces{Produces: []string{"py:alpha.run"}},
				Acceptance:   []string{"alpha.run returns 0"},
				Verify:       plan.Verify{TestsFirst: []string{"test:test_alpha"}},
				Size:         plan.Size{Lines: 200}, TimeboxMin: 30,
			},
			{
				ID: "B1", Title: "beta", Goal: "Add package beta.",
				Scope:        plan.Scope{Write: []string{"beta/**"}},
				DependsOn:    []string{"A1"},
				Requirements: []string{"REQ-2"},
				Interfaces:   plan.Interfaces{Consumes: []string{"py:alpha.run"}},
				Acceptance:   []string{"beta calls alpha.run"},
				Verify:       plan.Verify{TestsFirst: []string{"test:test_beta"}},
				Size:         plan.Size{Lines: 300}, TimeboxMin: 45, Locks: []string{"db"}, Risk: "high",
			},
		},
	}
	p.Normalize()
	return p
}

func firedRules(ps []Problem) []int {
	seen := map[int]bool{}
	var out []int
	for _, p := range ps {
		if !seen[p.Rule] {
			seen[p.Rule] = true
			out = append(out, p.Rule)
		}
	}
	sort.Ints(out)
	return out
}

func TestValidPlanIsClean(t *testing.T) {
	if ps := Lint(validPlan(), Options{}); len(ps) != 0 {
		t.Fatalf("valid plan has problems: %v", ps)
	}
}

func TestEachRuleFiresAlone(t *testing.T) {
	cases := []struct {
		name   string
		rule   int
		mutate func(p *plan.Plan)
	}{
		{"version", 1, func(p *plan.Plan) { p.Version = 2 }},
		{"slug format", 1, func(p *plan.Plan) { p.Slug = "Demo Run" }},
		{"integration is base", 1, func(p *plan.Plan) { p.Repo.Integration = "main" }},
		{"id format", 1, func(p *plan.Plan) { p.Tasks[1].ID = "b1" }},
		{"risk value", 1, func(p *plan.Plan) { p.Tasks[0].Risk = "medium" }},
		{"negative wave", 1, func(p *plan.Plan) { p.Tasks[0].Wave = -1 }},
		{"interface ref prefix", 1, func(p *plan.Plan) { p.Tasks[0].Interfaces.Produces = []string{"alpha.run"} }},
		{"wave title of 73 at wave 99", 1, func(p *plan.Plan) {
			p.Repo.WaveTitle = plan.DefaultWaveTitle + " " + strings.Repeat("x", 72-33)
		}},
		{"default wave title with a long slug", 1, func(p *plan.Plan) {
			p.Slug = "a-slug-that-is-much-too-long-for-the-wave-title"
		}},
		{"wave title without {k}", 1, func(p *plan.Plan) { p.Repo.WaveTitle = "chore(gang): land {slug}" }},
		{"lock name with a slash", 1, func(p *plan.Plan) { p.Locks = append(p.Locks, "k8s/kind") }},

		{"empty goal", 2, func(p *plan.Plan) { p.Tasks[0].Goal = " " }},
		{"no acceptance", 2, func(p *plan.Plan) { p.Tasks[0].Acceptance = []string{} }},
		{"empty acceptance entry", 2, func(p *plan.Plan) { p.Tasks[0].Acceptance = []string{"ok", ""} }},
		{"no tests_first", 2, func(p *plan.Plan) { p.Tasks[1].Verify.TestsFirst = nil }},
		{"no scope.write", 2, func(p *plan.Plan) { p.Tasks[1].Scope.Write = []string{} }},
		{"zero size", 2, func(p *plan.Plan) { p.Tasks[0].Size.Lines = 0 }},
		{"zero timebox", 2, func(p *plan.Plan) { p.Tasks[0].TimeboxMin = 0 }},
		{"no gate", 2, func(p *plan.Plan) { p.Commands.Gate = []string{} }},
		{"no remote", 2, func(p *plan.Plan) { p.Repo.Remote = "" }},

		{"duplicate id", 3, func(p *plan.Plan) { p.Tasks[1].ID = "A1" }},
		{"unknown depends_on", 3, func(p *plan.Plan) { p.Tasks[1].DependsOn = []string{"Z9"} }},
		{"unknown merge_after", 3, func(p *plan.Plan) { p.Tasks[0].MergeAfter = []string{"Z9"} }},
		{"unknown peer", 3, func(p *plan.Plan) { p.Tasks[0].Peers = []string{"Z9"} }},
		{"unknown requirement", 3, func(p *plan.Plan) { p.Tasks[0].Requirements = append(p.Tasks[0].Requirements, "REQ-9") }},
		{"unknown lock", 3, func(p *plan.Plan) { p.Tasks[0].Locks = []string{"gpu"} }},
		{"cycle through merge_after", 3, func(p *plan.Plan) { p.Tasks[0].MergeAfter = []string{"B1"} }},
		{"self dependency", 3, func(p *plan.Plan) { p.Tasks[0].DependsOn = []string{"A1"} }},
		{"branch is repo.integration", 3, func(p *plan.Plan) { p.Tasks[0].Branch = "gang-demo" }},
		{"branch is repo.base", 3, func(p *plan.Plan) { p.Tasks[1].Branch = "main" }},
		{"branch shared by two tasks", 3, func(p *plan.Plan) { p.Tasks[1].Branch = "gang/A1" }},

		{"uncovered requirement", 4, func(p *plan.Plan) {
			p.Requirements = append(p.Requirements, plan.Requirement{ID: "REQ-3", Source: "spec.md:3"})
		}},
		{"double coverage without notes", 4, func(p *plan.Plan) {
			p.Tasks[0].Requirements = []string{"REQ-1", "REQ-2"}
			p.Tasks[1].Notes = "REQ-2 needs both the alpha and beta halves"
		}},
		{"double coverage with notes that skip the id", 4, func(p *plan.Plan) {
			p.Tasks[0].Requirements = []string{"REQ-1", "REQ-2"}
			p.Tasks[0].Notes = "shares a requirement with beta"
			p.Tasks[1].Notes = "REQ-2 needs both the alpha and beta halves"
		}},

		{"overlapping scopes", 5, func(p *plan.Plan) { p.Tasks[1].Scope.Write = []string{"alpha/beta.go"} }},
		{"peers only on the later task", 5, func(p *plan.Plan) {
			p.Tasks[1].Scope.Write = []string{"alpha/beta.go"}
			p.Tasks[1].Peers = []string{"A1"}
		}},
		{"peers only on the earlier task", 5, func(p *plan.Plan) {
			p.Tasks[1].Scope.Write = []string{"alpha/beta.go"}
			p.Tasks[0].Peers = []string{"B1"}
		}},

		{"too large", 6, func(p *plan.Plan) { p.Tasks[1].Size.Lines = 1201 }},
		{"size above task_lines without a reason", 6, func(p *plan.Plan) { p.Tasks[0].Size.Lines = p.Limits.TaskLines + 1 }},
		{"size above task_lines with a blank reason", 6, func(p *plan.Plan) {
			p.Tasks[0].Size = plan.Size{Lines: p.Limits.TaskLines + 1, Reason: "  "}
		}},
		{"size above split_lines with a reason", 6, func(p *plan.Plan) {
			p.Tasks[0].Size = plan.Size{Lines: p.Limits.SplitLines + 1, Reason: "generated"}
		}},
		{"task_lines raised", 6, func(p *plan.Plan) { p.Limits.TaskLines = plan.DefaultLimits().TaskLines + 1 }},
		{"split_lines raised", 6, func(p *plan.Plan) { p.Limits.SplitLines = plan.DefaultLimits().SplitLines + 1 }},
		{"split_lines below task_lines", 6, func(p *plan.Plan) { p.Limits.SplitLines = p.Limits.TaskLines - 1 }},
		{"lowered limit", 6, func(p *plan.Plan) { p.Limits.TaskLines = 60; p.Tasks[0].Size.Lines = 70 }},

		{"raw model id in plan models", 7, func(p *plan.Plan) { p.Models["review"] = plan.Role{Model: "claude-opus-4-1"} }},
		{"bad effort in task model", 7, func(p *plan.Plan) {
			p.Tasks[0].Model = map[string]plan.Role{"implement": {Effort: "extreme"}}
		}},
		{"unknown role", 7, func(p *plan.Plan) { p.Models["reviewer"] = plan.Role{Model: "opus"} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validPlan()
			c.mutate(p)
			ps := Lint(p, Options{})
			if got := firedRules(ps); !reflect.DeepEqual(got, []int{c.rule}) {
				t.Errorf("rules %v, want [%d]: %v", got, c.rule, ps)
			}
		})
	}
}

// Contract 0 item 21: the slug names the ref refs/gang/<slug>/plan, so it
// must match ^[a-z0-9][a-z0-9-]*$. A bad slug fires rule 1 and nothing else.
func TestSlugShape(t *testing.T) {
	for _, slug := range []string{"Demo", "demo_run", "-demo", "demo/x", ".config", "demo run", "démo"} {
		p := validPlan()
		p.Slug = slug
		ps := Lint(p, Options{})
		if got := firedRules(ps); !reflect.DeepEqual(got, []int{1}) || !strings.Contains(ps[0].Msg, "slug") {
			t.Errorf("slug %q: %v, want one rule 1 slug problem", slug, ps)
		}
	}
	for _, slug := range []string{"config", "demo-phase2", "a", "0x9"} {
		p := validPlan()
		p.Slug = slug
		if ps := Lint(p, Options{}); len(ps) != 0 {
			t.Errorf("slug %q: %v, want clean", slug, ps)
		}
	}
}

// Contract 0 item 9: an empty gh_account means no GitHub, not a lint error.
// The default title for slug "demo" at wave 99 is 33 characters, and a
// title of exactly 72 passes.
func TestOptionalFieldsAndTitleLimit(t *testing.T) {
	p := validPlan()
	p.Repo.GHAccount = ""
	p.Repo.WaveTitle = plan.DefaultWaveTitle + " " + strings.Repeat("x", 72-33-1)
	if n := len(plan.WaveTitle(p, 99)); n != 72 {
		t.Fatalf("test setup: title is %d characters, want 72", n)
	}
	if ps := Lint(p, Options{}); len(ps) != 0 {
		t.Errorf("problems %v", ps)
	}
}

// gang claim switches a worktree onto the task branch, so a task branch that
// is the trunk or another task's branch would land unreviewed commits there.
// A shared branch is reported on the later task and names the earlier one.
func TestBranchProblems(t *testing.T) {
	cases := []struct {
		mutate func(p *plan.Plan)
		want   string
	}{
		{func(p *plan.Plan) { p.Tasks[0].Branch = "gang-demo" }, "rule 3: A1 : branch gang-demo is repo.integration"},
		{func(p *plan.Plan) { p.Tasks[0].Branch = "main" }, "rule 3: A1 : branch main is repo.base"},
		{func(p *plan.Plan) { p.Tasks[0].Branch, p.Tasks[1].Branch = "feat/x", "feat/x" }, "rule 3: B1 : branch feat/x is also the branch of A1"},
		{func(p *plan.Plan) { p.Tasks[0].Branch = "gang/B1" }, "rule 3: B1 : branch gang/B1 is also the branch of A1"},
	}
	for _, c := range cases {
		p := validPlan()
		c.mutate(p)
		ps := Lint(p, Options{})
		if len(ps) != 1 || ps[0].String() != c.want {
			t.Errorf("problems %v, want [%s]", ps, c.want)
		}
	}
	p := validPlan()
	p.Tasks[0].Branch = "feat/alpha"
	if ps := Lint(p, Options{}); len(ps) != 0 {
		t.Errorf("custom branch: problems %v, want none", ps)
	}
}

func TestSharedRequirementWithNotesIsClean(t *testing.T) {
	p := validPlan()
	p.Tasks[0].Requirements = []string{"REQ-1", "REQ-2"}
	p.Tasks[0].Notes = "REQ-2: alpha owns the parser half"
	p.Tasks[1].Notes = "REQ-2: beta owns the CLI half"
	if ps := Lint(p, Options{}); len(ps) != 0 {
		t.Errorf("problems %v", ps)
	}
}

func TestMutualPeersAllowOverlap(t *testing.T) {
	p := validPlan()
	p.Tasks[1].Scope.Write = []string{"alpha/beta.go"}
	p.Tasks[0].Peers = []string{"B1"}
	p.Tasks[1].Peers = []string{"A1"}
	if ps := Lint(p, Options{}); len(ps) != 0 {
		t.Errorf("problems %v", ps)
	}
}

func TestOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"pkg/a.go", "pkg/a.go", true},
		{"pkg/**", "pkg/a.go", true},
		{"pkg/foo", "pkg/foo/bar.go", true},
		{"pkg/foo/", "pkg/foo/bar.go", true},
		{"pkg/foo*.go", "pkg/foobar.go", true},
		{"**", "anything/at/all", true},
		{"pkg/*.go", "pkg/sub/x.go", true},
		{"pkg/foo", "pkg/foobar.go", false},
		{"pkg/foo*.go", "pkg/bar.go", false},
		{"alpha/**", "beta/**", false},
		{"cmd/a/main.go", "cmd/b/main.go", false},
	}
	for _, c := range cases {
		if got := overlap(c.a, c.b); got != c.want {
			t.Errorf("overlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
		if got := overlap(c.b, c.a); got != c.want {
			t.Errorf("overlap(%q, %q) = %v, want %v", c.b, c.a, got, c.want)
		}
	}
}

// Each size problem names the limit it breaks, and for a task, the way out.
// A task above split_lines is not offered a reason, because a reason cannot
// clear it.
func TestSizeBudgetMessages(t *testing.T) {
	d := plan.DefaultLimits()
	lowered := func(p *plan.Plan) {
		p.Limits.TaskLines, p.Limits.SplitLines = 60, 90
		p.Tasks[1].Size.Lines = 40
	}
	cases := []struct {
		name   string
		mutate func(p *plan.Plan)
		want   string
	}{
		{"task above task_lines", func(p *plan.Plan) {
			lowered(p)
			p.Tasks[0].Size.Lines = 70
		}, "rule 6: A1 : size.lines 70 is above limits.task_lines 60: split the task, or say in size.reason why it cannot be split"},
		{"task above split_lines with a reason", func(p *plan.Plan) {
			lowered(p)
			p.Tasks[0].Size = plan.Size{Lines: 95, Reason: "generated table"}
		}, "rule 6: A1 : size.lines 95 is above limits.split_lines 90: split the task"},
		{"task above split_lines without a reason", func(p *plan.Plan) {
			lowered(p)
			p.Tasks[0].Size.Lines = 95
		}, "rule 6: A1 : size.lines 95 is above limits.split_lines 90: split the task"},
		{"task_lines raised", func(p *plan.Plan) { p.Limits.TaskLines = d.TaskLines + 1 },
			fmt.Sprintf("rule 6: - : limits.task_lines %d is above the default %d: a plan may lower it, not raise it", d.TaskLines+1, d.TaskLines)},
		{"split_lines raised", func(p *plan.Plan) { p.Limits.SplitLines = d.SplitLines + 1 },
			fmt.Sprintf("rule 6: - : limits.split_lines %d is above the default %d: a plan may lower it, not raise it", d.SplitLines+1, d.SplitLines)},
		{"split_lines below task_lines", func(p *plan.Plan) { p.Limits.SplitLines = p.Limits.TaskLines - 1 },
			fmt.Sprintf("rule 6: - : limits.split_lines %d is below limits.task_lines %d", d.TaskLines-1, d.TaskLines)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validPlan()
			c.mutate(p)
			if ps := Lint(p, Options{}); len(ps) != 1 || ps[0].String() != c.want {
				t.Errorf("problems %v, want [%s]", ps, c.want)
			}
		})
	}
}

func TestSizeReasonAllowsUpToSplitLines(t *testing.T) {
	p := validPlan()
	p.Tasks[0].Size = plan.Size{Lines: p.Limits.SplitLines, Reason: "generated table"}
	if ps := Lint(p, Options{}); len(ps) != 0 {
		t.Errorf("a reason up to split_lines is clean, got %v", ps)
	}
}

// A size at task_lines needs no reason, split_lines may equal task_lines, and
// a plan that lowers both limits and keeps to them is clean.
func TestSizeBudgetAllowsTheLimits(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(p *plan.Plan)
	}{
		{"size at task_lines without a reason", func(p *plan.Plan) { p.Tasks[0].Size.Lines = p.Limits.TaskLines }},
		{"split_lines equal to task_lines", func(p *plan.Plan) { p.Limits.SplitLines = p.Limits.TaskLines }},
		{"lowered limits with sizes inside them", func(p *plan.Plan) {
			p.Limits.TaskLines, p.Limits.SplitLines = 60, 90
			p.Tasks[0].Size.Lines = 60
			p.Tasks[1].Size = plan.Size{Lines: 90, Reason: "generated table"}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validPlan()
			c.mutate(p)
			if ps := Lint(p, Options{}); len(ps) != 0 {
				t.Errorf("problems %v, want none", ps)
			}
		})
	}
}

func TestProblemString(t *testing.T) {
	cases := map[string]Problem{
		"rule 3: B1 : depends_on Z9 is not a task": {Rule: 3, Task: "B1", Msg: "depends_on Z9 is not a task"},
		"rule 2: - : done is empty":                {Rule: 2, Msg: "done is empty"},
	}
	for want, p := range cases {
		if got := p.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func writeSource(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "graph.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAdoptSource(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		wantMsg []string
	}{
		{"snake case keys", `{"tasks":[{"id":"A1","depends_on":[]},{"id":"B1","depends_on":["A1"],"merge_after":[]}]}`, nil},
		{"demo keys", `{"source":"x","tasks":[{"id":"A1","deps":[],"mergeAfter":[]},{"id":"B1","deps":["A1"],"size":300}]}`, nil},
		{"camel case keys", `{"tasks":[{"id":"A1"},{"id":"B1","dependsOn":["A1"]}]}`, nil},
		{"extra source task", `{"tasks":[{"id":"A1"},{"id":"B1","deps":["A1"]},{"id":"C1","deps":["B1"]}]}`, []string{
			"rule 8: - : task count 2 differs from the source's 3",
			"rule 8: C1 : task is in the source but not in the plan",
			"rule 8: C1 : edge C1 depends_on B1 is in the source but not in the plan",
		}},
		{"different edge", `{"tasks":[{"id":"A1","mergeAfter":["B1"]},{"id":"B1"}]}`, []string{
			"rule 8: A1 : edge A1 merge_after B1 is in the source but not in the plan",
			"rule 8: B1 : edge B1 depends_on A1 is in the plan but not in the source",
		}},
		{"bad json", `{"tasks":`, []string{"rule 8: - : adopt source"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ps := Lint(validPlan(), Options{AdoptSource: writeSource(t, c.source)})
			if len(ps) != len(c.wantMsg) {
				t.Fatalf("problems %v, want %d", ps, len(c.wantMsg))
			}
			for i, want := range c.wantMsg {
				if !strings.HasPrefix(ps[i].String(), want) {
					t.Errorf("problem %d = %q, want prefix %q", i, ps[i], want)
				}
			}
		})
	}
	ps := Lint(validPlan(), Options{AdoptSource: filepath.Join(t.TempDir(), "missing.json")})
	if got := firedRules(ps); !reflect.DeepEqual(got, []int{8}) {
		t.Errorf("missing source: %v", ps)
	}
}

// A task with a source stands for that source task, so the parts of a split
// task count once and an edge between two parts drops out.
func TestAdoptSplit(t *testing.T) {
	src := `{"tasks":[{"id":"T1"},{"id":"T2","depends_on":["T1"]}]}`
	cases := []struct {
		name, source, plan string
		want               []string
	}{
		{"parts keep the source id", src,
			`{"tasks":[{"id":"T10","source":"T1"},{"id":"T11","source":"T1","depends_on":["T10"]},{"id":"T2","depends_on":["T11"]}]}`, nil},
		{"dependent loses its edge", src,
			`{"tasks":[{"id":"T10","source":"T1"},{"id":"T11","source":"T1","depends_on":["T10"]},{"id":"T2"}]}`,
			[]string{"rule 8: T2 : edge T2 depends_on T1 is in the source but not in the plan"}},
		{"first part takes the source dependencies", `{"tasks":[{"id":"T0"},{"id":"T1","depends_on":["T0"],"merge_after":["T0"]}]}`,
			`{"tasks":[{"id":"T0"},{"id":"T10","source":"T1","depends_on":["T0"],"merge_after":["T0"]},{"id":"T11","source":"T1","depends_on":["T10"]}]}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := plan.Load(writeSource(t, c.plan))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, pr := range Lint(p, Options{AdoptSource: writeSource(t, c.source)}) {
				if pr.Rule == 8 {
					got = append(got, pr.String())
				}
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("rule 8 problems %q, want %q", got, c.want)
			}
		})
	}
}
