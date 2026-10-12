// Command gang is the CLI behind /monet:gang. It owns the plan model, run
// state, git merges, gates, locks, and GitHub calls for one gang dir.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/store"
)

// skillDir is the skill folder, set at build time with
// -ldflags "-X main.skillDir=<path>".
var skillDir string

// executable reports the running binary's path. Tests replace it.
var executable = os.Executable

// Ctx is what every command handler receives.
type Ctx struct {
	Dir, Repo, SkillDir string
	Stdin               io.Reader
	Stdout, Stderr      io.Writer
	Now                 func() time.Time
}

// commands maps a command's first word to its handler. Each
// cmd_<name>.go file registers itself from init.
var commands = map[string]func(c *Ctx, args []string) error{}

const usageLine = "usage: gang [--dir <gang dir>] [--skill <skill dir>] <command> [args]"

func main() {
	c := &Ctx{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Now: time.Now}
	os.Exit(dispatch(c, os.Args[1:]))
}

// dispatch resolves the global flags into c, runs one command, and maps its
// error to an exit code, printing "gang: <msg>" to c.Stderr.
func dispatch(c *Ctx, args []string) int {
	err := run(c, args)
	if err == nil {
		return exit.OK
	}
	var e *exit.Error
	if errors.As(err, &e) {
		if e.Msg != "" {
			fmt.Fprintf(c.Stderr, "gang: %s\n", err)
		}
		return e.Code
	}
	fmt.Fprintf(c.Stderr, "gang: %s\n", err)
	return exit.User
}

func run(c *Ctx, args []string) error {
	fs := newFlagSet("gang")
	dir := fs.String("dir", "", "gang dir")
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
	if c.Dir, err = resolveDir(*dir); err != nil {
		return err
	}
	if c.Repo, err = gitx.MainRepo(c.Dir); err != nil {
		return exit.New(exit.Usage, "gang dir %s is not in a git repo: %v", c.Dir, err)
	}
	c.SkillDir = firstSet(*skill, os.Getenv("GANG_SKILL"), skillDir)
	return h(c, fs.Args()[1:])
}

// resolveDir picks the gang dir: the flag, then GANG_DIR, then the binary's
// own location when it is <dir>/run/bin/gang. The dir must exist.
func resolveDir(flagDir string) (string, error) {
	dir := firstSet(flagDir, os.Getenv("GANG_DIR"))
	if dir == "" {
		if exe, err := executable(); err == nil {
			bin := filepath.Dir(exe)
			if filepath.Base(exe) == "gang" && filepath.Base(bin) == "bin" && filepath.Base(filepath.Dir(bin)) == "run" {
				dir = filepath.Dir(filepath.Dir(bin))
			}
		}
	}
	if dir == "" {
		return "", exit.New(exit.Usage, "no gang dir: pass --dir, set GANG_DIR, or run <dir>/run/bin/gang")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", exit.New(exit.Usage, "gang dir %s: %v", dir, err)
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return "", exit.New(exit.Usage, "gang dir %s does not exist", abs)
	}
	return abs, nil
}

func firstSet(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// newFlagSet returns a FlagSet that reports errors only through its result.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parseFlags parses args with flags allowed before, between, or after the
// positional arguments, and returns the positionals in order. "--" ends flag
// parsing. A bad flag is a usage error.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, exit.New(exit.Usage, "%s: %v", fs.Name(), err)
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		if used := len(args) - len(rest); used > 0 && args[used-1] == "--" {
			return append(pos, rest...), nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

// subcommands returns a handler that dispatches on its first argument to
// subs, for multi-word commands such as "plan commit" or "ledger record".
func subcommands(name string, subs map[string]func(c *Ctx, args []string) error) func(c *Ctx, args []string) error {
	return func(c *Ctx, args []string) error {
		names := make([]string, 0, len(subs))
		for n := range subs {
			names = append(names, n)
		}
		sort.Strings(names)
		if len(args) == 0 {
			return exit.New(exit.Usage, "%s: want a subcommand: %s", name, strings.Join(names, ", "))
		}
		h, ok := subs[args[0]]
		if !ok {
			return exit.New(exit.Usage, "%s: unknown subcommand %q: want %s", name, args[0], strings.Join(names, ", "))
		}
		return h(c, args[1:])
	}
}

// loadPlan loads plan/plan.json from the gang dir. A failure is a user error.
func loadPlan(c *Ctx) (*plan.Plan, error) {
	p, err := plan.Load(store.Dir{Root: c.Dir}.Plan())
	if err != nil {
		return nil, exit.New(exit.User, "%v", err)
	}
	return p, nil
}

// needSkill returns the skill dir, or a usage error when none was resolved.
func needSkill(c *Ctx) (string, error) {
	if c.SkillDir == "" {
		return "", exit.New(exit.Usage, "no skill dir: pass --skill or set GANG_SKILL")
	}
	return c.SkillDir, nil
}

// decodeStrict decodes the one JSON value in path into v. Unknown fields and
// trailing data are errors.
func decodeStrict(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := dec.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: trailing data after the JSON value", path)
	}
	return nil
}
