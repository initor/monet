package main

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"source/internal/exit"
	"source/internal/item"
	"source/internal/state"
	"source/internal/store"
	"source/internal/words"
)

func init() { commands["item"] = subcommands("item", itemSubs) }

// itemSubs maps each item subcommand to its handler. Later item commands add to it.
var itemSubs = map[string]func(c *Ctx, args []string) error{"open": itemOpen, "rule": itemRule}

// itemID is the form of a decision item id, so an id never names a path outside <run>/items or a wall.
var itemID = regexp.MustCompile(`^D[1-9][0-9]*$`)

// loadItem loads the decision item id. An id outside the itemID form or with no file is exit 2.
func loadItem(d store.Dir, id string) (it item.Item, err error) {
	err = os.ErrNotExist
	if itemID.MatchString(id) {
		err = item.Load(d, id, &it)
	}
	if errors.Is(err, os.ErrNotExist) {
		return it, exit.New(exit.Refused, "item %s: no such item", id)
	}
	return it, err
}

// opening is the stdin of item open.
type opening struct {
	Default string `json:"default"`
	WayBack string `json:"way_back"`
	Affects string `json:"affects"`
}

// itemOpen renews brain, then under the state mutex needs the item, its draft status, a strict stdin,
// and a non-blank way back. Each refusal is exit 2 and writes nothing. It saves the item as open with
// the three fields and records ITEM <id>.
func itemOpen(c *Ctx, args []string) error {
	fs := newFlagSet("item open")
	holder := fs.String("holder", "", "the brain lease holder")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	} else if len(pos) != 1 {
		return exit.New(exit.Usage, "usage: monet-source item open <id> --holder <session>")
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	id, d := pos[0], store.Dir{Root: c.Run}
	if err := state.Update(d, func(*state.State) error {
		it, err := loadItem(d, id)
		if err != nil {
			return err
		} else if it.Status != "draft" {
			return exit.New(exit.Refused, "item %s is %s", id, it.Status)
		}
		var o opening
		if err := decodeStdin(c, &o); err != nil {
			return err
		} else if strings.TrimSpace(o.WayBack) == "" {
			return exit.New(exit.Refused, "item %s: no way back", id)
		}
		it.Status, it.Default, it.WayBack, it.Affects = "open", o.Default, o.WayBack, o.Affects
		return item.Save(d, id, it)
	}); err != nil {
		return err
	}
	return record(c, d, "ITEM "+id)
}

// itemRule needs --pick default or other (exit 64, before the lease) and renews brain. Under the state mutex it needs
// a run that is not delivered, the open item <id>, and the entry --words, each exit 2 with no write. It saves the item
// as ruled with the pick and the words id, deletes the count of its persona in its phase, and with other moves the run
// to the phase of the item when the run is elsewhere (R27). It records RULING <id> <pick> <words id>, then PHASE.
func itemRule(c *Ctx, args []string) error {
	fs := newFlagSet("item rule")
	pick := fs.String("pick", "", "default or other")
	wid := fs.String("words", "", "the id of the owner words that rule the item")
	holder := fs.String("holder", "", "the brain lease holder")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	} else if len(pos) != 1 || *wid == "" || (*pick != "default" && *pick != "other") {
		return exit.New(exit.Usage, "usage: monet-source item rule <id> --pick default|other --words <words id> --holder <session>")
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	id, d, move := pos[0], store.Dir{Root: c.Run}, ""
	if err := state.Update(d, func(s *state.State) error {
		if s.Phase == phaseDelivered {
			return exit.New(exit.Refused, "item rule: the run is delivered: reopen it with /monet:source resume")
		}
		it, err := loadItem(d, id)
		if err != nil {
			return err
		} else if it.Status != "open" {
			return exit.New(exit.Refused, "item %s is %s", id, it.Status)
		}
		if _, err := words.Get(d.Words(), *wid); err != nil {
			return exit.New(exit.Refused, "%v", err)
		}
		delete(s.Counts[it.Phase], it.Persona)
		if *pick == "other" && s.Phase != it.Phase {
			move = moveTo(s, it.Phase)
		}
		it.Status, it.Pick, it.Words = "ruled", *pick, *wid
		return item.Save(d, id, it)
	}); err != nil {
		return err
	}
	if err := record(c, d, "RULING "+id+" "+*pick+" "+*wid); err != nil || move == "" {
		return err
	}
	return record(c, d, move)
}
