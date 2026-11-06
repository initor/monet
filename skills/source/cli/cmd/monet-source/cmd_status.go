package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/item"
	"source/internal/lock"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["status"] = cmdStatus }

// cmdStatus prints the run, phase, round, waiting items and walls, charter, brain and dev leases, and events
// in 8 lines (5.9, R37). It takes no lease.
func cmdStatus(c *Ctx, args []string) error {
	if len(args) != 0 {
		return exit.New(exit.Usage, "usage: monet-source status")
	}
	d := store.Dir{Root: c.Run}
	s, err := state.Load(d)
	if err != nil {
		return err
	}
	sealed := "not sealed"
	if s.Sealed != nil {
		sealed = "sealed with words " + s.Sealed.Words
	}
	rl, err := roundLine(c.Run, s)
	if err != nil {
		return err
	}
	items, walls, err := item.Waiting(d)
	if err != nil {
		return err
	}
	waiting := "none"
	if ids := append(items, walls...); len(ids) > 0 {
		waiting = fmt.Sprintf("%d items, %d walls: %s", len(items), len(walls), strings.Join(ids, " "))
	}
	out := "run " + s.Slug + " at " + c.Run + "\nphase " + firstSet(s.Phase, "kickoff") + "\n" + rl + "waiting " + waiting + "\ncharter " + sealed + "\n"
	for _, name := range []string{"brain", "dev"} {
		held := "free"
		if l, ok, err := lock.Read(d.Locks(), name); err != nil {
			return err
		} else if ok && c.Now().Before(l.Expires) {
			held = l.Holder + " until " + l.Expires.UTC().Format(time.RFC3339)
		}
		out += "lease " + name + " " + held + "\n"
	}
	rows, err := store.ReadTSV(d.Events(), events.Header)
	if err != nil {
		return err
	}
	last := append([][]string{{"none"}}, rows...)[len(rows)][0] // the line of the last event, or none
	_, err = fmt.Fprintf(c.Stdout, "%sevents %d, last %s\n", out, len(rows), last)
	return err
}

// roundLine is round none before the first round, else the newest round with its state (open, ACCEPT, or
// FIX), its phase and version or final, and the block counts of its phase sorted by persona (R37).
func roundLine(run string, s state.State) (string, error) {
	if s.Round == 0 {
		return "round none\n", nil
	}
	r, err := round.Load(run, s.Round)
	if err != nil {
		return "", err
	}
	on := fmt.Sprintf("%s v%d", r.Phase, r.Version)
	if r.Final {
		on = "final"
	}
	var counts []string
	for _, p := range slices.Sorted(maps.Keys(s.Counts[r.Phase])) {
		counts = append(counts, fmt.Sprintf("%s=%d", p, s.Counts[r.Phase][p]))
	}
	return fmt.Sprintf("round %d %s on %s, counts %s\n", r.N, firstSet(r.Outcome, "open"), on, firstSet(strings.Join(counts, ","), "none")), nil
}
