package main

import (
	"time"

	"source/internal/exit"
	"source/internal/store"
)

func init() {
	commands["decision"] = subcommands("decision", map[string]func(c *Ctx, args []string) error{"add": decisionAdd})
}

// decisionHeader is the header of decisions.tsv, the run decisions and their reasons.
var decisionHeader = []string{"at", "by", "decision", "reason"}

// logDecision appends [now, by, text, reason] to <run>/decisions.tsv, with now in UTC.
func logDecision(c *Ctx, d store.Dir, by, text, reason string) error {
	row := []string{c.Now().UTC().Format(time.RFC3339Nano), by, text, reason}
	return store.AppendTSV(d.Path("decisions.tsv"), decisionHeader, row)
}

// decisionAdd renews dev for --holder and logs <text> and --reason by dev (R23).
// One text and a reason are required (exit 64), and another holder is exit 4.
func decisionAdd(c *Ctx, args []string) error {
	fs := newFlagSet("decision add")
	reason := fs.String("reason", "", "why the decision was made")
	holder := fs.String("holder", "", "the dev lease holder")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || pos[0] == "" || *reason == "" {
		return exit.New(exit.Usage, "usage: monet-source decision add <text> --reason <reason> --holder <session>")
	}
	if err := requireLease(c, "dev", *holder); err != nil {
		return err
	}
	return logDecision(c, store.Dir{Root: c.Run}, "dev", pos[0], *reason)
}
