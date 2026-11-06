package main

import (
	"path/filepath"

	"source/internal/exit"
)

// runHome returns <home>, the folder three levels above a run at
// <home>/.planning/source/<slug>, and refuses any other shape with exit 2.
func runHome(run string) (string, error) {
	src := filepath.Dir(run)
	if filepath.Base(src) != "source" || filepath.Base(filepath.Dir(src)) != ".planning" {
		return "", exit.New(exit.Refused, "run %s is not at <home>/.planning/source/<slug>", run)
	}
	return filepath.Dir(filepath.Dir(src)), nil
}
