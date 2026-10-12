package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"gang/internal/exit"
	"gang/internal/notes"
	"gang/internal/repair"
	"gang/internal/store"
)

func init() {
	commands["report"] = cmdReport
	commands["gates"] = subcommands("gates", map[string]func(c *Ctx, args []string) error{
		"park":   gatesPark,
		"list":   gatesList,
		"answer": gatesAnswer,
	})
	commands["standing"] = subcommands("standing", map[string]func(c *Ctx, args []string) error{
		"show":       standingEdit(nil, false),
		"add":        standingEdit(notes.Add, true),
		"stop":       standingEdit(notes.Stop, true),
		"clear-stop": standingEdit(func(d store.Dir, _ string) error { return notes.ClearStop(d) }, false),
	})
}

func cmdReport(c *Ctx, args []string) error {
	fs := newFlagSet("report")
	heading := fs.String("heading", "report", "section heading for the append")
	name := fs.String("name", "", "write stdin to tasks/<id>/<name> instead of appending")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return exit.New(exit.Usage, "usage: gang report <id> [--heading <text>] [--name <file.md>]")
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	if _, ok := repair.TaskFor(p, pos[0]); !ok {
		return exit.New(exit.User, "unknown task %s", pos[0])
	}
	body, err := io.ReadAll(c.Stdin)
	if err != nil {
		return err
	}
	var path string
	if *name != "" {
		if len(strings.TrimSpace(string(body))) == 0 {
			return exit.New(exit.User, "stdin is empty")
		}
		path, err = notes.WriteNamed(d, pos[0], *name, body)
	} else {
		path, err = notes.Report(d, pos[0], *heading, body)
	}
	if err != nil {
		return exit.New(exit.User, "%v", err)
	}
	fmt.Fprintln(c.Stdout, path)
	return nil
}

func gatesPark(c *Ctx, args []string) error {
	fs := newFlagSet("gates park")
	question := fs.String("question", "", "the question")
	def := fs.String("default", "", "the default the agent continues on")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *question == "" || *def == "" {
		return exit.New(exit.Usage, "usage: gang gates park <id> --question <q> --default <d>")
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	if _, ok := repair.TaskFor(p, pos[0]); !ok {
		return exit.New(exit.Refused, "unknown task %s", pos[0])
	}
	id, err := notes.Park(d, pos[0], *question, *def)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, id)
	return nil
}

// gatesList prints each open gate in the gates.md block format.
func gatesList(c *Ctx, args []string) error {
	if len(args) != 0 {
		return exit.New(exit.Usage, "usage: gang gates list")
	}
	gs, err := notes.List(store.Dir{Root: c.Dir})
	if err != nil {
		return err
	}
	for i, g := range gs {
		if i > 0 {
			fmt.Fprintln(c.Stdout)
		}
		fmt.Fprintf(c.Stdout, "## %s %s %s\nQuestion: %s\nDefault: %s\n", g.ID, g.Task, g.State, g.Question, g.Default)
	}
	return nil
}

func gatesAnswer(c *Ctx, args []string) error {
	fs := newFlagSet("gates answer")
	answer := fs.String("answer", "", "the owner's answer")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *answer == "" {
		return exit.New(exit.Usage, "usage: gang gates answer <G-id> --answer <a>")
	}
	err = notes.Answer(store.Dir{Root: c.Dir}, pos[0], *answer)
	if errors.Is(err, notes.ErrNotFound) {
		return exit.New(exit.Refused, "%v", err)
	}
	return err
}

// standingEdit runs edit with the rest of the words joined (when it takes
// text), then prints the standing orders. A nil edit only prints.
func standingEdit(edit func(d store.Dir, text string) error, takesText bool) func(c *Ctx, args []string) error {
	return func(c *Ctx, args []string) error {
		text := strings.Join(args, " ")
		if takesText == (text == "") {
			return exit.New(exit.Usage, "usage: gang standing show | add <line> | stop <reason> | clear-stop")
		}
		d := store.Dir{Root: c.Dir}
		if edit != nil {
			if err := edit(d, text); err != nil {
				return exit.New(exit.User, "%v", err)
			}
		}
		s, err := notes.Show(d)
		if err != nil {
			return err
		}
		fmt.Fprint(c.Stdout, s)
		return nil
	}
}
