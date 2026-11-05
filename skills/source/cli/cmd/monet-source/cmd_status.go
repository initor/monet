package main

import (
	"fmt"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/lock"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["status"] = cmdStatus }

// cmdStatus prints the run, phase, charter, brain and dev leases, and events in 6 lines (5.9). It takes no lease.
func cmdStatus(c *Ctx, args []string) error {
	if len(args) != 0 {
		return exit.New(exit.Usage, "usage: monet-source status")
	}
	d := store.Dir{Root: c.Run}
	s, err := state.Load(d)
	if err != nil {
		return err
	}
	out := "run " + s.Slug + " at " + c.Run + "\nphase " + firstSet(s.Phase, "kickoff") + "\ncharter not sealed\n"
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
