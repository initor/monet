package main

import (
	"fmt"
	"strings"
)

// kickoffMarker starts the first line of every kickoff text (R3).
const kickoffMarker = "source relay kickoff:"

// shellQuote returns s in single quotes for a POSIX shell. Each single quote
// in s becomes the four bytes quote, backslash, quote, quote.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// devKickoffLine is the first line of the developer kickoff of run slug.
func devKickoffLine(slug string) string {
	return kickoffMarker + " developer of run " + slug
}

// devKickoff returns the developer kickoff text of run slug at run, which
// names the rules and role files under the skill folder skill (R3).
func devKickoff(slug, run, skill string) string {
	return devKickoffLine(slug) + "\n\n" +
		fmt.Sprintf("You are the developer of the monet:source run %s. Read %s/reference/rules.md and %s/reference/developer.md with the Read tool first, and follow them.\n", slug, skill, skill) +
		fmt.Sprintf("The run folder is %s, and your folder is %s/dev. Run each engine command as %s/bin/monet-source with --holder <your session id>, and start with dev hello.\n", run, run, run)
}

// devStartCommand returns the claude --bg command that starts the developer
// in <run>/dev on the kickoff at kickoffPath (R6).
func devStartCommand(slug, run, kickoffPath string) string {
	msg := devKickoffLine(slug) + ". Read " + kickoffPath + " with the Read tool first, and follow it."
	return "(cd " + shellQuote(run+"/dev") + " && claude --bg --name " + shellQuote("source "+slug+" dev") +
		" --effort medium --permission-mode acceptEdits " + shellQuote(msg) + ")"
}
