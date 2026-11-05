// Command monet-source is the engine behind /monet:source. It owns the run
// folder of one source run: its leases, events, owner words, and charter.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"source/internal/exit"
)

// skillDir is the skill folder, set at build time with
// -ldflags "-X main.skillDir=<path>".
var skillDir string

// executable reports the running binary's path. Tests replace it.
var executable = os.Executable

// Ctx is what every command handler receives.
type Ctx struct {
	Run, SkillDir  string
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Now            func() time.Time
}

// commands maps a command's first word to its handler. Each
// cmd_<name>.go file registers itself from init.
var commands = map[string]func(c *Ctx, args []string) error{}

const usageLine = "usage: monet-source [--run <run dir>] [--skill <skill dir>] <command> [args]"

func main() {
	c := &Ctx{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Now: time.Now}
	os.Exit(dispatch(c, os.Args[1:]))
}

// dispatch resolves the global flags into c, runs one command, and maps its
// error to an exit code, printing "monet-source: <msg>" to c.Stderr.
func dispatch(c *Ctx, args []string) int {
	err := run(c, args)
	if err == nil {
		return exit.OK
	}
	var e *exit.Error
	if errors.As(err, &e) {
		if e.Msg != "" {
			fmt.Fprintf(c.Stderr, "monet-source: %s\n", err)
		}
		return e.Code
	}
	fmt.Fprintf(c.Stderr, "monet-source: %s\n", err)
	return exit.User
}

// run checks the command name before the run dir, so a typo needs no run.
func run(c *Ctx, args []string) error {
	fs := newFlagSet("monet-source")
	runDir := fs.String("run", "", "run dir")
	skill := fs.String("skill", "", "skill dir")
	if err := fs.Parse(args); err != nil {
		return exit.New(exit.Usage, "%v\n%s", err, usageLine)
	}
	if fs.NArg() == 0 {
		return exit.New(exit.Usage, "%s", usageLine)
	}
	h, ok := commands[fs.Arg(0)]
	if !ok {
		return exit.New(exit.Usage, "unknown command %q\n%s", fs.Arg(0), usageLine)
	}
	var err error
	if c.Run, err = resolveRun(*runDir); err != nil {
		return err
	}
	c.SkillDir = firstSet(*skill, os.Getenv("SOURCE_SKILL"), skillDir)
	return h(c, fs.Args()[1:])
}

// resolveRun picks the run dir: the flag, then SOURCE_RUN, then <dir> when
// the binary is <dir>/bin/monet-source. The dir must exist. It returns the
// absolute path with symlinks resolved, the only form the engine stores.
func resolveRun(flagRun string) (string, error) {
	dir := firstSet(flagRun, os.Getenv("SOURCE_RUN"))
	if dir == "" {
		if exe, err := executable(); err == nil && filepath.Base(exe) == "monet-source" && filepath.Base(filepath.Dir(exe)) == "bin" {
			dir = filepath.Dir(filepath.Dir(exe))
		}
	}
	if dir == "" {
		return "", exit.New(exit.Usage, "no run dir: pass --run, set SOURCE_RUN, or run <dir>/bin/monet-source")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", exit.New(exit.Usage, "run dir %s: %v", dir, err)
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return "", exit.New(exit.Usage, "run dir %s does not exist", abs)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", exit.New(exit.Usage, "run dir %s: %v", abs, err)
	}
	return resolved, nil
}

// firstSet returns the first non-empty value, or "" when all are empty.
func firstSet(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// needSkill returns the skill dir, or a usage error when none was resolved.
func needSkill(c *Ctx) (string, error) {
	if c.SkillDir == "" {
		return "", exit.New(exit.Usage, "no skill dir: pass --skill or set SOURCE_SKILL")
	}
	return c.SkillDir, nil
}

// newFlagSet returns a FlagSet that reports errors only through its result.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}
