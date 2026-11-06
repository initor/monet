package main

import (
	"encoding/json"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"source/internal/charter"
	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
)

func init() {
	commands["persona"] = subcommands("persona", map[string]func(c *Ctx, args []string) error{"set": personaSet})
}

// personaSet renews brain and stores stdin as <run>/personas/<id>.md, its hash in Briefs, and the
// hash of its card in BriefCards. It needs a seal, a clean charter with card <id>, no brief for
// <id> yet (D13), and a brief that is not blank, else exit 2 and no write.
func personaSet(c *Ctx, args []string) error {
	fs := newFlagSet("persona set")
	holder := fs.String("holder", "", "the brain lease holder")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return exit.New(exit.Usage, "usage: monet-source persona set <id> --holder <session>")
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	brief, err := io.ReadAll(c.Stdin)
	if err != nil {
		return err
	}
	d, id := store.Dir{Root: c.Run}, pos[0]
	return state.Update(d, func(s *state.State) error {
		if s.Sealed == nil {
			return exit.New(exit.Refused, "charter not sealed: seal it before persona set")
		}
		_, ch, err := cleanCharter(c, d)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(ch.Personas, func(p charter.Card) bool { return p.ID == id })
		if i < 0 {
			return exit.New(exit.Refused, "persona %s is not in the charter", id)
		}
		if _, ok := s.Briefs[id]; ok {
			return exit.New(exit.Refused, "persona %s already has a brief", id)
		}
		if strings.TrimSpace(string(brief)) == "" {
			return exit.New(exit.Refused, "persona %s: the brief on stdin is empty", id)
		}
		if err := store.WriteFileAtomic(filepath.Join(d.Personas(), id+".md"), brief, 0o644); err != nil {
			return err
		}
		if s.Briefs == nil {
			s.Briefs = map[string]string{}
		}
		if s.BriefCards == nil {
			s.BriefCards = map[string]string{}
		}
		s.Briefs[id], s.BriefCards[id] = charter.Hash(brief), cardHash(ch.Personas[i])
		return nil
	})
}

// cardHash is the hash of the json.Marshal bytes of p, so amend can tell a changed card.
func cardHash(p charter.Card) string {
	b, _ := json.Marshal(p) // a Card holds only strings and string slices, so Marshal cannot fail
	return charter.Hash(b)
}

// dropChangedBriefs removes from s the brief hashes of each persona whose card in ch is gone or
// has other JSON than the card that its brief expands, and returns their ids.
func dropChangedBriefs(s *state.State, ch *charter.Charter) []string {
	cards := map[string]charter.Card{}
	for _, p := range ch.Personas {
		cards[p.ID] = p
	}
	var dropped []string
	for id := range s.Briefs {
		if cardHash(cards[id]) != s.BriefCards[id] {
			delete(s.Briefs, id)
			delete(s.BriefCards, id)
			dropped = append(dropped, id)
		}
	}
	return dropped
}
