package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"source/internal/charter"
	"source/internal/exit"
	"source/internal/item"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["round"] = subcommands("round", roundSubs) }

// roundSubs maps each round subcommand to its handler. Later round commands add to it.
var roundSubs = map[string]func(c *Ctx, args []string) error{"open": roundOpen, "close": roundClose}

// roundOpen renews brain, then under the state mutex needs a charter phase, no open round, and a
// newest version that no round checked (R13), fixes the picks at the first round of the phase (R12),
// and opens round Round+1 on that version through openChecks. Each refusal is exit 2 and writes nothing.
func roundOpen(c *Ctx, args []string) error {
	fs := newFlagSet("round open")
	holder := fs.String("holder", "", "the brain lease holder")
	pick := fs.String("pick", "", "the personas with no phases to check, comma separated")
	if pos, err := parseFlags(fs, args); err != nil {
		return err
	} else if len(pos) != 0 {
		return exit.New(exit.Usage, "usage: monet-source round open --holder <session> [--pick <id,...>]")
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
		k := s.Versions[s.Phase]
		switch {
		case !slices.Contains(ch.Phases, s.Phase):
			return exit.New(exit.Refused, "round open: phase %s takes no round", s.Phase)
		case s.RoundOpen:
			return exit.New(exit.Refused, "round open: round %d is open", s.Round)
		case k == 0:
			return exit.New(exit.Refused, "round open: %s has no version", s.Phase)
		case s.Checked[s.Phase] == k:
			return exit.New(exit.Refused, "round open: v%d of %s was already checked", k, s.Phase)
		}
		if s.Checked[s.Phase] == 0 && *pick != "" {
			if s.Picks == nil {
				s.Picks = map[string][]string{}
			}
			s.Picks[s.Phase] = strings.Split(*pick, ",")
		}
		cards, err := round.Panel(ch, s.Phase, s.Picks[s.Phase])
		if err != nil {
			return exit.New(exit.Refused, "round open: %v", err)
		}
		r := &round.Round{N: s.Round + 1, Phase: s.Phase, Version: k, Versions: []string{d.Path("artifacts", s.Phase, "v"+strconv.Itoa(k))}}
		return openChecks(c, d, s, ch, r, cards)
	})
}

// openChecks needs a brief for each card (exit 2), writes the brief of each check of r, saves
// round.json, sets Round, RoundOpen, and Checked[r.Phase], and prints one check line per check (R15).
// It runs inside state.Update, and it refuses before its first write.
func openChecks(c *Ctx, d store.Dir, s *state.State, ch *charter.Charter, r *round.Round, cards []charter.Card) error {
	for _, p := range cards {
		if _, ok := s.Briefs[p.ID]; !ok {
			return exit.New(exit.Refused, "persona %s has no brief", p.ID)
		}
	}
	md, err := os.ReadFile(d.CharterMD())
	if err != nil {
		return err
	}
	var lines strings.Builder
	for _, p := range cards {
		persona, err := os.ReadFile(filepath.Join(d.Personas(), p.ID+".md"))
		if err != nil {
			return err
		}
		id := fmt.Sprintf("%d-%s", r.N, p.ID)
		ck := round.Check{ID: id, Persona: p.ID, Weight: p.Weight, Effort: round.Effort(s.Counts[r.Phase][p.ID], r.Final),
			Brief: round.Path(c.Run, r.N, "briefs", id+".md"), Verdict: round.Path(c.Run, r.N, "verdicts", id+".json")}
		brief := round.Brief(round.BriefInput{Check: id, Persona: p.ID, Effort: ck.Effort, Charter: string(md), PersonaBrief: string(persona),
			Verdict: ck.Verdict, Sources: p.Sources, Versions: r.Versions})
		if err := store.WriteFileAtomic(ck.Brief, []byte(brief), 0o644); err != nil {
			return err
		}
		ck.BriefHash = charter.Hash([]byte(brief))
		r.Checks = append(r.Checks, ck)
		fmt.Fprintf(&lines, "check %s %s %s %s %s\n", id, p.ID, ck.Effort, ck.Brief, ck.Verdict)
	}
	if err := round.Save(c.Run, *r); err != nil {
		return err
	}
	if s.Checked == nil {
		s.Checked = map[string]int{}
	}
	s.Round, s.RoundOpen, s.Checked[r.Phase] = r.N, true, r.Version
	_, err = fmt.Fprint(c.Stdout, lines.String())
	return err
}

// roundClose renews brain, then under the state mutex needs an open round (exit 2), sets each check result
// from its verdict file (R18), counts each block of a blocking persona, and at a count of 2 or more saves a
// draft item that holds the check (R19). It writes findings.md, sets the outcome and, for an ACCEPT of a round
// that is not final, Accepted. It records FINDINGS, then prints the outcome and one line per draft.
func roundClose(c *Ctx, args []string) error {
	fs := newFlagSet("round close")
	holder := fs.String("holder", "", "the brain lease holder")
	if pos, err := parseFlags(fs, args); err != nil {
		return err
	} else if len(pos) != 0 {
		return exit.New(exit.Usage, "usage: monet-source round close --holder <session>")
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	var r round.Round
	var drafts strings.Builder
	err := state.Update(d, func(s *state.State) (err error) {
		if !s.RoundOpen {
			return exit.New(exit.Refused, "round close: no round is open")
		} else if r, err = round.Load(c.Run, s.Round); err != nil {
			return err
		}
		entries := make([]round.Entry, len(r.Checks))
		for i := range r.Checks {
			ck := &r.Checks[i]
			v, err := round.ParseVerdict(ck.Verdict, ck.ID, ck.Persona)
			switch ck.Result = v.Verdict; {
			case errors.Is(err, os.ErrNotExist):
				ck.Result = "missing"
			case err != nil:
				ck.Result = "invalid: " + err.Error()
			case ck.Result == "block" && ck.Weight == "blocking":
				if s.Counts == nil {
					s.Counts = map[string]map[string]int{}
				}
				if s.Counts[r.Phase] == nil {
					s.Counts[r.Phase] = map[string]int{}
				}
				if s.Counts[r.Phase][ck.Persona]++; s.Counts[r.Phase][ck.Persona] >= 2 {
					s.Items++
					it := item.Item{ID: fmt.Sprintf("D%d", s.Items), Phase: r.Phase, Persona: ck.Persona, Round: r.N, Status: "draft"}
					for _, f := range v.Findings {
						if f.Severity == "blocker" {
							it.Findings = append(it.Findings, f.ID)
						}
					}
					if err := item.Save(d, it.ID, it); err != nil {
						return err
					}
					ck.Item = it.ID
					fmt.Fprintf(&drafts, "item %s draft for %s\n", it.ID, ck.Persona)
				}
			}
			entries[i] = round.Entry{Persona: ck.Persona, Weight: ck.Weight, Result: ck.Result, Findings: v.Findings}
		}
		if err := store.WriteFileAtomic(round.Path(c.Run, r.N, "findings.md"), []byte(round.Findings(r.N, entries)), 0o644); err != nil {
			return err
		}
		if r.Outcome = round.Outcome(r.Checks); r.Outcome == "ACCEPT" && !r.Final {
			if s.Accepted == nil {
				s.Accepted = map[string]int{}
			}
			s.Accepted[r.Phase] = r.Version
		}
		s.RoundOpen = false
		return round.Save(c.Run, r)
	})
	if err == nil {
		if err = record(c, d, fmt.Sprintf("FINDINGS %d %s", r.N, r.Outcome)); err == nil {
			_, err = fmt.Fprint(c.Stdout, r.Outcome+"\n"+drafts.String())
		}
	}
	return err
}
