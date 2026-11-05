// Package charter holds the charter of a run and loads it strictly.
package charter

import (
	"encoding/json"
	"errors"
	"os"

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
