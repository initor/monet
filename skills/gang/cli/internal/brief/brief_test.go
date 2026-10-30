package brief

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/notes"
	"gang/internal/plan"
	"gang/internal/store"
)

func fixturePlan() *plan.Plan {
	p := &plan.Plan{
		Version:      1,
		Slug:         "toy",
		Repo:         plan.Repo{Base: "main", Integration: "integration"},
		Commands:     plan.Commands{Gate: []string{"make check"}, Health: []string{"make check", "make e2e"}},
		Forbidden:    []string{"git stash", "force-push"},
		Requirements: []plan.Requirement{{ID: "ENG-25", Source: "spec/10-engineering.md:670"}, {ID: "ENG-26", Source: "spec/10-engineering.md:702"}},
		Tasks: []plan.Task{{
			ID:           "B2",
			Title:        "Parse the config file",
			Goal:         "Load config.yaml into a typed Config and reject unknown keys.",
			Scope:        plan.Scope{Write: []string{"internal/config/**"}, Shared: []string{"Makefile"}},
			Peers:        []string{"C3"},
			DependsOn:    []string{"A1"},
			MergeAfter:   []string{},
			Requirements: []string{"ENG-25", "ENG-27"},
			Interfaces:   plan.Interfaces{Consumes: []string{"file:config.yaml"}, Produces: []string{"py:config.load"}},
			Acceptance:   []string{"Unknown keys fail with exit 1.", "Defaults fill missing keys."},
			Verify:       plan.Verify{TestsFirst: []string{"TestLoadRejectsUnknownKeys"}, Commands: []string{"go test ./internal/config/..."}},
			Size:         plan.Size{Lines: 300, Reason: "schema and loader cannot be split"},
			TimeboxMin:   45,
			Forbidden:    []string{"git stash", "edit go.mod"},
			Risk:         "high",
			Wave:         2,
		}},
	}
	p.Normalize()
	return p
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The golden file is Contract 5 written out by hand for the fixture above.
func TestRenderGolden(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	for _, o := range []string{"Run make check before every commit.", "No new dependencies."} {
		if err := notes.Add(d, o); err != nil {
			t.Fatal(err)
		}
	}
	write(t, d.Run("tasks", "B2", "owner.md"), "Ruling: keys are case sensitive.\n")
	if _, err := notes.Report(d, "B2", "", []byte("round 1 notes")); err != nil {
		t.Fatal(err)
	}
	got, err := Render(d, fixturePlan(), "B2")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "brief.golden"))
	if err != nil {
		t.Fatal(err)
	}
	got = strings.ReplaceAll(got, d.Root, "<DIR>")
	if got != string(want) {
		t.Fatalf("brief differs from testdata/brief.golden\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestRenderStopLineComesFirst(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	if err := notes.Add(d, "Keep going."); err != nil {
		t.Fatal(err)
	}
	if err := notes.Stop(d, "usage reset at 14:00"); err != nil {
		t.Fatal(err)
	}
	got, err := Render(d, fixturePlan(), "B2")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(got, "\n")
	if lines[0] != "STOP: usage reset at 14:00" || lines[1] != "TASK B2: Parse the config file" {
		t.Fatalf("first lines %q", lines[:2])
	}
	for _, want := range []string{"\nOWNER RULINGS\nnone\n", "\nPRIOR REPORT\nnone\n", "\nSTANDING\nSTOP: usage reset at 14:00\n1. Keep going.\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("brief lacks %q", want)
		}
	}
}

// Contract 0 rule 16: F01 is not in the plan, so it renders the synthetic
// repair brief, whose gate is plan.commands.health. The golden file is
// written by hand from the rule.
func TestRenderRepairBrief(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	got, err := Render(d, fixturePlan(), "F01")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "repair.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("brief differs from testdata/repair.golden\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

// The budget line takes its numbers from the plan's limits and follows
// size.Check: a reason that is blank once trimmed counts as none.
func TestRenderStatesTheBudget(t *testing.T) {
	const rest = ", tests included, lock and binary files not counted"
	for _, c := range []struct {
		name, reason string
		task, split  int
		want         string
	}{
		{"no reason", "", 100, 300, "budget: 100 changed lines" + rest},
		{"blank reason", " \t ", 100, 300, "budget: 100 changed lines" + rest},
		{"lowered task_lines", "", 60, 200, "budget: 60 changed lines" + rest},
		{"reason on one line", "one parser\nand its loader", 60, 200, "budget: up to 200 changed lines (size.reason: one parser and its loader)" + rest},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := fixturePlan()
			p.Limits.TaskLines, p.Limits.SplitLines = c.task, c.split
			p.Tasks[0].Size = plan.Size{Lines: 80, Reason: c.reason}
			got, err := Render(store.Dir{Root: t.TempDir()}, p, "B2")
			if err != nil {
				t.Fatal(err)
			}
			if want := "\nTIMEBOX\nlines: 80\n" + c.want + "\nminutes: 45\n"; !strings.Contains(got, want) {
				t.Errorf("brief lacks %q\n%s", want, got)
			}
		})
	}
}

func TestRenderWithoutStandingOrStop(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	got, err := Render(d, fixturePlan(), "B2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "TASK B2: ") || !strings.Contains(got, "\nSTANDING\nnone\n") {
		t.Fatalf("brief:\n%s", got)
	}
	for _, id := range []string{"Z9", "F1"} {
		if _, err := Render(d, fixturePlan(), id); err == nil {
			t.Fatalf("unknown task %s rendered", id)
		}
	}
}
