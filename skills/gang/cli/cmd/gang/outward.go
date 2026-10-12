package main

import (
	"flag"

	"gang/internal/exit"
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
