// Package charter holds the charter of a run and loads it strictly.
package charter

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"slices"
	"strconv"
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

var (
	invariantID = regexp.MustCompile(`^I[0-9]+$`)
	personaID   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// Lint returns the problems of c in the order of the plan's A12 and A13
// tables. Step 1 has no implementation phase (call 24).
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
	return append(out, lintCards(c)...)
}

// lintCards returns the persona and grant problems of c. Done needs a
// blocking persona that can accept, so a charter with none is refused (call 4).
func lintCards(c *Charter) []string {
	var out []string
	if len(c.Personas) == 0 {
		out = append(out, "personas: none")
	}
	ids, blocking := map[string]bool{}, false
	for _, p := range c.Personas {
		at := "personas: " + p.ID + ": "
		if !personaID.MatchString(p.ID) {
			out = append(out, at+"want an id like code-reviewer")
		}
		if ids[p.ID] {
			out = append(out, at+"duplicate")
		}
		ids[p.ID] = true
		if p.Weight != "blocking" && p.Weight != "advisory" {
			out = append(out, at+"weight "+p.Weight+": want blocking or advisory")
		}
		blocking = blocking || p.Weight == "blocking"
		if strings.TrimSpace(p.Description) == "" {
			out = append(out, at+"description: empty")
		}
		for _, ph := range p.Phases {
			if !slices.Contains(c.Phases, ph) {
				out = append(out, at+"phase "+ph+" is not in phases")
			}
		}
	}
	if len(c.Personas) > 0 && !blocking {
		out = append(out, "personas: no blocking persona, so no persona can accept (call 4)")
	}
	for i, g := range c.Grants {
		for _, f := range []struct{ name, v string }{{"step", g.Step}, {"words", g.Words}} {
			if strings.TrimSpace(f.v) == "" {
				out = append(out, "grants: "+strconv.Itoa(i+1)+": "+f.name+": empty")
			}
		}
	}
	return out
}
