package main

import (
	"math/big"
	"regexp"
	"slices"
	"strings"
)

// The respawn flags that a relayed session must not reuse: the relay sets its
// own name and mode, and the next session is a new one, not a resumed one.
var droppedWithValue = []string{"--name", "-n", "--permission-mode", "--session-id", "--resume", "-r"}
var droppedAlone = []string{"--continue", "-c", "--fork-session", "--inherit-permission-mode"}

// modelAliases are the short model names that claude --model takes.
var modelAliases = []string{"opus", "sonnet", "haiku", "fable"}

var plainToken = regexp.MustCompile(`^[A-Za-z0-9._=-]+$`)

// numberedName differs from relay.mjs only for \r, \v, or Unicode spaces
// (RE2 . matches \r, RE2 \s skips the others); session names hold none.
var numberedName = regexp.MustCompile(`^(.*\S) (\d+)$`)

// token returns s as it is when it needs no shell quotes, else shellQuote(s).
func token(s string) string {
	if plainToken.MatchString(s) {
		return s
	}
	return shellQuote(s)
}

// nextName raises a trailing number of name by one, or adds " 2".
func nextName(name string) string {
	m := numberedName.FindStringSubmatch(name)
	if m == nil {
		return name + " 2"
	}
	n, _ := new(big.Int).SetString(m[2], 10)
	return m[1] + " " + n.Add(n, big.NewInt(1)).String()
}

// flagValue returns the value of the flag name in flags, given as the next
// word or as name=value, else "".
func flagValue(flags []string, name string) string {
	if i := slices.Index(flags, name); i >= 0 && i+1 < len(flags) {
		return flags[i+1]
	}
	for _, f := range flags {
		if v, ok := strings.CutPrefix(f, name+"="); ok {
			return v
		}
	}
	return ""
}

// carriedFlags keeps the respawn flags of the job that the next session
// needs, such as --model and --plugin-dir.
func carriedFlags(flags []string) []string {
	dropped := slices.Concat(droppedWithValue, droppedAlone)
	joined := func(f string) bool {
		return slices.ContainsFunc(dropped, func(d string) bool { return strings.HasPrefix(f, d+"=") })
	}
	kept := []string{}
	for i := 0; i < len(flags); i++ {
		switch f := flags[i]; {
		case slices.Contains(droppedWithValue, f):
			i++
		case slices.Contains(droppedAlone, f), joined(f):
		default:
			kept = append(kept, f)
		}
	}
	return kept
}

// modelAlias turns a model id such as claude-opus-5-5 into its alias, or
// returns id when no alias names it.
func modelAlias(id string) string {
	for _, a := range modelAliases {
		if strings.Contains(id, "-"+a+"-") || id == a {
			return a
		}
	}
	return id
}
