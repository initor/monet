package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"source/internal/relay"
	"source/internal/transcript"
)

// relayInputs are the mode, name, model, and job flags of the session that a
// relay command restarts.
type relayInputs struct {
	Mode, Name, Model string
	Flags             []string
}

// relaySources reads the job state, the transcript, and the hook mode loosely,
// and takes the mode and the name from the first source that holds one (R24).
// A missing or broken source gives none, so it never fails.
func relaySources(modeFlag, transcriptFlag string) relayInputs {
	name, flags := jobState()
	var f transcript.Fields
	if p, err := transcript.Find(transcriptFlag, relay.SessionID()); err == nil {
		f, _ = transcript.Newest(p)
	}
	return relayInputs{
		Mode:  firstSet(modeFlag, relay.HookMode(), flagValue(flags, "--permission-mode"), f.PermissionMode),
		Name:  firstSet(name, f.AgentName, f.CustomTitle),
		Model: f.Model,
		Flags: flags,
	}
}

// jobState returns the name and the respawnFlags of $CLAUDE_JOB_DIR/state.json.
// No job dir, no file, or no JSON object gives none, and a field of another
// type gives none for that field.
func jobState() (name string, flags []string) {
	dir := os.Getenv("CLAUDE_JOB_DIR")
	if dir == "" {
		return "", nil
	}
	var m map[string]json.RawMessage
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil || json.Unmarshal(data, &m) != nil {
		return "", nil
	}
	_ = json.Unmarshal(m["name"], &name) // another type leaves name empty
	if json.Unmarshal(m["respawnFlags"], &flags) != nil {
		flags = nil // a list that holds a non-string keeps an empty string for it
	}
	return name, flags
}
