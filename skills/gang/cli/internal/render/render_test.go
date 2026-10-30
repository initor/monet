package render

import (
	"strings"
	"testing"

	"gang/internal/plan"
	"gang/internal/schedule"
)

// goldenPlan exercises the escapes: a pipe in a table cell, a backtick and a
// newline in a goal, a branch override, model overrides, and notes.
func goldenPlan() *plan.Plan {
	p := &plan.Plan{
		Version:   1,
		Slug:      "demo",
		Repo:      plan.Repo{Remote: "github.com/example/demo", Base: "main", Integration: "gang-demo", GHAccount: "example"},
		Commands:  plan.Commands{Gate: []string{"make check"}, Health: []string{"make check", "make e2e"}, WaveGate: []string{"make smoke | tee smoke.log"}},
		Env:       map[string]string{"KUBECONFIG": "./.kube/config"},
		Locks:     []string{"db"},
		Models:    map[string]plan.Role{"review": {Model: "sonnet"}},
		Forbidden: []string{"git stash", "force-push"},
		Done:      "every task is on main",
		Requirements: []plan.Requirement{
			{ID: "REQ-1", Source: "spec.md:10"},
			{ID: "REQ-2", Source: "spec.md:20"},
		},
		Tasks: []plan.Task{
			{
				ID: "A1", Title: "alpha parser", Goal: "Parse `alpha` files.\nReturn a tree.",
				Scope:        plan.Scope{Write: []string{"alpha/**"}, Shared: []string{"docs/index.md"}},
				Requirements: []string{"REQ-1"},
				Interfaces:   plan.Interfaces{Produces: []string{"py:alpha.parse"}},
				Acceptance:   []string{"parse returns a tree"},
				Verify:       plan.Verify{TestsFirst: []string{"test:test_parse"}},
				Size:         plan.Size{Lines: 300, Reason: "generated parser tables"}, TimeboxMin: 40, Wave: 1,
			},
			{
				ID: "B1", Title: "beta cli", Goal: "Add the beta command.", Branch: "feat/beta",
				Scope:        plan.Scope{Write: []string{"beta/**"}},
				DependsOn:    []string{"A1"},
				Requirements: []string{"REQ-2"},
				Interfaces:   plan.Interfaces{Consumes: []string{"py:alpha.parse"}},
				Acceptance:   []string{"beta --help exits 0", "beta prints the tree"},
				Verify:       plan.Verify{TestsFirst: []string{"test:test_beta"}, Commands: []string{"make beta"}},
				Model:        map[string]plan.Role{"implement": {Effort: "xhigh"}},
				Notes:        "Split from A1 at review.",
				Size:         plan.Size{Lines: 150}, TimeboxMin: 30, Locks: []string{"db"}, Risk: "high", Wave: 2,
			},
		},
	}
	p.Normalize()
	return p
}

// Estimates: A1 40 min, B1 30 min, in series. Makespan 70, so 56-105.
const golden = "# Plan: demo\n" +
	"\n" +
	"Rendered from plan.json by `gang render`. Do not edit by hand.\n" +
	"\n" +
	"## Run config\n" +
	"\n" +
	"| Field | Value |\n" +
	"|---|---|\n" +
	"| Remote | `github.com/example/demo` |\n" +
	"| Base | `main` |\n" +
	"| Integration | `gang-demo` |\n" +
	"| GitHub account | `example` |\n" +
	"| Wave title | `chore(gang): land wave {k} of {slug}` |\n" +
	"| Gate | `make check` |\n" +
	"| Health | `make check`, `make e2e` |\n" +
	"| Wave gate | `make smoke \\| tee smoke.log` |\n" +
	"| Env | `KUBECONFIG=./.kube/config` |\n" +
	"| Locks | `db` |\n" +
	"| Limits | task_lines 100, split_lines 300, fix_rounds 3, repair_tries 2, blocked_stop 3, null_stop 6, retries 2, gate_slots 3, gate_timeout_min 60, stall_min 60, max_worktrees 400 |\n" +
	"| Models | review=sonnet/- |\n" +
	"| Forbidden | `git stash`, `force-push` |\n" +
	"| Done | every task is on main |\n" +
	"\n" +
	"## Waves\n" +
	"\n" +
	"| Wave | Tasks |\n" +
	"|---|---|\n" +
	"| 1 | A1 |\n" +
	"| 2 | B1 |\n" +
	"\n" +
	"Critical chain: A1 > B1\n" +
	"\n" +
	"Estimated range: 56-105 min\n" +
	"\n" +
	"## A1: alpha parser\n" +
	"\n" +
	"- Goal: Parse `alpha` files. Return a tree.\n" +
	"- Branch: `gang/A1`\n" +
	"- Wave: 1\n" +
	"- Risk: low\n" +
	"- Size: 300 lines (size.reason: generated parser tables), timebox 40 min\n" +
	"- Scope write: `alpha/**`\n" +
	"- Scope shared: `docs/index.md`\n" +
	"- Peers: none\n" +
	"- Depends on: none\n" +
	"- Merge after: none\n" +
	"- Requirements: REQ-1 (`spec.md:10`)\n" +
	"- Consumes: none\n" +
	"- Produces: `py:alpha.parse`\n" +
	"- Verify commands: none\n" +
	"- Forbidden: none\n" +
	"- Locks: none\n" +
	"- Model: none\n" +
	"\n" +
	"Acceptance:\n" +
	"\n" +
	"- parse returns a tree\n" +
	"\n" +
	"Tests first:\n" +
	"\n" +
	"- `test:test_parse`\n" +
	"\n" +
	"## B1: beta cli\n" +
	"\n" +
	"- Goal: Add the beta command.\n" +
	"- Branch: `feat/beta`\n" +
	"- Wave: 2\n" +
	"- Risk: high\n" +
	"- Size: 150 lines, timebox 30 min\n" +
	"- Scope write: `beta/**`\n" +
	"- Scope shared: none\n" +
	"- Peers: none\n" +
	"- Depends on: A1\n" +
	"- Merge after: none\n" +
	"- Requirements: REQ-2 (`spec.md:20`)\n" +
	"- Consumes: `py:alpha.parse`\n" +
	"- Produces: none\n" +
	"- Verify commands: `make beta`\n" +
	"- Forbidden: none\n" +
	"- Locks: `db`\n" +
	"- Model: implement=-/xhigh\n" +
	"- Notes: Split from A1 at review.\n" +
	"\n" +
	"Acceptance:\n" +
	"\n" +
	"- beta --help exits 0\n" +
	"- beta prints the tree\n" +
	"\n" +
	"Tests first:\n" +
	"\n" +
	"- `test:test_beta`\n"

func TestMarkdownGolden(t *testing.T) {
	p := goldenPlan()
	s, err := schedule.Run(p, schedule.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := Markdown(p, s); got != golden {
		t.Errorf("Markdown mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, golden)
	}
}

func TestMarkdownWithoutSchedule(t *testing.T) {
	got := Markdown(goldenPlan(), nil)
	if strings.Contains(got, "Critical chain") || strings.Contains(got, "Estimated range") {
		t.Error("a nil schedule still rendered the chain or the range")
	}
	want := strings.Replace(golden, "\nCritical chain: A1 > B1\n\nEstimated range: 56-105 min\n", "", 1)
	if got != want {
		t.Errorf("nil schedule output differs from the golden text minus the schedule lines:\n%s", got)
	}
}

func TestMarkdownUnscheduledAndCodeWithBacktick(t *testing.T) {
	p := goldenPlan()
	p.Tasks[0].Wave = 0
	p.Commands.Gate = []string{"echo `date`"}
	got := Markdown(p, nil)
	if !strings.Contains(got, "| unscheduled | A1 |\n") {
		t.Error("wave 0 is not listed as unscheduled")
	}
	if !strings.Contains(got, "| Gate | `` echo `date` `` |\n") {
		t.Error("a command holding a backtick is not wrapped in double backticks")
	}
}

// A blank size.reason counts as none, as in size.Check, and a reason with a
// newline stays on the one line of its bullet.
func TestMarkdownSizeReason(t *testing.T) {
	p := goldenPlan()
	p.Tasks[0].Size.Reason = "two\nlines"
	p.Tasks[1].Size.Reason = "   "
	got := Markdown(p, nil)
	for _, want := range []string{
		"- Size: 300 lines (size.reason: two lines), timebox 40 min\n",
		"- Size: 150 lines, timebox 30 min\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plan.md lacks %q", want)
		}
	}
}
