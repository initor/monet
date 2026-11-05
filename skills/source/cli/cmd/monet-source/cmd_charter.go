package main

import (
	"fmt"

	"source/internal/charter"
	"source/internal/exit"
	"source/internal/store"
)

// charter is a subcommands group, so each charter step adds its own word.
func init() {
	commands["charter"] = subcommands("charter", map[string]func(c *Ctx, args []string) error{
		"lint": charterLint,
	})
}

// charterLint checks the charter at its one path, by default <run>/charter.json.
// It takes no lease and writes nothing. A problem or a load error exits 2.
func charterLint(c *Ctx, args []string) error {
	pos, err := parseFlags(newFlagSet("charter lint"), args)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return exit.New(exit.Usage, "charter lint: want at most one path")
	}
	path := store.Dir{Root: c.Run}.Charter()
	if len(pos) == 1 {
		path = pos[0]
	}
	ch, err := charter.Load(path)
	if err != nil {
		return exit.New(exit.Refused, "%v", err)
	}
	problems := charter.Lint(ch)
	if len(problems) == 0 {
		problems = []string{"clean"}
	}
	for _, p := range problems {
		if _, err := fmt.Fprintf(c.Stdout, "charter: %s\n", p); err != nil {
			return err
		}
	}
	if problems[0] != "clean" {
		return &exit.Error{Code: exit.Refused}
	}
	return nil
}
