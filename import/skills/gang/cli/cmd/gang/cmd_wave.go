package main

import (
	"flag"
	"fmt"
	"strconv"

	"gang/internal/exit"
	"gang/internal/wave"
)

func init() { commands["wave"] = subcommands("wave", owWaveSubs) }

// owWaveSubs holds the gang wave subcommands. cmd_wave_merge.go adds merge.
var owWaveSubs = map[string]func(c *Ctx, args []string) error{
	"open":   cmdWaveOpen,
	"status": cmdWaveStatus,
}

// owWave reads the one wave number, with flags before or after it.
func owWave(fs *flag.FlagSet, args []string) (int, error) {
	pos, err := owArgs(fs, args, 1)
	if err != nil {
		return 0, err
	}
	k, err := strconv.Atoi(pos[0])
	if err != nil || k < 1 {
		return 0, exit.New(exit.Usage, "%s: wave must be a positive integer, got %q", fs.Name(), pos[0])
	}
	return k, nil
}

// cmdWaveOpen is gang wave open <k> [--dry-run].
func cmdWaveOpen(c *Ctx, args []string) error {
	fs := newFlagSet("wave open")
	dry := fs.Bool("dry-run", false, "print the PR instead of opening it")
	k, err := owWave(fs, args)
	if err != nil {
		return err
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	out, err := wave.Open(c.Repo, d, p, k, *dry)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, out)
	return nil
}

// cmdWaveStatus is gang wave status: the waves.tsv table.
func cmdWaveStatus(c *Ctx, args []string) error {
	if _, err := owArgs(newFlagSet("wave status"), args, 0); err != nil {
		return err
	}
	d, _, err := openRun(c)
	if err != nil {
		return err
	}
	rows, err := wave.Read(d)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, "wave\tpr\thead_sha\tstate\topened_at")
	for _, r := range rows {
		fmt.Fprintf(c.Stdout, "%d\t%d\t%s\t%s\t%s\n", r.Wave, r.PR, r.HeadSHA, r.State, r.OpenedAt)
	}
	return nil
}
