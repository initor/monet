package main

import (
	"strconv"

	"source/internal/charter"
	"source/internal/exit"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["accept"] = cmdAccept }

// cmdAccept renews brain, then under the state mutex needs the phase acceptance and no open round
// (exit 2), and opens final round Round+1 on the last charter phase through openChecks, with each
// blocking card of its panel at max effort on the accepted version of every charter phase (R34).
func cmdAccept(c *Ctx, args []string) error {
	fs := newFlagSet("accept")
	holder := fs.String("holder", "", "the brain lease holder")
	if pos, err := parseFlags(fs, args); err != nil {
		return err
	} else if len(pos) != 0 {
		return exit.New(exit.Usage, "usage: monet-source accept --holder <session>")
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	return state.Update(d, func(s *state.State) error {
		_, ch, err := cleanCharter(c, d)
		if err != nil {
			return err
		}
		switch {
		case s.Phase != phaseAcceptance:
			return exit.New(exit.Refused, "accept: phase %s is not acceptance", firstSet(s.Phase, "kickoff"))
		case s.RoundOpen:
			return exit.New(exit.Refused, "accept: round %d is open", s.Round)
		}
		last := ch.Phases[len(ch.Phases)-1]
		panel, err := round.Panel(ch, last, s.Picks[last])
		if err != nil {
			return exit.New(exit.Refused, "accept: %v", err)
		}
		var cards []charter.Card
		for _, p := range panel {
			if p.Weight == "blocking" {
				cards = append(cards, p)
			}
		}
		// Version is the accepted one of last, so openChecks leaves Checked[last] as it was.
		r := &round.Round{N: s.Round + 1, Phase: last, Final: true, Version: s.Accepted[last]}
		for _, p := range ch.Phases {
			r.Versions = append(r.Versions, d.Path("artifacts", p, "v"+strconv.Itoa(s.Accepted[p])))
		}
		return openChecks(c, d, s, ch, r, cards)
	})
}
