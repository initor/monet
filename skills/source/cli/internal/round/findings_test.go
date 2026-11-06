package round

import (
	"os"
	"testing"
)

func TestFindingsGolden(t *testing.T) {
	got := Findings(1, []Entry{
		{Persona: "reviewer", Weight: "blocking", Result: "block", Findings: []Finding{
			{ID: "F1", Severity: "blocker", Problem: "The table drops two rows:\n- row 2, `owner`\n- row 5, **cost**",
				Evidence: "report.md:12", Ask: "Add both rows."},
			{ID: "F2", Severity: "minor", Problem: "One heading is long.", Evidence: "report.md:3", Ask: "Cut it to five words."}}},
		{Persona: "reader", Weight: "advisory", Result: "accept"}})
	want, err := os.ReadFile("testdata/findings.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("Findings differs from testdata/findings.md\n--- got\n%q\n--- want\n%q", got, want)
	}
}

func TestFindingsRoundNumber(t *testing.T) {
	if got := Findings(2, nil); got != "# Round 2 findings\n" {
		t.Errorf("Findings(2, nil) = %q", got)
	}
}

func TestOutcome(t *testing.T) {
	var states []Check
	for _, r := range []string{"accept", "block", "missing", "invalid: x"} {
		states = append(states, Check{Result: r}, Check{Result: r, Item: "D1"})
	}
	for _, a := range states {
		for _, b := range states {
			for _, c := range states {
				a.Weight, b.Weight, c.Weight = "blocking", "advisory", "blocking"
				checks := []Check{a, b, c}
				want := "ACCEPT"
				for _, k := range []Check{a, c} {
					if k.Result != "accept" && k.Item == "" {
						want = "FIX"
					}
				}
				if got := Outcome(checks); got != want {
					t.Errorf("Outcome(%+v) = %s, want %s", checks, got, want)
				}
			}
		}
	}
}
