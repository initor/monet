package main

import (
	"errors"
	"path/filepath"
	"testing"

	"source/internal/exit"
)

// Over every path /r/<p> of 1 to 4 segments from a fixed set, runHome returns
// the folder three levels up exactly when p ends in .planning, source, <slug>,
// and otherwise refuses with exit 2.
func TestRunHome(t *testing.T) {
	segs := []string{".planning", "source", "demo", ".plan", "sources", "x"}
	level, paths := [][]string{{}}, [][]string{}
	for n := 1; n <= 4; n++ {
		var next [][]string
		for _, p := range level {
			for _, s := range segs {
				next = append(next, append(append([]string{}, p...), s))
			}
		}
		paths, level = append(paths, next...), next
	}
	if len(paths) != 6+36+216+1296 {
		t.Fatalf("made %d paths", len(paths))
	}
	for _, p := range paths {
		n, run := len(p), filepath.Join(append([]string{"/r"}, p...)...)
		home, err := runHome(run)
		var ee *exit.Error
		if n >= 3 && p[n-3] == ".planning" && p[n-2] == "source" {
			if want := filepath.Join(append([]string{"/r"}, p[:n-3]...)...); err != nil || home != want {
				t.Errorf("%s: home %q, err %v; want %q", run, home, err, want)
			}
		} else if !errors.As(err, &ee) || ee.Code != exit.Refused || ee.Msg != "run "+run+" is not at <home>/.planning/source/<slug>" || home != "" {
			t.Errorf("%s: home %q, err %#v; want exit 2", run, home, err)
		}
	}
}
