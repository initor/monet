package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"source/internal/charter"
	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
	"source/internal/words"
)

// charter is a subcommands group, so each charter step adds its own word.
func init() {
	commands["charter"] = subcommands("charter", map[string]func(c *Ctx, args []string) error{
		"amend":  charterAmend,
		"lint":   charterLint,
		"render": charterRender,
		"seal":   charterSeal,
	})
}

// logHeader is the header of log.tsv, the dated lines that the report packet reads.
var logHeader = []string{"at", "line"}

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
	data, ch, err := cleanCharter(c, d)
	if err != nil {
		return err
	}
	if err := store.WriteFileAtomic(d.CharterMD(), []byte(charter.Render(ch, charter.Hash(data))), 0o644); err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.Stdout, d.CharterMD())
	return err
}

// cleanCharter returns the bytes of <run>/charter.json and the charter. A read or load
// error exits 2, and lint problems print as charter lint prints them and exit 2.
func cleanCharter(c *Ctx, d store.Dir) ([]byte, *charter.Charter, error) {
	data, err := os.ReadFile(d.Charter())
	if err != nil {
		return nil, nil, exit.New(exit.Refused, "%v", err)
	}
	ch, err := charter.Load(d.Charter())
	if err != nil {
		return nil, nil, exit.New(exit.Refused, "%v", err)
	}
	problems := charter.Lint(ch)
	for _, p := range problems {
		if _, err := fmt.Fprintf(c.Stdout, "charter: %s\n", p); err != nil {
			return nil, nil, err
		}
	}
	if len(problems) > 0 {
		return nil, nil, &exit.Error{Code: exit.Refused}
	}
	return data, ch, nil
}

// charterSeal stores the seal and sets the first phase. A sealed run gives exit 2.
func charterSeal(c *Ctx, args []string) error {
	return confirmCharter(c, "seal", args, func(s *state.State, seal state.Seal, ch *charter.Charter) error {
		if s.Sealed != nil {
			return exit.New(exit.Refused, "charter already sealed with words %s", s.Sealed.Words)
		}
		s.Sealed, s.Phase = &seal, ch.Phases[0]
		return nil
	})
}

// charterAmend appends an amend to Amends. A run with no seal gives exit 2.
func charterAmend(c *Ctx, args []string) error {
	return confirmCharter(c, "amend", args, func(s *state.State, seal state.Seal, _ *charter.Charter) error {
		if s.Sealed == nil {
			return exit.New(exit.Refused, "charter not sealed: seal it before an amend")
		}
		s.Amends = append(s.Amends, seal)
		return nil
	})
}

// confirmCharter renews brain, then needs a clean lint, the entry --words in owner-words.md, a
// charter.md that ends with the hash of charter.json, and a state that apply takes, in that order.
// Then it logs one row "charter <name>ed with words <id>". A refusal writes nothing.
func confirmCharter(c *Ctx, name string, args []string, apply func(s *state.State, seal state.Seal, ch *charter.Charter) error) error {
	fs := newFlagSet("charter " + name)
	id := fs.String("words", "", "the id of the owner words that confirm charter.md")
	holder := fs.String("holder", "", "the brain lease holder")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 0 || *id == "" {
		return exit.New(exit.Usage, "usage: monet-source charter %s --words <id> --holder <session>", name)
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	data, ch, err := cleanCharter(c, d)
	if err != nil {
		return err
	}
	if _, err := words.Get(d.Words(), *id); err != nil {
		return exit.New(exit.Refused, "%v", err)
	}
	hash := charter.Hash(data)
	md, err := os.ReadFile(d.CharterMD())
	if err != nil {
		return exit.New(exit.Conflict, "%v", err)
	}
	if !strings.HasSuffix(string(md), "<!-- charter sha256="+hash+" -->\n") {
		return exit.New(exit.Conflict, "charter.json changed after charter.md was rendered: render it and get the yes of the owner again")
	}
	now := c.Now()
	if err := state.Update(d, func(s *state.State) error {
		return apply(s, state.Seal{Words: *id, At: now, Hash: hash}, ch)
	}); err != nil {
		return err
	}
	return store.AppendTSV(d.Log(), logHeader, []string{now.UTC().Format(time.RFC3339Nano), "charter " + name + "ed with words " + *id})
}
