// Package events keeps <run>/events.tsv: one row per event line, written once,
// and read per role from a row index so each watcher sees each line once.
package events

import (
	"fmt"
	"strings"
	"time"

	"source/internal/lock"
	"source/internal/store"
)

// Header is the header of events.tsv. The key is the whole event line.
var Header = []string{"key", "at"}

// Routes maps the first word of an event line to the roles that read it
// (design 5.9, step 1).
var Routes = map[string][]string{
	"VERSION": {"brain"}, "RESPONSE": {"brain"}, "ITEM": {"brain"},
	"DEV_RELAYED": {"brain"}, "DEV_SILENT": {"brain"}, "REPORT_DUE": {"brain"},
	"FINDINGS": {"dev"}, "RULING": {"dev"},
	"PHASE": {"brain", "dev"}, "WALL": {"brain", "dev"},
	"CLEARED": {"brain", "dev"}, "DELIVERED": {"brain", "dev"},
}

// Emit appends [line, now] to events.tsv under <run>/locks/events.mutex,
// unless a row already holds the sanitized line (store.SanitizeCell) as its
// key. It reports whether it appended.
func Emit(d store.Dir, line string, now time.Time) (bool, error) {
	key := store.SanitizeCell(line)
	appended := false
	err := lock.WithMutex(d.Path("locks", "events.mutex"), func() error {
		rows, err := store.ReadTSV(d.Events(), Header)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r[0] == key {
				return nil
			}
		}
		appended = true
		return store.AppendTSV(d.Events(), Header, []string{line, now.UTC().Format(time.RFC3339Nano)})
	})
	return appended && err == nil, err
}

// For returns the lines of the rows from index from on whose first word routes
// to role, and the row count as next, the cursor for the following call.
func For(d store.Dir, role string, from int) (lines []string, next int, err error) {
	if role != "brain" && role != "dev" {
		return nil, 0, fmt.Errorf("role %q: want brain or dev", role)
	}
	rows, err := store.ReadTSV(d.Events(), Header)
	if err != nil {
		return nil, 0, err
	}
	if from < 0 || from > len(rows) {
		return nil, 0, fmt.Errorf("events row %d: want 0 to %d", from, len(rows))
	}
	for _, r := range rows[from:] {
		word, _, _ := strings.Cut(r[0], " ")
		for _, got := range Routes[word] {
			if got == role {
				lines = append(lines, r[0])
			}
		}
	}
	return lines, len(rows), nil
}
