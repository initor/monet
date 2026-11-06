package main

import (
	"time"

	"source/internal/events"
	"source/internal/store"
)

// record emits line as an event, then appends [now, line] to log.tsv, with now
// read once from c.Now and written in UTC. A failed emit writes no log row.
func record(c *Ctx, d store.Dir, line string) error {
	now := c.Now()
	if _, err := events.Emit(d, line, now); err != nil {
		return err
	}
	return store.AppendTSV(d.Log(), logHeader, []string{now.UTC().Format(time.RFC3339Nano), line})
}

// recordOnce is record for a line that must log once: it appends the log.tsv row
// only when the emit appended the event, and reports whether it did. An error
// from the log.tsv append comes back with true, since the event did land.
func recordOnce(c *Ctx, d store.Dir, line string) (bool, error) {
	now := c.Now()
	if ok, err := events.Emit(d, line, now); !ok || err != nil {
		return false, err
	}
	return true, store.AppendTSV(d.Log(), logHeader, []string{now.UTC().Format(time.RFC3339Nano), line})
}
