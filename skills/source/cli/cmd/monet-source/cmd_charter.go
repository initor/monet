package main

import (
	"fmt"
	"os"

	"source/internal/charter"
	"source/internal/exit"
	"source/internal/store"
)

// charter is a subcommands group, so each charter step adds its own word.
func init() {
	commands["charter"] = subcommands("charter", map[string]func(c *Ctx, args []string) error{
		"lint":   charterLint,
		"render": charterRender,
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

// charterRender renews brain, then prints lint problems as charter lint does
// and exits 2, or writes <run>/charter.md atomically and prints its path. It
// hashes charter.json before it loads it, so a change in between fails seal.
func charterRender(c *Ctx, args []string) error {
	fs := newFlagSet("charter render")
	holder := fs.String("holder", "", "the brain lease holder")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 0 {
		return exit.New(exit.Usage, "charter render: want no arguments")
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	data, err := os.ReadFile(d.Charter())
	if err != nil {
		return exit.New(exit.Refused, "%v", err)
	}
	ch, err := charter.Load(d.Charter())
	if err != nil {
		return exit.New(exit.Refused, "%v", err)
	}
	if problems := charter.Lint(ch); len(problems) > 0 {
		for _, p := range problems {
			if _, err := fmt.Fprintf(c.Stdout, "charter: %s\n", p); err != nil {
				return err
			}
		}
		return &exit.Error{Code: exit.Refused}
	}
	if err := store.WriteFileAtomic(d.CharterMD(), []byte(charter.Render(ch, charter.Hash(data))), 0o644); err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.Stdout, d.CharterMD())
	return err
}
