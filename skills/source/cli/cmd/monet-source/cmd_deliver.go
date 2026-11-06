package main

import (
	"fmt"
	"strings"

	"source/internal/exit"
	"source/internal/item"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["deliver"] = cmdDeliver }

// cmdDeliver renews brain, then under the state mutex needs the phase acceptance, no open round, and a
// newest round that is final with ACCEPT (exit 2), reads the waiting items and walls, which do not stop it
// (R30), and sets the phase to delivered. It records DELIVERED <round>, prints each folder of that round and
// each waiting id, and releases no lease (R36).
func cmdDeliver(c *Ctx, args []string) error {
	fs := newFlagSet("deliver")
	holder := fs.String("holder", "", "the brain lease holder")
	if pos, err := parseFlags(fs, args); err != nil {
		return err
	} else if len(pos) != 0 {
		return exit.New(exit.Usage, "usage: monet-source deliver --holder <session>")
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	var r round.Round
	var items, walls []string
	if err := state.Update(d, func(s *state.State) (err error) {
		if s.Phase == phaseAcceptance && !s.RoundOpen {
			if r, err = round.Load(c.Run, s.Round); err != nil {
				return err
			}
		}
		if !r.Final || r.Outcome != "ACCEPT" {
			return exit.New(exit.Refused, "deliver: no accepted final check")
		}
		if items, walls, err = item.Waiting(d); err != nil {
			return err
		}
		s.Phase = phaseDelivered
		return nil
	}); err != nil {
		return err
	}
	if err := record(c, d, fmt.Sprintf("DELIVERED %d", r.N)); err != nil {
		return err
	}
	var b strings.Builder
	for _, v := range r.Versions {
		fmt.Fprintf(&b, "deliverable %s\n", v)
	}
	for _, id := range append(items, walls...) {
		fmt.Fprintf(&b, "waiting %s\n", id)
	}
	_, err := fmt.Fprint(c.Stdout, b.String())
	return err
}
