package main

import (
	"errors"
	"os"
	"time"

	"source/internal/state"
	"source/internal/store"
)

// silenceAfter is how long a run goes with no page write, or the developer with no beat, before watch says so.
const silenceAfter = 60 * time.Minute

// dueEvents returns REPORT_DUE <start> once silenceAfter has passed since the last page write, or else the seal,
// then DEV_SILENT <dev> <last beat> once silenceAfter has passed since the last beat (R31, R32). REPORT_DUE needs
// the seal, DEV_SILENT needs a beat, and neither comes after delivery. Each time is RFC 3339 UTC with nanoseconds,
// so each write or beat is a new key.
func dueEvents(s state.State, now time.Time) (lines []string) {
	if s.Phase == phaseDelivered {
		return nil
	}
	if s.Sealed != nil {
		start := s.Sealed.At
		if s.LastReport != nil {
			start = *s.LastReport
		}
		if !now.Before(start.Add(silenceAfter)) {
			lines = append(lines, "REPORT_DUE "+start.UTC().Format(time.RFC3339Nano))
		}
	}
	if s.LastBeat != nil && !now.Before(s.LastBeat.Add(silenceAfter)) {
		lines = append(lines, "DEV_SILENT "+s.Dev+" "+s.LastBeat.UTC().Format(time.RFC3339Nano))
	}
	return lines
}

// recordDue records each line of dueEvents at c.Now through recordOnce, so each key logs once (R3).
// A run with no state.json has no clock to check.
func recordDue(c *Ctx, d store.Dir) error {
	if _, err := os.Lstat(d.State()); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	s, err := state.Load(d)
	if err != nil {
		return err
	}
	for _, l := range dueEvents(s, c.Now()) {
		if _, err := recordOnce(c, d, l); err != nil {
			return err
		}
	}
	return nil
}
