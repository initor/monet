// Package item holds the decision items and walls of a run, one file each in <run>/items.
package item

import (
	"path/filepath"
	"slices"
	"strconv"

	"source/internal/store"
)

// Item is a decision item D<n>. Status is draft, open, or ruled.
type Item struct {
	ID       string   `json:"id"`
	Phase    string   `json:"phase"`
	Persona  string   `json:"persona"`
	Round    int      `json:"round"`
	Findings []string `json:"findings"`
	Status   string   `json:"status"`
	Default  string   `json:"default,omitempty"`
	WayBack  string   `json:"way_back,omitempty"`
	Affects  string   `json:"affects,omitempty"`
	Pick     string   `json:"pick,omitempty"`
	Words    string   `json:"words,omitempty"`
}

// Wall is a wall W<n>. Status is open or cleared.
type Wall struct {
	ID       string `json:"id"`
	By       string `json:"by"`
	Question string `json:"question"`
	Default  string `json:"default"`
	Blocks   string `json:"blocks"`
	Status   string `json:"status"`
	Words    string `json:"words,omitempty"`
}

// Path is <run>/items/<id>.json. Load decodes that file strictly into v, and Save writes v to it atomically.
func Path(d store.Dir, id string) string       { return d.Path("items", id+".json") }
func Load(d store.Dir, id string, v any) error { return store.DecodeStrict(Path(d, id), v) }
func Save(d store.Dir, id string, v any) error { return store.WriteJSON(Path(d, id), v) }

// Waiting returns the items that are not ruled and the walls that are not cleared, each in numeric id order.
func Waiting(d store.Dir) (items, walls []string, err error) {
	paths, err := filepath.Glob(d.Path("items", "[DW]*.json"))
	for _, p := range paths {
		id := filepath.Base(p[:len(p)-len(".json")])
		it, w := Item{}, Wall{}
		v, done, out := any(&it), "ruled", &items
		if id[0] == 'W' {
			v, done, out = &w, "cleared", &walls
		}
		if err := Load(d, id, v); err != nil {
			return nil, nil, err
		}
		if it.Status+w.Status != done {
			*out = append(*out, id)
		}
	}
	by := func(a, b string) int { x, _ := strconv.Atoi(a[1:]); y, _ := strconv.Atoi(b[1:]); return x - y }
	slices.SortFunc(items, by)
	slices.SortFunc(walls, by)
	return items, walls, err
}
