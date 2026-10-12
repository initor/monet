package main

import (
	"errors"
	"time"

	"gang/internal/exit"
	"gang/internal/lock"
	"gang/internal/store"
)

func init() {
	commands["lock"] = subcommands("lock", map[string]func(c *Ctx, args []string) error{
		"acquire": lockVerb("acquire"),
		"renew":   lockVerb("renew"),
		"release": lockVerb("release"),
	})
}

const lockUsage = "usage: gang lock acquire|renew|release <name> --holder <h> [--ttl 5m]"

// lockVerb maps lock.ErrHeld to exit 4. CLI leases record pid 0, so only
// expiry frees them: the gang process exits as soon as the call returns.
func lockVerb(verb string) func(c *Ctx, args []string) error {
	return func(c *Ctx, args []string) error {
		fs := newFlagSet("lock " + verb)
		holder := fs.String("holder", "", "who holds the lease")
		ttl := fs.Duration("ttl", 5*time.Minute, "lease length")
		pos, err := parseFlags(fs, args)
		if err != nil {
			return err
		}
		if len(pos) != 1 || *holder == "" {
			return exit.New(exit.Usage, lockUsage)
		}
		dir := store.Dir{Root: c.Dir}.Run("locks")
		switch verb {
		case "acquire":
			_, err = lock.Acquire(dir, pos[0], *holder, *ttl, c.Now())
		case "renew":
			err = lock.Renew(dir, pos[0], *holder, *ttl, c.Now())
		default:
			err = lock.Release(dir, pos[0], *holder)
		}
		if errors.Is(err, lock.ErrHeld) {
			return exit.New(exit.Held, "%v", err)
		}
		return err
	}
}
