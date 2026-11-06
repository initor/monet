package main

import (
	"fmt"
	"os"

	"source/internal/exit"
	"source/internal/lock"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["dev"] = subcommands("dev", devSubs) }

// devSubs maps each dev subcommand to its handler. Later dev commands add to it.
var devSubs = map[string]func(c *Ctx, args []string) error{"hello": devHello, "beat": devBeat, "takeover": devTakeover, "start": devStart}

// devStart needs a skill dir, renews brain, and needs a seal. Then it makes <run>/dev when
// missing, writes the kickoff <run>/handoff/dev-<stamp>.md (R4), and prints the command that
// starts the developer. It runs nothing (call 14).
func devStart(c *Ctx, args []string) error {
	fs := newFlagSet("dev start")
	holder := fs.String("holder", "", "the brain lease holder")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return exit.New(exit.Usage, "usage: monet-source dev start --holder <session>")
	}
	skill, err := needSkill(c)
	if err != nil {
		return err
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	s, err := state.Load(d)
	if err != nil {
		return err
	}
	if s.Sealed == nil {
		return exit.New(exit.Refused, "dev start: seal the charter first")
	}
	if err := os.MkdirAll(d.Path("dev"), 0o755); err != nil {
		return err
	}
	kick := d.Path("handoff", "dev-"+c.Now().UTC().Format("20060102T150405Z")+".md")
	if err := store.WriteFileAtomic(kick, []byte(devKickoff(s.Slug, c.Run, skill)), 0o644); err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.Stdout, devStartCommand(s.Slug, c.Run, kick))
	return err
}

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
		return setDev(c, *holder)
	}
}

// setDev records holder as Dev and now as LastBeat.
func setDev(c *Ctx, holder string) error {
	now := c.Now()
	return state.Update(store.Dir{Root: c.Run}, func(s *state.State) error {
		s.Dev, s.LastBeat = holder, &now
		return nil
	})
}

// devTakeover moves the live dev lease of --from to --holder (a lease that --from
// does not hold live is exit 4; no --from, no --holder, or any other argument is
// exit 64), records holder as Dev and now as LastBeat, and records
// DEV_RELAYED <from> <holder>.
func devTakeover(c *Ctx, args []string) error {
	fs, d := newFlagSet("dev"), store.Dir{Root: c.Run}
	from, holder := fs.String("from", "", "the old dev session"), fs.String("holder", "", "the new dev lease holder")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || *from == "" || *holder == "" {
		return exit.New(exit.Usage, "usage: monet-source dev takeover --from <old session> --holder <session>")
	} else if err := heldExit(lock.Takeover(d.Locks(), "dev", *from, *holder, leaseTTL, c.Now())); err != nil {
		return err
	} else if err := setDev(c, *holder); err != nil {
		return err
	}
	return record(c, d, "DEV_RELAYED "+*from+" "+*holder)
}
