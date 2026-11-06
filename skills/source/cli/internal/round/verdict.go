package round

import (
	"fmt"
	"slices"

	"source/internal/store"
)

// Finding is one finding of a verdict (R17).
type Finding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Problem  string `json:"problem"`
	Evidence string `json:"evidence"`
	Ask      string `json:"ask"`
}

// Verdict is the verdict file that the evaluator of one check writes (R17).
type Verdict struct {
	Persona  string    `json:"persona"`
	Check    string    `json:"check"`
	Verdict  string    `json:"verdict"`
	Findings []Finding `json:"findings"`
}

// The value sets of R17.
var (
	verdicts   = map[string]bool{"accept": true, "block": true}
	severities = map[string]bool{"blocker": true, "major": true, "minor": true}
)

// ParseVerdict decodes the verdict file at path strictly and checks it as the
// verdict of check id check by persona: the reuse rule of R18, then the rules of
// R17. On any error it returns a zero Verdict.
func ParseVerdict(path, check, persona string) (v Verdict, err error) {
	if err = store.DecodeStrict(path, &v); err != nil {
		return Verdict{}, err
	}
	bad := slices.IndexFunc(v.Findings, func(f Finding) bool { return f.ID == "" || !severities[f.Severity] })
	blocker := slices.ContainsFunc(v.Findings, func(f Finding) bool { return f.Severity == "blocker" })
	switch {
	case v.Check != check:
		return Verdict{}, fmt.Errorf("check %s: want %s", v.Check, check)
	case v.Persona != persona:
		return Verdict{}, fmt.Errorf("persona %s: want %s", v.Persona, persona)
	case !verdicts[v.Verdict]:
		return Verdict{}, fmt.Errorf("verdict: want accept or block")
	case bad >= 0 && v.Findings[bad].ID == "":
		return Verdict{}, fmt.Errorf("finding %d: no id", bad+1)
	case bad >= 0:
		return Verdict{}, fmt.Errorf("finding %s: severity: want blocker, major, or minor", v.Findings[bad].ID)
	case v.Verdict == "block" && !blocker:
		return Verdict{}, fmt.Errorf("block: no blocker finding")
	case v.Verdict == "accept" && blocker:
		return Verdict{}, fmt.Errorf("accept: holds a blocker finding")
	}
	return v, nil
}
