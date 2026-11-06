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
)

func init() { commands["item"] = subcommands("item", itemSubs) }

// itemSubs maps each item subcommand to its handler. Later item commands add to it.
var itemSubs = map[string]func(c *Ctx, args []string) error{"open": itemOpen}

// itemID is the form of a decision item id, so an id never names a path outside <run>/items or a wall.
var itemID = regexp.MustCompile(`^D[1-9][0-9]*$`)

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
		var it item.Item
		err := os.ErrNotExist
		if itemID.MatchString(id) {
			err = item.Load(d, id, &it)
		}
		if errors.Is(err, os.ErrNotExist) {
			return exit.New(exit.Refused, "item %s: no such item", id)
		} else if err != nil {
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
