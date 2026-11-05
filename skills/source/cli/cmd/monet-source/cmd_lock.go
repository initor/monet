package main

import (
	"source/internal/exit"
	"source/internal/lock"
	"source/internal/store"
)

func init() {
	commands["lock"] = subcommands("lock", map[string]func(c *Ctx, args []string) error{
		"acquire": lockVerb("acquire"),
		"renew":   lockVerb("renew"),
		"release": lockVerb("release"),
	})
}

const lockUsage = "usage: monet-source lock acquire|renew|release <name> --holder <h> [--ttl 12h]"

// lockVerb runs one lease verb on <run>/locks/<name>.json. Another holder is
// exit 4, and a call without one name and a holder is exit 64.
func lockVerb(verb string) func(c *Ctx, args []string) error {
	return func(c *Ctx, args []string) error {
		fs := newFlagSet("lock " + verb)
		holder := fs.String("holder", "", "who holds the lease")
		ttl := fs.Duration("ttl", leaseTTL, "lease length")
		pos, err := parseFlags(fs, args)
		if err != nil {
			return err
		}
		if len(pos) != 1 || *holder == "" {
			return exit.New(exit.Usage, "%s", lockUsage)
		}
		dir := store.Dir{Root: c.Run}.Locks()
		switch verb {
		case "acquire":
			_, err = lock.Acquire(dir, pos[0], *holder, *ttl, c.Now())
		case "renew":
			err = lock.Renew(dir, pos[0], *holder, *ttl, c.Now())
		default:
			err = lock.Release(dir, pos[0], *holder)
		}
		return heldExit(err)
	}
}
