package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"source/internal/artifact"
	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["submit"] = cmdSubmit }

const submitUsage = "usage: monet-source submit --path <file>... --holder <session>"

// cmdSubmit renews dev, then under the state mutex needs a seal and a phase of a clean charter,
// refuses a path in <run>/notes, and freezes the paths as artifacts/<phase>/v<k> (k = Versions[phase]+1).
// Each refusal is exit 2 and writes nothing. It records VERSION <phase> v<k> and prints the folder.
func cmdSubmit(c *Ctx, args []string) error {
	fs := newFlagSet("submit")
	holder := fs.String("holder", "", "the dev lease holder")
	var paths []string
	fs.Func("path", "a file of the version (repeats)", func(p string) error { paths = append(paths, p); return nil })
	if pos, err := parseFlags(fs, args); err != nil {
		return err
	} else if len(pos) != 0 || len(paths) == 0 {
		return exit.New(exit.Usage, "%s", submitUsage)
	}
	if err := requireLease(c, "dev", *holder); err != nil {
		return err
	}
	d := store.Dir{Root: c.Run}
	var line, dest string
	err := state.Update(d, func(s *state.State) error {
		if _, ch, err := cleanCharter(c, d); err != nil {
			return err
		} else if s.Sealed == nil || !slices.Contains(ch.Phases, s.Phase) {
			return exit.New(exit.Refused, "submit: phase %s takes no version", s.Phase)
		}
		for _, p := range paths {
			if in, err := underNotes(c.Run, p); err != nil {
				return exit.New(exit.Refused, "submit: %v", err)
			} else if in {
				return exit.New(exit.Refused, "submit: %s is in %s", p, d.Path("notes"))
			}
		}
		k := s.Versions[s.Phase] + 1
		dest = d.Path("artifacts", s.Phase, "v"+strconv.Itoa(k))
		if _, err := artifact.Freeze(dest, s.Phase, k, c.Now(), paths); err != nil {
			return exit.New(exit.Refused, "submit: %v", err)
		}
		if s.Versions == nil {
			s.Versions = map[string]int{}
		}
		s.Versions[s.Phase], line = k, fmt.Sprintf("VERSION %s v%d", s.Phase, k)
		return nil
	})
	if err == nil {
		if err = record(c, d, line); err == nil {
			_, err = fmt.Fprintln(c.Stdout, dest)
		}
	}
	return err
}

// underNotes reports whether path, made absolute with no Clean and resolved as the kernel does (a symlink, then ..),
// is <run>/notes or under it, by os.SameFile on each folder up (so NOTES counts on a case-insensitive disk), or an error.
func underNotes(run, path string) (bool, error) {
	abs, err := path, error(nil)
	if !filepath.IsAbs(path) {
		abs, err = os.Getwd()
		abs += "/" + path
	}
	if err == nil {
		abs, err = filepath.EvalSymlinks(abs)
	}
	notes, _ := os.Stat(filepath.Join(run, "notes")) // no notes folder gives nil, which matches no folder
	for p := abs; err == nil; p = filepath.Dir(p) {
		if fi, _ := os.Stat(p); os.SameFile(fi, notes) {
			return true, nil
		} else if p == filepath.Dir(p) {
			break
		}
	}
	return false, err
}
