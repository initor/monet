package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"source/internal/exit"
	"source/internal/lock"
	"source/internal/relay"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["init"] = cmdInit }

const initUsage = "usage: monet-source init --slug <slug> --holder <session>"

// slugRE is the form of a run slug, which is also the base name of <run>.
var slugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// runFolders are the folders that init makes in <run>.
var runFolders = []string{"locks", "relay", "personas", "artifacts", "rounds", "items", "handoff", "notes", "dev", "watch", "bin"}

// relaySteps reads <skill>/reference/relay.md. A failed read is exit 2,
// "<cmd>: <path>: <error>".
func relaySteps(cmd, skill string) ([]byte, error) {
	p := filepath.Join(skill, "reference", "relay.md")
	data, err := os.ReadFile(p)
	if pe := (*fs.PathError)(nil); errors.As(err, &pe) {
		err = pe.Err
	}
	if err != nil {
		return nil, exit.New(exit.Refused, "%s: %s: %v", cmd, p, err)
	}
	return data, nil
}

// cmdInit checks the slug, runHome, the relay steps of the skill, and that <run> has no state.json,
// takes brain for 12 hours, makes the run folders, copies the steps to relay/brain.md, writes the
// config of <home> unless it names another procedure or cannot be read (R7), adds the run lines to
// the local git exclude of <home> (R8), and writes state.json last. A refusal (exit 64, 2, or 4)
// changes nothing, and a failed write (exit 1) leaves no state.json.
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
	home, err := runHome(c.Run)
	if err != nil {
		return err
	}
	skill, err := needSkill(c)
	if err != nil {
		return err
	}
	steps, err := relaySteps("init", skill)
	if err != nil {
		return err
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
	brain := d.Path("relay", "brain.md")
	if err := store.WriteFileAtomic(brain, steps, 0o644); err != nil {
		return err
	}
	if cfg, found, err := relay.ReadConfig(home); err == nil && (!found || cfg.Procedure == brain) {
		if err := relay.WriteConfig(home, brain); err != nil {
			return err
		}
	}
	if err := excludeRun(home); err != nil {
		return err
	}
	return store.WriteJSON(d.State(), state.State{Version: 1, Slug: *slug, Created: now})
}
