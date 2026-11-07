package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// excludeLines are the lines that init keeps in <home>/.git/info/exclude (R8).
var excludeLines = []string{"/.planning/source/", "/.claude/handoff-relay.json"}

// excludeText returns the text to append to an exclude file that holds old: each
// excludeLines entry that no line of old equals, with a newline, after one newline
// when old is not empty and does not end in one. It is "" when nothing is missing.
func excludeText(old string) string {
	have, add := strings.Split(old, "\n"), ""
	for _, w := range excludeLines {
		if !slices.Contains(have, w) {
			add += w + "\n"
		}
	}
	if add != "" && old != "" && !strings.HasSuffix(old, "\n") {
		add = "\n" + add
	}
	return add
}

// excludeRun appends excludeText of <home>/.git/info/exclude to that file in one
// O_APPEND write, making info/ when missing. It does nothing unless <home>/.git
// is a folder, so a linked worktree gets nothing, and it runs no git command.
func excludeRun(home string) error {
	git := filepath.Join(home, ".git")
	if fi, err := os.Lstat(git); errors.Is(err, os.ErrNotExist) || err == nil && !fi.IsDir() {
		return nil
	} else if err != nil {
		return err
	}
	p := filepath.Join(git, "info", "exclude")
	old, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := excludeText(string(old))
	if text == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
