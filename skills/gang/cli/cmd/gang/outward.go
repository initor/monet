package main

import (
	"errors"
	"flag"
	"time"

	"gang/internal/exit"
	"gang/internal/lock"
	"gang/internal/store"
)

// owArgs parses flags anywhere in args (plan 01 parseFlags) and requires
// exactly n positional arguments.
func owArgs(fs *flag.FlagSet, args []string, n int) ([]string, error) {
	pos, err := parseFlags(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) != n {
		return nil, exit.New(exit.Usage, "%s: want %d argument(s), got %d", fs.Name(), n, len(pos))
	}
	return pos, nil
}

// requireDriver renews the driver lease for holder for 12 hours, before any outward step.
// An empty holder is exit 64, another holder or no lease is exit 4, and any other lease error stops the command.
func requireDriver(c *Ctx, holder string) error {
	if holder == "" {
		return exit.New(exit.Usage, "--holder <session> is required: the driver lease holder")
	}
	err := lock.Renew(store.Dir{Root: c.Dir}.Run("locks"), "driver", holder, 12*time.Hour, c.Now())
	if errors.Is(err, lock.ErrHeld) {
		return exit.New(exit.Held, "%v", err)
	}
	return err
}
