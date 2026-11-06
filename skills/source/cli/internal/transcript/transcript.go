// Package transcript reads the session transcripts of Claude Code loosely.
package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Origin is where a message came from.
type Origin struct {
	Kind string `json:"kind"`
}

// Usage is the token usage of an assistant record.
type Usage struct {
	Input         int `json:"input_tokens"`
	CacheCreation int `json:"cache_creation_input_tokens"`
	CacheRead     int `json:"cache_read_input_tokens"`
}

// Message is the message of a transcript entry.
type Message struct {
	Content json.RawMessage `json:"content"`
	Usage   *Usage          `json:"usage"`
	Model   string          `json:"model"`
}

// Attachment is the attachment of a transcript entry, such as a queued command.
type Attachment struct {
	Type   string          `json:"type"`
	Origin *Origin         `json:"origin"`
	Prompt json.RawMessage `json:"prompt"`
}

// Entry is one line of a transcript. Unknown fields are ignored.
type Entry struct {
	Type           string      `json:"type"`
	UUID           string      `json:"uuid"`
	ParentUUID     string      `json:"parentUuid"`
	PromptID       string      `json:"promptId"`
	Timestamp      string      `json:"timestamp"`
	IsSidechain    bool        `json:"isSidechain"`
	IsMeta         bool        `json:"isMeta"`
	Origin         *Origin     `json:"origin"`
	PromptSource   string      `json:"promptSource"`
	Message        *Message    `json:"message"`
	Attachment     *Attachment `json:"attachment"`
	AgentName      string      `json:"agentName"`
	CustomTitle    string      `json:"customTitle"`
	PermissionMode string      `json:"permissionMode"`
}

// Find returns flag when it names an existing file, else the first
// <CLAUDE_CONFIG_DIR or ~/.claude>/projects/<dir>/<session>.jsonl in name order.
func Find(flag, session string) (string, error) {
	if flag != "" {
		if _, err := os.Stat(flag); err != nil {
			return "", fmt.Errorf("no transcript at %s", flag)
		}
		return flag, nil
	}
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".claude")
	}
	projects := filepath.Join(base, "projects")
	if session == "" {
		return "", fmt.Errorf("no transcript for session (CLAUDE_CODE_SESSION_ID is not set) under %s", projects)
	}
	dirs, _ := os.ReadDir(projects)
	for _, d := range dirs {
		p := filepath.Join(projects, d.Name(), session+".jsonl")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no transcript for session %s under %s", session, projects)
}

// Each calls fn for each line of path that decodes into an Entry, and skips
// blank lines and lines that do not decode. Lines have no length limit.
func Each(path string, fn func(Entry)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		var e Entry
		if json.Unmarshal(line, &e) == nil {
			fn(e)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
