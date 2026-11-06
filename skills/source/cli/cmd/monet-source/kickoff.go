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

// relayKickoffLine is the first line of the kickoff of a relayed role of run
// slug, which names the session name and holder that it takes over from (R22).
func relayKickoffLine(role, slug, name, holder string) string {
	return kickoffMarker + " " + role + " of run " + slug + ", from session " + name + " (" + holder + ")."
}

// relayKickoff returns the kickoff text of a relayed brain or dev session: the
// rules and role files under skill, the handoff, the run folder, the owner
// words, and the first engine command of the role (R22).
func relayKickoff(role, slug, name, holder, run, skill, handoff string) string {
	title, file, first := "developer", "developer.md", "dev takeover --from "+holder
	if role == "brain" {
		title, file, first = "Brain", "brain.md", "lock acquire brain"
	}
	return relayKickoffLine(role, slug, name, holder) + "\n\n" +
		fmt.Sprintf("The handoff relay started this session, and the session named above has stopped working. This session takes over as the %s of the monet:source run %s. Read %s/reference/rules.md and %s/reference/%s with the Read tool first, and follow them.\n", title, slug, skill, skill, file) +
		fmt.Sprintf("Then read the handoff at %s. The run folder is %s, and the words of the owner are in %s/owner-words.md.\n", handoff, run, run) +
		fmt.Sprintf("Run each engine command as %s/bin/monet-source with --holder <your session id>, and start with %s.\n", run, first)
}

// relayCommand returns the claude --bg command that starts a relayed session
// in folder under name, with words (already shell tokens) and a message of
// line plus a pointer to the kickoff file at prompt.
func relayCommand(folder, name string, words []string, line, prompt string) string {
	parts := append([]string{"(cd", shellQuote(folder), "&& claude --bg --name", shellQuote(name)}, words...)
	msg := line + "\n\nRead " + prompt + " with the Read tool first, and follow it."
	return strings.Join(append(parts, shellQuote(msg)), " ") + ")"
}
