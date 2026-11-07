package main

import (
	"fmt"
	"os"
	"os/exec"

	"source/internal/exit"
	"source/internal/preflight"
)

func init() { commands["preflight"] = cmdPreflight }

// runTool runs a program with GOTOOLCHAIN=local and gives its combined output. Tests replace it.
var runTool preflight.Runner = func(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// cmdPreflight prints a FAIL line per failed check of preflight.All and exits 2, or prints preflight: clean.
func cmdPreflight(c *Ctx, args []string) error {
	if len(args) != 0 {
		return exit.New(exit.Usage, "usage: monet-source preflight")
	}
	home, err := runHome(c.Run)
	if err != nil {
		return err
	}
	failed := false
	for _, ch := range preflight.All(runTool, home, c.Run) {
		if !ch.OK {
			failed = true
			fmt.Fprintf(c.Stdout, "FAIL %s: %s\n", ch.Name, ch.Detail)
		}
	}
	if failed {
		return &exit.Error{Code: exit.Refused}
	}
	fmt.Fprintln(c.Stdout, "preflight: clean")
	return nil
}
