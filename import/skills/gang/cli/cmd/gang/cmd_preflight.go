package main

import (
	"fmt"

	"gang/internal/exit"
	"gang/internal/preflight"
)

func init() { commands["preflight"] = cmdPreflight }

// cmdPreflight is gang preflight --holder <session>. Any FAIL exits 1.
func cmdPreflight(c *Ctx, args []string) error {
	fs := newFlagSet("preflight")
	holder := fs.String("holder", "", "driver lock holder, the coordinator session id (required)")
	if _, err := owArgs(fs, args, 0); err != nil {
		return err
	}
	if *holder == "" {
		return exit.New(exit.Usage, "preflight: --holder is required")
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	checks, err := preflight.Run(c.Repo, d, p, *holder, c.Now())
	if err != nil {
		return err
	}
	failed := 0
	for _, ch := range checks {
		if ch.Skip {
			fmt.Fprintf(c.Stdout, "SKIP %s\n", ch.Name)
			continue
		}
		if ch.OK {
			fmt.Fprintf(c.Stdout, "PASS %s\n", ch.Name)
			continue
		}
		failed++
		fmt.Fprintf(c.Stdout, "FAIL %s: %s\n", ch.Name, ch.Detail)
	}
	if failed > 0 {
		return exit.New(exit.User, "preflight: %d of %d checks failed", failed, len(checks))
	}
	return nil
}
