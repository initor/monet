package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"source/internal/exit"
	"source/internal/lock"
	"source/internal/relay"
	"source/internal/state"
	"source/internal/store"
	"source/internal/transcript"
	"source/internal/words"
)

func init() { commands["relay"] = subcommands("relay", relaySubs) }

// relaySubs maps each relay subcommand to its handler.
var relaySubs = map[string]func(c *Ctx, args []string) error{"mute": relayMute, "prompt": relayPrompt, "words": relayWords}

const relayWordsUsage = "usage: monet-source relay words [--transcript <path> | --comment <id> --at <time>]"

// appendWord appends one entry to owner-words.md. Tests replace it.
var appendWord = words.Append

// relayWords copies the typed messages of the session transcript that
// owner-words.md lacks into it, reading and appending under the words mutex,
// and prints the new and total counts and each new id (R17, R18, R20). It
// takes no lease (R1). With --comment it copies the stdin text instead (R19).
func relayWords(c *Ctx, args []string) error {
	fs := newFlagSet("relay words")
	tr, id, at := fs.String("transcript", "", ""), fs.String("comment", "", ""), fs.String("at", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || (*id == "") != (*at == "") || *id != "" && *tr != "" {
		return exit.New(exit.Usage, "%s", relayWordsUsage)
	}
	var comment transcript.Words
	if *id != "" {
		if _, err := time.Parse(time.RFC3339, *at); err != nil {
			return exit.New(exit.Usage, "%s", relayWordsUsage)
		}
		text, err := io.ReadAll(c.Stdin)
		if err != nil {
			return err
		}
		comment = transcript.Words{ID: *id, At: *at, Text: strings.TrimRight(string(text), "\n")}
		if strings.TrimSpace(comment.Text) == "" {
			return exit.New(exit.Refused, "relay words: the comment text is blank")
		}
	}
	d := store.Dir{Root: c.Run}
	var fresh []words.Entry
	var total int
	err := lock.WithMutex(d.Path("locks", "words.mutex"), func() error {
		have, err := words.Read(d.Words())
		if err != nil {
			return err
		}
		msgs, fromHuman := []transcript.Words{comment}, 1 // a page comment is a message of the user
		if *id == "" {
			p, err := transcript.Find(*tr, relay.SessionID())
			if err != nil {
				return exit.New(exit.Refused, "relay words: %v", err)
			}
			if msgs, fromHuman, err = transcript.OwnerWords(p, kickoffMarker); err != nil {
				return err
			}
		}
		if fromHuman == 0 && len(have) == 0 {
			return exit.New(exit.User, "relay: no message from the user in the transcript, so it is the wrong transcript or its format changed")
		}
		seen := map[string]bool{}
		for _, e := range have {
			seen[e.ID] = true
		}
		for _, m := range msgs {
			if e := words.Entry(m); !seen[e.ID] {
				if err := words.Check(e); err != nil {
					return exit.New(exit.Refused, "relay words: entry %s: %v", e.ID, err)
				}
				seen[e.ID], fresh = true, append(fresh, e)
			}
		}
		for _, e := range fresh {
			if err := appendWord(d.Words(), e); err != nil {
				return err
			}
		}
		total = len(have) + len(fresh)
		return nil
	})
	if err != nil {
		return err
	}
	out := fmt.Sprintf("owner-words: %d new, %d in %s\n", len(fresh), total, d.Words())
	for _, e := range fresh {
		out += e.ID + "\n"
	}
	_, err = io.WriteString(c.Stdout, out)
	return err
}

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
