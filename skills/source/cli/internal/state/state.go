// Package state reads and updates <run>/state.json, one writer at a time.
package state

import (
	"errors"
	"os"
	"time"

	"source/internal/lock"
	"source/internal/store"
)

// State is the content of state.json.
type State struct {
	Version int       `json:"version"`
	Slug    string    `json:"slug"`
	Created time.Time `json:"created"`
	Phase   string    `json:"phase"`
	Sealed  *Seal     `json:"sealed,omitempty"`
	Amends  []Seal    `json:"amends,omitempty"` // one entry for each charter amend, oldest first
	// Briefs maps a persona id to the hash of its brief in <run>/personas/<id>.md, and
	// BriefCards maps it to the hash of the JSON of the card that the brief expands.
	Briefs     map[string]string `json:"briefs,omitempty"`
	BriefCards map[string]string `json:"brief_cards,omitempty"`
	// The loop: the dev session and beat, versions and blocks per phase, rounds, picks, id counters, moves.
	Dev       string                    `json:"dev,omitempty"`
	LastBeat  *time.Time                `json:"last_beat,omitempty"`
	Versions  map[string]int            `json:"versions,omitempty"`
	Checked   map[string]int            `json:"checked,omitempty"`
	Accepted  map[string]int            `json:"accepted,omitempty"`
	Round     int                       `json:"round,omitempty"`
	RoundOpen bool                      `json:"round_open,omitempty"`
	Counts    map[string]map[string]int `json:"counts,omitempty"`
	Picks     map[string][]string       `json:"picks,omitempty"`
	Items     int                       `json:"items,omitempty"`
	Walls     int                       `json:"walls,omitempty"`
	Moves     int                       `json:"moves,omitempty"`
}

// Seal is the confirmation of the charter: the id of the owner words, when the
// seal was made, and the hash of charter.json that charter.md shows.
type Seal struct {
	Words string    `json:"words"`
	At    time.Time `json:"at"`
	Hash  string    `json:"hash"`
}

// Load decodes state.json strictly. A missing file means init has not run.
func Load(d store.Dir) (s State, err error) {
	if err = store.DecodeStrict(d.State(), &s); errors.Is(err, os.ErrNotExist) {
		err = errors.New("no run state: run init first")
	}
	return s, err
}

// Update loads the state, calls fn, and writes it atomically under <run>/locks/state.mutex.
func Update(d store.Dir, fn func(*State) error) error {
	return lock.WithMutex(d.Path("locks", "state.mutex"), func() error {
		s, err := Load(d)
		if err == nil {
			if err = fn(&s); err == nil {
				err = store.WriteJSON(d.State(), s)
			}
		}
		return err
	})
}
