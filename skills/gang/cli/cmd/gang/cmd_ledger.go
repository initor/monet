package main

import (
	"fmt"
	"strings"
	"time"

	"gang/internal/exit"
	"gang/internal/ledger"
	"gang/internal/repair"
)

func init() {
	commands["ledger"] = subcommands("ledger", map[string]func(c *Ctx, args []string) error{
		"record":  ledgerRecord,
		"check":   ledgerCheck,
		"summary": ledgerSummary,
	})
}

const ledgerUsage = "usage: gang ledger record <id> <sha> --lens review|adversarial --blockers N --majors N --survivors N [--gate-ran] [--mutation-checked] --evidence <path> | check <id> <sha> | summary"

func ledgerRecord(c *Ctx, args []string) error {
	fs := newFlagSet("ledger record")
	lens := fs.String("lens", "", "review or adversarial")
	blockers := fs.Int("blockers", -1, "blocker count")
	majors := fs.Int("majors", -1, "major count")
	survivors := fs.Int("survivors", -1, "surviving mutant count")
	gateRan := fs.Bool("gate-ran", false, "the reviewer re-ran the gate")
	mutation := fs.Bool("mutation-checked", false, "the reviewer ran the mutation check")
	evidence := fs.String("evidence", "", "path of the review findings")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 2 || *lens == "" || *blockers < 0 || *majors < 0 || *survivors < 0 || *evidence == "" {
		return exit.New(exit.Usage, ledgerUsage)
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	if _, ok := repair.TaskFor(p, pos[0]); !ok {
		return exit.New(exit.User, "unknown task %s", pos[0])
	}
	r := ledger.Row{
		Task: pos[0], SHA: pos[1], Lens: ledger.Lens(*lens),
		Blockers: *blockers, Majors: *majors, Survivors: *survivors, GateRan: *gateRan, MutationChecked: *mutation,
		Evidence: *evidence, At: c.Now().UTC().Format(time.RFC3339),
	}
	if err := ledger.Record(d, r); err != nil {
		return exit.New(exit.User, "%v", err)
	}
	fmt.Fprintln(c.Stdout, ledger.Compute(r.Blockers, r.Majors, r.Survivors, r.GateRan, r.MutationChecked))
	return nil
}

func ledgerCheck(c *Ctx, args []string) error {
	if len(args) != 2 {
		return exit.New(exit.Usage, ledgerUsage)
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	t, ok := repair.TaskFor(p, args[0])
	if !ok {
		return exit.New(exit.Refused, "unknown task %s", args[0])
	}
	ok, missing, err := ledger.Check(d, t.ID, args[1], ledger.RequiredLenses(t))
	if err != nil {
		return err
	}
	if ok {
		fmt.Fprintln(c.Stdout, ledger.Clean)
		return nil
	}
	names := make([]string, len(missing))
	for i, l := range missing {
		names[i] = string(l)
	}
	fmt.Fprintf(c.Stdout, "NOT-VERIFIED %s\n", strings.Join(names, ","))
	return exit.New(exit.Refused, "%s at %s is not verified", t.ID, args[1])
}

func ledgerSummary(c *Ctx, args []string) error {
	if len(args) != 0 {
		return exit.New(exit.Usage, ledgerUsage)
	}
	d, _, err := openRun(c)
	if err != nil {
		return err
	}
	m, err := ledger.Summary(d)
	if err != nil {
		return err
	}
	for _, v := range []ledger.Verdict{ledger.Clean, ledger.Changes, ledger.Incomplete} {
		fmt.Fprintf(c.Stdout, "%s %d\n", v, m[v])
	}
	return nil
}
