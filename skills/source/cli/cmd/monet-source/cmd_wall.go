package main

import (
	"fmt"

	"source/internal/exit"
	"source/internal/item"
	"source/internal/lock"
	"source/internal/state"
	"source/internal/store"
)

// wallSubs are the subcommands of wall.
var wallSubs = map[string]func(c *Ctx, args []string) error{"open": wallOpen}

func init() { commands["wall"] = subcommands("wall", wallSubs) }

// eitherLease renews brain when holder holds brain, else dev, and returns the name (R29).
func eitherLease(c *Ctx, holder string) (string, error) {
	name := "dev"
	if l, ok, err := lock.Read(store.Dir{Root: c.Run}.Locks(), "brain"); err != nil {
		return "", err
	} else if ok && l.Holder == holder {
		name = "brain"
	}
	return name, requireLease(c, name, holder)
}

// wallIn is the stdin of wall open.
type wallIn struct {
	Question string `json:"question"`
	Default  string `json:"default"`
	Blocks   string `json:"blocks"`
}

// wallOpen renews brain or dev, decodes a strict stdin (exit 2), and under the state mutex saves
// items/W<Walls+1>.json as open by that lease and counts it in Walls. It records WALL W<n> and prints the id.
func wallOpen(c *Ctx, args []string) error {
	fs := newFlagSet("wall open")
	holder := fs.String("holder", "", "the brain or dev lease holder")
	if pos, err := parseFlags(fs, args); err != nil {
		return err
	} else if len(pos) != 0 {
		return exit.New(exit.Usage, "usage: monet-source wall open --holder <session>")
	}
	by, err := eitherLease(c, *holder)
	if err != nil {
		return err
	}
	var in wallIn
	if err := decodeStdin(c, &in); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	var id string
	err = state.Update(d, func(s *state.State) error {
		s.Walls++
		id = fmt.Sprintf("W%d", s.Walls)
		return item.Save(d, id, item.Wall{ID: id, By: by, Question: in.Question, Default: in.Default, Blocks: in.Blocks, Status: "open"})
	})
	if err == nil {
		err = record(c, d, "WALL "+id)
	}
	if err == nil {
		fmt.Fprintln(c.Stdout, id)
	}
	return err
}
