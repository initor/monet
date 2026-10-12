// Package gen fills the engine template with plan data (Contract 8).
package gen

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
)

// PromptKeys is the closed set of prompt keys in reference/roles.md, in print order.
var PromptKeys = []string{
	"common", "implement_new", "implement_resume", "implement_fix", "implement_sync",
	"review_full", "review_fix", "review_merge", "adversarial", "merge", "health", "repair",
}

var (
	headingRe = regexp.MustCompile(`^## +(\S.*?)\s*$`)
	fenceRe   = regexp.MustCompile("^(`{3,})(\\S*)\\s*$")
	varNameRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
)

// ParseVars reads the VARS block of engine.template.js (Contract 0.19): the
// line "const VARS = {", strict JSON, then the line "}". It maps every prompt
// key to the variables the engine fills for it.
func ParseVars(tpl []byte) (map[string][]string, error) {
	lines := strings.Split(strings.ReplaceAll(string(tpl), "\r\n", "\n"), "\n")
	start := slices.Index(lines, "const VARS = {")
	if start < 0 {
		return nil, errors.New("engine template: no line \"const VARS = {\"")
	}
	end := slices.Index(lines[start:], "}")
	if end < 0 {
		return nil, errors.New("engine template: the VARS block has no closing \"}\" line")
	}
	text := "{" + strings.Join(lines[start+1:start+end], "\n") + "}"
	dec := json.NewDecoder(strings.NewReader(text))
	var vars map[string][]string
	if err := dec.Decode(&vars); err != nil {
		return nil, fmt.Errorf("engine template VARS: %w", err)
	}
	if err := dec.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("engine template VARS: trailing data after the JSON object")
	}
	for _, k := range PromptKeys {
		if _, ok := vars[k]; !ok {
			return nil, fmt.Errorf("engine template VARS: no entry for %s", k)
		}
	}
	for k, names := range vars {
		if !slices.Contains(PromptKeys, k) {
			return nil, fmt.Errorf("engine template VARS: unknown prompt key %q", k)
		}
		for _, n := range names {
			if !varNameRe.MatchString(n) {
				return nil, fmt.Errorf("engine template VARS: %s has a bad variable name %q", k, n)
			}
		}
	}
	return vars, nil
}

// ParsePrompts reads roles.md (Contract 9): every "## <key>" heading owns exactly
// one fenced text block, whose content is taken verbatim.
func ParsePrompts(md []byte) (map[string]string, error) {
	known := map[string]bool{}
	for _, k := range PromptKeys {
		known[k] = true
	}
	out := map[string]string{}
	lines := strings.Split(strings.ReplaceAll(string(md), "\r\n", "\n"), "\n")
	key := ""
	for i := 0; i < len(lines); i++ {
		if m := headingRe.FindStringSubmatch(lines[i]); m != nil {
			if key != "" {
				if _, ok := out[key]; !ok {
					return nil, fmt.Errorf("roles.md: ## %s has no text block", key)
				}
			}
			key = m[1]
			if !known[key] {
				return nil, fmt.Errorf("roles.md line %d: unknown prompt key %q", i+1, key)
			}
			if _, dup := out[key]; dup {
				return nil, fmt.Errorf("roles.md line %d: ## %s appears twice", i+1, key)
			}
			continue
		}
		m := fenceRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		end := -1
		for j := i + 1; j < len(lines); j++ {
			if closes(lines[j], len(m[1])) {
				end = j
				break
			}
		}
		if end < 0 {
			return nil, fmt.Errorf("roles.md line %d: fenced block is not closed", i+1)
		}
		body := strings.Join(lines[i+1:end], "\n")
		switch {
		case key == "":
			return nil, fmt.Errorf("roles.md line %d: fenced block before the first prompt heading", i+1)
		case m[2] != "text":
			return nil, fmt.Errorf("roles.md line %d: ## %s block is %q, want text", i+1, key, m[2])
		default:
			if _, ok := out[key]; ok {
				return nil, fmt.Errorf("roles.md line %d: ## %s has a second block", i+1, key)
			}
			out[key] = body
		}
		i = end
	}
	if key != "" {
		if _, ok := out[key]; !ok {
			return nil, fmt.Errorf("roles.md: ## %s has no text block", key)
		}
	}
	var missing []string
	for _, k := range PromptKeys {
		if _, ok := out[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("roles.md: missing prompt keys: %s", strings.Join(missing, ", "))
	}
	return out, nil
}

// closes reports whether line closes a fence opened with n backticks.
func closes(line string, n int) bool {
	t := strings.TrimRight(line, " \t")
	return len(t) >= n && strings.Trim(t, "`") == ""
}
