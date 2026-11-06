package main

import (
	"source/internal/exit"
	"source/internal/lock"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["dev"] = subcommands("dev", devSubs) }

// devSubs maps each dev subcommand to its handler. Later dev commands add to it.
var devSubs = map[string]func(c *Ctx, args []string) error{"hello": devHello, "beat": devBeat}

// devHello acquires the dev lease for holder (another live holder is exit 4).
var devHello = devMark(func(c *Ctx, h string) error {
	_, err := lock.Acquire(store.Dir{Root: c.Run}.Locks(), "dev", h, leaseTTL, c.Now())
	return heldExit(err)
})

// devBeat renews the dev lease of holder (no lease or another holder is exit 4).
var devBeat = devMark(func(c *Ctx, h string) error { return requireLease(c, "dev", h) })

// devMark returns a handler that reads --holder (none, or any other argument, is
// exit 64), runs lease for it, and then records holder as Dev and now as LastBeat.
func devMark(lease func(c *Ctx, holder string) error) func(c *Ctx, args []string) error {
	return func(c *Ctx, args []string) error {
		fs := newFlagSet("dev")
		holder := fs.String("holder", "", "the dev lease holder")
		if err := fs.Parse(args); err != nil || fs.NArg() != 0 || *holder == "" {
			return exit.New(exit.Usage, "usage: monet-source dev hello|beat --holder <session>")
		} else if err := lease(c, *holder); err != nil {
			return err
		}
		now := c.Now()
		return state.Update(store.Dir{Root: c.Run}, func(s *state.State) error {
			s.Dev, s.LastBeat = *holder, &now
			return nil
		})
	}
}
