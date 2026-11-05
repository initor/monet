// Package charter holds the charter of a run and loads it strictly.
package charter

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strings"

	"source/internal/store"
)

// Charter is the decoded charter.json of a run.
type Charter struct {
	Version     int         `json:"version"`
	Goal        string      `json:"goal"`
	Context     string      `json:"context"`
	Deliverable string      `json:"deliverable"`
	QualityBar  string      `json:"quality_bar"`
	Page        string      `json:"page"`
	Invariants  []Invariant `json:"invariants"`
	Phases      []string    `json:"phases"`
	Personas    []Card      `json:"personas"`
	Grants      []Grant     `json:"grants"`
}

// Invariant is a rule that every phase of the work keeps.
type Invariant struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Card is a persona that reviews the work in its phases.
type Card struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Weight      string   `json:"weight"`
	Sources     []string `json:"sources"`
	Phases      []string `json:"phases"`
}

// Grant is an outward step that the charter covers, with the id of its words.
type Grant struct {
	Step  string `json:"step"`
	Words string `json:"words"`
}

// Load decodes path strictly, after it refuses a top-level repo key (call 24).
func Load(path string) (*Charter, error) {
	var top struct{ Repo json.RawMessage }
	if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &top) == nil && top.Repo != nil {
		return nil, errors.New("charter: repo comes with step 2 (call 24)")
	}
	var c Charter
	if err := store.DecodeStrict(path, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

var invariantID = regexp.MustCompile(`^I[0-9]+$`)

// Lint returns the field, invariant, and phase problems of c in the order of
// the plan's A12 table. Step 1 has no implementation phase (call 24).
func Lint(c *Charter) []string {
	var out []string
	if c.Version != 1 {
		out = append(out, "version: want 1")
	}
	for _, f := range []struct{ name, v string }{
		{"goal", c.Goal}, {"context", c.Context}, {"deliverable", c.Deliverable}, {"quality_bar", c.QualityBar},
	} {
		if strings.TrimSpace(f.v) == "" {
			out = append(out, f.name+": empty")
		}
	}
	if !strings.HasPrefix(c.Page, "https://") {
		out = append(out, "page: want an https link")
	}
	ids := map[string]bool{}
	for _, inv := range c.Invariants {
		if !invariantID.MatchString(inv.ID) {
			out = append(out, "invariants: "+inv.ID+": want an id like I1")
		}
		if ids[inv.ID] {
			out = append(out, "invariants: "+inv.ID+": duplicate")
		}
		ids[inv.ID] = true
		if strings.TrimSpace(inv.Text) == "" {
			out = append(out, "invariants: "+inv.ID+": empty text")
		}
	}
	if len(c.Phases) == 0 {
		out = append(out, "phases: empty")
	}
	seen, impl := map[string]bool{}, false
	for _, p := range c.Phases {
		switch {
		case p == "implementation":
			impl = true
		case p != "research" && p != "design":
			out = append(out, "phases: "+p+": want research or design")
		case seen[p]:
			out = append(out, "phases: "+p+": duplicate")
		}
		seen[p] = true
	}
	if len(c.Phases) > 0 && c.Phases[0] != "research" {
		out = append(out, "phases: want research first")
	}
	if impl {
		out = append(out, "phases: implementation comes with step 2 (call 24)")
	}
	return out
}
