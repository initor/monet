package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"source/internal/exit"
	"source/internal/relay"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["relay"] = subcommands("relay", relaySubs) }

// relaySubs maps each relay subcommand to its handler.
var relaySubs = map[string]func(c *Ctx, args []string) error{"mute": relayMute, "prompt": relayPrompt}

const relayPromptUsage = "usage: monet-source relay prompt --role brain|dev --handoff <path> --holder <session> [--mode <mode>] [--transcript <path>]"

// relayPrompt renews the lease of the role, writes the relay kickoff beside
// the handoff, and prints the claude --bg command of the next session of the
// role (R21, R24, R25). It runs nothing and emits no event.
func relayPrompt(c *Ctx, args []string) error {
	fs := newFlagSet("relay prompt")
	role, handoff, holder := fs.String("role", "", ""), fs.String("handoff", "", ""), fs.String("holder", "", "")
	mode, tr := fs.String("mode", "", ""), fs.String("transcript", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || (*role != "brain" && *role != "dev") || *handoff == "" {
		return exit.New(exit.Usage, "%s", relayPromptUsage)
	}
	if err := requireLease(c, *role, *holder); err != nil {
		return err
	}
	skill, err := needSkill(c)
	if err != nil {
		return err
	}
	path, err := inHandoff(c.Run, *handoff)
	if err != nil {
		return err
	}
	s, err := state.Load(store.Dir{Root: c.Run})
	if err != nil {
		return err
	}
	in := relaySources(*mode, *tr)
	words, folder := carriedFlags(in.Flags), filepath.Join(c.Run, "dev")
	if *role == "dev" {
		words = append(withoutFlag(words, "--effort"), "--effort", "medium")
	} else if in.Mode == "" {
		return exit.New(exit.Refused, "relay prompt: no permission mode found for this session, so pass --mode <mode>")
	} else if folder, err = runHome(c.Run); err != nil {
		return err
	} else if e := os.Getenv("CLAUDE_EFFORT"); e != "" && flagValue(words, "--effort") == "" {
		words = append(words, "--effort", e)
	}
	if in.Model != "" && flagValue(words, "--model") == "" {
		words = append(words, "--model", modelAlias(in.Model))
	}
	words = append(words, "--permission-mode", firstSet(in.Mode, "acceptEdits"))
	for i, w := range words {
		words[i] = token(w)
	}
	name, prompt := firstSet(in.Name, "source "+s.Slug+" "+*role), strings.TrimSuffix(path, ".md")+".prompt.txt"
	if err := store.WriteFileAtomic(prompt, []byte(relayKickoff(*role, s.Slug, name, *holder, c.Run, skill, path)), 0o644); err != nil {
		return err
	}
	_, err = fmt.Fprintln(c.Stdout, relayCommand(folder, nextName(name), words, relayKickoffLine(*role, s.Slug, name, *holder), prompt))
	return err
}

// withoutFlag returns flags without the flag name and its value, given as the
// next word or as name=value.
func withoutFlag(flags []string, name string) []string {
	kept := []string{}
	for i := 0; i < len(flags); i++ {
		if flags[i] == name {
			i++
		} else if !strings.HasPrefix(flags[i], name+"=") {
			kept = append(kept, flags[i])
		}
	}
	return kept
}

// inHandoff resolves path to an absolute path with symlinks resolved, and
// returns it when it is a regular .md file directly in <run>/handoff (R21).
func inHandoff(run, path string) (string, error) {
	p, err := filepath.Abs(path)
	if err == nil {
		p, err = filepath.EvalSymlinks(p)
	}
	var fi os.FileInfo
	if err == nil {
		fi, err = os.Stat(p)
	}
	if err != nil || filepath.Dir(p) != filepath.Join(run, "handoff") || !strings.HasSuffix(p, ".md") || !fi.Mode().IsRegular() {
		return "", exit.New(exit.Refused, "relay prompt: %s is not a .md file in %s/handoff", path, run)
	}
	return p, nil
}

const relayMuteUsage = "usage: monet-source relay mute"

// relayMute is relay mute. It sets muted in the hook state of this session
// and prints the state path. It takes no argument and no lease.
func relayMute(c *Ctx, args []string) error {
	if len(args) != 0 {
		return exit.New(exit.Usage, "%s", relayMuteUsage)
	}
	p, err := relay.Mute()
	if err != nil {
		return exit.New(exit.Refused, "%v", err)
	}
	_, err = fmt.Fprintf(c.Stdout, "relay: muted %s\n", p)
	return err
}
