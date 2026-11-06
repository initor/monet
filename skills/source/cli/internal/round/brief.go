package round

import (
	"fmt"
	"strings"
)

// VerdictSchema is the JSON form of a verdict (R17), with one example finding.
const VerdictSchema = `{
  "persona": "<persona>",
  "check": "<check>",
  "verdict": "block",
  "findings": [
    {"id": "F1", "severity": "blocker", "problem": "<what is wrong>", "evidence": "<a file and line, or a quote>", "ask": "<what would fix it>"}
  ]
}`

// BriefInput holds the allowlisted inputs of one evaluator brief (D8, R15, R16).
type BriefInput struct {
	Check, Persona, Effort, Charter, PersonaBrief, Verdict, Response string
	Sources, Versions                                                []string
	Blockers                                                         []Finding
}

// Brief renders the brief of one check from in alone, in a fixed section order. The blocker
// findings and the answer of the developer appear only for a second check (R16).
func Brief(in BriefInput) string {
	blocks := []string{
		fmt.Sprintf("# Check %s by %s at %s effort\n", in.Check, in.Persona, in.Effort),
		"## Charter\n", text(in.Charter),
		"## Persona\n", text(in.PersonaBrief),
		"## Sources\n", "Read these files. Do not change them.\n", list(in.Sources),
		"## Version\n", list(in.Versions),
		"## Verdict\n", fmt.Sprintf("Write your verdict as JSON to %s, in this form:\n", in.Verdict),
		"```json\n" + VerdictSchema + "\n```\n",
		"Set check and persona to the values of this check. verdict is accept or block, and severity is " +
			"blocker, major, or minor. A block needs a blocker finding, and an accept holds none.\n",
	}
	if len(in.Blockers) > 0 || in.Response != "" {
		blocks = append(blocks, "## Your blocker findings\n")
		for _, f := range in.Blockers {
			blocks = append(blocks, RenderFinding(f))
		}
		blocks = append(blocks, "## The answer of the developer\n", text(in.Response))
	}
	return strings.Join(blocks, "\n")
}

// text returns s byte for byte, with a final newline when s has none.
func text(s string) string {
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

// list returns one "- <item>" line for each item.
func list(items []string) string {
	var b strings.Builder
	for _, s := range items {
		b.WriteString("- " + s + "\n")
	}
	return b.String()
}
