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
