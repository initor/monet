package transcript

import (
	"bytes"
	"cmp"
	"encoding/json"
	"regexp"
	"strings"
)

// command matches a text that starts, after blanks, with the tag of a slash command.
var command = regexp.MustCompile("^[\t\n\v\f\r ]*<(command-message|command-name)>")

// Words is one message that the owner typed, with the id of the newest earlier
// kept message under the same parent, which it replaces.
type Words struct {
	ID, At, Supersedes, Text string
}

// OwnerWords lists the messages that the owner typed in path, in file order, without
// the texts that start with marker or with a slash command, and the ones with no id.
// fromHuman counts every typed message, kept or not (relay.mjs words).
func OwnerWords(path, marker string) (msgs []Words, fromHuman int, err error) {
	byParent := map[string]string{}
	err = Each(path, func(e Entry) {
		text, ok := fromUser(e)
		if !ok {
			return
		}
		fromHuman++
		id := cmp.Or(e.UUID, e.PromptID, e.Timestamp)
		if command.MatchString(text) || strings.HasPrefix(text, marker) || id == "" {
			return
		}
		w := Words{ID: id, At: cmp.Or(e.Timestamp, "unknown"), Text: text}
		if p := e.ParentUUID; p != "" {
			w.Supersedes = byParent[p]
			byParent[p] = id
		}
		msgs = append(msgs, w)
	})
	return msgs, fromHuman, err
}

// textOf gives a JSON string as it is, or the text blocks of a content array
// joined with a blank line. An array with a tool_result block, or with no text
// block that holds a string, gives false (relay.mjs textOf).
func textOf(raw json.RawMessage) (string, bool) {
	// UseNumber keeps a number out of float64 range, such as 1e999, from
	// failing the decode of the whole array, as JSON.parse reads it as Infinity.
	var v any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if d.Decode(&v) != nil {
		return "", false
	}
	switch c := v.(type) {
	case string:
		return c, true
	case []any:
		var parts []string
		for _, b := range c {
			m, _ := b.(map[string]any)
			if m["type"] == "tool_result" {
				return "", false
			}
			if s, ok := m["text"].(string); ok && m["type"] == "text" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n\n"), len(parts) > 0
	}
	return "", false
}

// fromUser gives the text of a main-chain message that the user typed: a user
// entry from a human or typed prompt, or a queued_command attachment from a
// human, which is a message typed while a turn runs (relay.mjs fromUser).
func fromUser(e Entry) (string, bool) {
	if e.IsSidechain || e.IsMeta {
		return "", false
	}
	switch e.Type {
	case "user":
		if (e.Origin == nil || e.Origin.Kind != "human") && e.PromptSource != "typed" || e.Message == nil {
			return "", false
		}
		return textOf(e.Message.Content)
	case "attachment":
		a := e.Attachment
		if a == nil || a.Type != "queued_command" || a.Origin == nil || a.Origin.Kind != "human" {
			return "", false
		}
		return textOf(a.Prompt)
	}
	return "", false
}
