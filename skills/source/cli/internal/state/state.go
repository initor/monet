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
