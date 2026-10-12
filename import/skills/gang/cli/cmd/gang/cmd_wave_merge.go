package main

import (
	"fmt"

	"gang/internal/wave"
)

func init() { owWaveSubs["merge"] = cmdWaveMerge }

// cmdWaveMerge is gang wave merge <k>.
func cmdWaveMerge(c *Ctx, args []string) error {
	k, err := owWave(newFlagSet("wave merge"), args)
	if err != nil {
		return err
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	sha, err := wave.Merge(c.Repo, d, p, k)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, sha)
	return nil
}
