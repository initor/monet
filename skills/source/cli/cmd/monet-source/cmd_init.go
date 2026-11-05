package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"source/internal/exit"
	"source/internal/lock"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["init"] = cmdInit }

const initUsage = "usage: monet-source init --slug <slug> --holder <session>"

// slugRE is the form of a run slug, which is also the base name of <run>.
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// runFolders are the folders that init makes in <run>.
var runFolders = []string{"locks", "relay", "personas", "artifacts", "rounds", "items", "handoff", "notes", "dev", "watch", "bin"}

// cmdInit checks the slug and that <run> has no state.json, takes brain for
// 12 hours, then makes the run folders and writes state.json. A refused init
// (exit 64, 2, or 4) changes nothing. A failed mkdir or write (exit 1) keeps
// the lease and writes no state.json, so the holder can run init again.
func cmdInit(c *Ctx, args []string) error {
	flags := newFlagSet("init")
	slug := flags.String("slug", "", "the run slug, the base name of <run>")
	holder := flags.String("holder", "", "the brain lease holder")
	pos, err := parseFlags(flags, args)
	if err != nil {
		return err
	}
	if len(pos) != 0 || *holder == "" {
		return exit.New(exit.Usage, "%s", initUsage)
	}
	if base := filepath.Base(c.Run); !slugRE.MatchString(*slug) || *slug != base {
		return exit.New(exit.Usage, "init: slug %q must match %s and equal the run folder name %q", *slug, slugRE, base)
	}
	d := store.Dir{Root: c.Run}
	if _, err := os.Lstat(d.State()); err == nil {
		return exit.New(exit.Refused, "run %s already has state", *slug)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	now := c.Now()
	if _, err := lock.Acquire(d.Locks(), "brain", *holder, leaseTTL, now); err != nil {
		return heldExit(err)
	}
	for _, name := range runFolders {
		if err := os.MkdirAll(d.Path(name), 0o755); err != nil {
			return err
		}
	}
	return store.WriteJSON(d.State(), state.State{Version: 1, Slug: *slug, Created: now})
}
