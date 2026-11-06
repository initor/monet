// Package round holds the rounds of a run, the panel of a phase, and the effort of a check.
package round

import (
	"fmt"
	"path/filepath"
	"slices"

	"source/internal/charter"
	"source/internal/store"
)

// Check is one persona check of a round.
type Check struct {
	ID        string `json:"id"`
	Persona   string `json:"persona"`
	Weight    string `json:"weight"`
	Effort    string `json:"effort"`
	Brief     string `json:"brief"`
	BriefHash string `json:"brief_sha256"`
	Verdict   string `json:"verdict"`
	Result    string `json:"result,omitempty"`
	Item      string `json:"item,omitempty"`
}

// Round is the round.json of one round.
type Round struct {
	N        int      `json:"round"`
	Phase    string   `json:"phase"`
	Final    bool     `json:"final,omitempty"`
	Version  int      `json:"version"`
	Versions []string `json:"versions"`
	Checks   []Check  `json:"checks"`
	Outcome  string   `json:"outcome,omitempty"`
}

// Path joins parts under <run>/rounds/<n>.
func Path(run string, n int, parts ...string) string {
	return filepath.Join(append([]string{run, "rounds", fmt.Sprint(n)}, parts...)...)
}

// Load decodes the round.json of round n strictly.
func Load(run string, n int) (r Round, err error) {
	err = store.DecodeStrict(Path(run, n, "round.json"), &r)
	return r, err
}

// Save writes r to the round.json of round r.N atomically.
func Save(run string, r Round) error { return store.WriteJSON(Path(run, r.N, "round.json"), r) }

// Panel returns, in charter order, each card whose phases hold phase, plus each
// card with no phases whose id is in picks. A pick of any other id is an error.
func Panel(ch *charter.Charter, phase string, picks []string) ([]charter.Card, error) {
	var out []charter.Card
	for _, c := range ch.Personas {
		if slices.Contains(c.Phases, phase) || slices.Contains(picks, c.ID) {
			out = append(out, c)
		}
	}
	for _, p := range picks {
		if !slices.ContainsFunc(out, func(c charter.Card) bool { return c.ID == p && len(c.Phases) == 0 }) {
			return nil, fmt.Errorf("pick %s: not a persona with no phases", p)
		}
	}
	return out, nil
}

// Effort returns max for a final check or a second check (a count of 1), else medium (R16).
func Effort(blocks int, final bool) string {
	if final || blocks == 1 {
		return "max"
	}
	return "medium"
}
