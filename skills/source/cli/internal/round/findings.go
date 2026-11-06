package round

import (
	"fmt"
	"strings"
)

// Entry is the section of one persona in findings.md: its check result and its findings.
type Entry struct {
	Persona, Weight, Result string
	Findings                []Finding
}

// Findings renders the findings.md of round n: a title, then one section per
// entry with each finding through RenderFinding, one blank line between blocks (R21).
func Findings(n int, entries []Entry) string {
	blocks := []string{fmt.Sprintf("# Round %d findings\n", n)}
	for _, e := range entries {
		blocks = append(blocks, fmt.Sprintf("## %s (%s): %s\n", e.Persona, e.Weight, e.Result))
		for _, f := range e.Findings {
			blocks = append(blocks, RenderFinding(f))
		}
	}
	return strings.Join(blocks, "\n")
}

// RenderFinding renders f as a heading and its problem, evidence, and ask lines, text byte for byte.
func RenderFinding(f Finding) string {
	return fmt.Sprintf("### %s (%s)\nProblem: %s\nEvidence: %s\nAsk: %s\n", f.ID, f.Severity, f.Problem, f.Evidence, f.Ask)
}

// Outcome returns ACCEPT when each blocking check has the result accept or an item, else FIX (R20).
func Outcome(checks []Check) string {
	for _, c := range checks {
		if c.Weight == "blocking" && c.Result != "accept" && c.Item == "" {
			return "FIX"
		}
	}
	return "ACCEPT"
}
