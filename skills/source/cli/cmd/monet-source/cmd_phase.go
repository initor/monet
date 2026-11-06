package main

import (
	"fmt"
	"slices"

	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["phase"] = subcommands("phase", phaseSubs) }

// phaseSubs maps each phase subcommand to its handler. Later phase commands add to it.
var phaseSubs = map[string]func(c *Ctx, args []string) error{"next": phaseNext, "back": phaseBack}

// The phases after the charter phases: acceptance follows the last one, and delivered ends the run.
const (
	phaseAcceptance = "acceptance"
	phaseDelivered  = "delivered"
)

// moveTo deletes the counts of the phase that the run leaves, adds 1 to Moves, sets Phase to to,
// and returns the event line PHASE <Moves> <to>, so each move is a new event key (R33).
func moveTo(s *state.State, to string) string {
	delete(s.Counts, s.Phase)
	s.Moves++
	s.Phase = to
	return fmt.Sprintf("PHASE %d %s", s.Moves, to)
}

// phaseNext renews brain, then under the state mutex needs a charter phase, no open round, and an
// ACCEPT on the newest version of the phase (R32), and moves to the next charter phase, or to
// acceptance after the last one. Each refusal is exit 2 and writes nothing. It records the move after.
func phaseNext(c *Ctx, args []string) error {
	fs := newFlagSet("phase next")
	holder := fs.String("holder", "", "the brain lease holder")
	if pos, err := parseFlags(fs, args); err != nil {
		return err
	} else if len(pos) != 0 {
		return exit.New(exit.Usage, "usage: monet-source phase next --holder <session>")
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	var line string
	if err := state.Update(d, func(s *state.State) error {
		_, ch, err := cleanCharter(c, d)
		if err != nil {
			return err
		}
		i, k := slices.Index(ch.Phases, s.Phase), s.Versions[s.Phase]
		switch {
		case i < 0:
			return exit.New(exit.Refused, "phase next: phase %s is not a charter phase", firstSet(s.Phase, "kickoff"))
		case s.RoundOpen:
			return exit.New(exit.Refused, "phase next: round %d is open", s.Round)
		case k == 0 || s.Accepted[s.Phase] != k:
			return exit.New(exit.Refused, "phase next: v%d of %s has no ACCEPT", k, s.Phase)
		}
		to := phaseAcceptance
		if i+1 < len(ch.Phases) {
			to = ch.Phases[i+1]
		}
		line = moveTo(s, to)
		return nil
	}); err != nil {
		return err
	}
	return record(c, d, line)
}

// phaseBack renews brain, then under the state mutex needs <phase> to be a charter phase before the current
// one, or any charter phase from acceptance or delivered (R32), and moves there. A refusal is exit 2 and writes
// nothing. After the move it logs the run decision by brain (R23) and records the move. This is the engine part of resume (R28).
func phaseBack(c *Ctx, args []string) error {
	fs := newFlagSet("phase back")
	reason := fs.String("reason", "", "why the run goes back")
	holder := fs.String("holder", "", "the brain lease holder")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *reason == "" {
		return exit.New(exit.Usage, "usage: monet-source phase back <phase> --reason <reason> --holder <session>")
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	to, d, line := pos[0], store.Dir{Root: c.Run}, ""
	if err := state.Update(d, func(s *state.State) error {
		_, ch, err := cleanCharter(c, d)
		if err != nil {
			return err
		}
		j, i := slices.Index(ch.Phases, to), slices.Index(ch.Phases, s.Phase)
		if s.Phase == phaseAcceptance || s.Phase == phaseDelivered {
			i = len(ch.Phases)
		}
		if j < 0 || j >= i {
			return exit.New(exit.Refused, "phase back: %s is not before %s", to, firstSet(s.Phase, "kickoff"))
		}
		line = moveTo(s, to)
		return nil
	}); err != nil {
		return err
	}
	if err := logDecision(c, d, "brain", "send back to "+to, *reason); err != nil {
		return err
	}
	return record(c, d, line)
}
