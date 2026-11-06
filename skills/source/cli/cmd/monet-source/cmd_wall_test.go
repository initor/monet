package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/item"
	"source/internal/state"
	"source/internal/store"
	"source/internal/words"
)

// The rows run in order on one run where h1 has brain and d1 has dev. A brain holder opens a wall
// by brain and renews brain, a dev holder opens one by dev and renews dev, and another holder, a
// bad stdin, or a positional argument writes no wall. WALL W1 and WALL W2 go to both roles.
func TestWallOpen(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	in := `{"question": "Keep the v1 layout?", "default": "keep it", "blocks": "design"}`
	leases := map[string]time.Time{"brain": testNow.Add(leaseTTL), "dev": testNow.Add(leaseTTL)}
	for i, tc := range []struct {
		args  []string
		stdin string
		code  int
		id    string // the wall it prints and saves, or none
		by    string // the lease it renews and the By of the wall
	}{
		{[]string{"--holder", "h1"}, in, exit.OK, "W1", "brain"},
		{[]string{"--holder", "d1"}, in, exit.OK, "W2", "dev"},
		{[]string{"--holder", "x9"}, in, exit.Held, "", ""},
		{[]string{"--holder", "d1"}, `{"question": "q", "status": "cleared"}`, exit.Refused, "", "dev"},
		{[]string{"--holder", "h1"}, in + "{}", exit.Refused, "", "brain"},
		{[]string{"x", "--holder", "h1"}, in, exit.Usage, "", ""},
	} {
		now := testNow.Add(time.Duration(i+1) * time.Hour)
		out, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(tc.stdin), Now: func() time.Time { return now }}, append([]string{"wall", "open"}, tc.args...)...)
		if tc.by != "" {
			leases[tc.by] = now.Add(leaseTTL)
		}
		if want := tc.id + "\n"; code != tc.code || (tc.id != "" && (out != want || e != "")) || (tc.id == "" && out != "") {
			t.Errorf("row %d %q: code %d, stdout %q, stderr %q; want %d and %q", i, tc.args, code, out, e, tc.code, tc.id)
		}
		if tc.id != "" {
			var w item.Wall
			want := item.Wall{ID: tc.id, By: tc.by, Question: "Keep the v1 layout?", Default: "keep it", Blocks: "design", Status: "open"}
			if err := item.Load(d, tc.id, &w); err != nil || w != want {
				t.Errorf("row %d: %s is %+v (err %v); want %+v", i, tc.id, w, err, want)
			}
		}
		checkLease(t, run, "brain", leases["brain"])
		checkLease(t, run, "dev", leases["dev"])
	}
	if _, err := os.Stat(item.Path(d, "W3")); !os.IsNotExist(err) {
		t.Errorf("W3 stat err %v; want no W3", err)
	}
	if s, err := state.Load(d); err != nil || s.Walls != 2 {
		t.Errorf("walls %d, load err %v; want 2", s.Walls, err)
	}
	for _, role := range []string{"brain", "dev"} {
		lines, _, err := events.For(d, role, 0)
		var walls []string
		for _, l := range lines {
			if strings.HasPrefix(l, "WALL ") {
				walls = append(walls, l)
			}
		}
		if want := []string{"WALL W1", "WALL W2"}; err != nil || !reflect.DeepEqual(walls, want) {
			t.Errorf("%s events %q (err %v); want the WALL lines %q", role, lines, err, want)
		}
	}
	if l := readOr(t, d.Log()); !strings.HasSuffix(l, "\tWALL W1\n2026-11-01T14:00:00Z\tWALL W2\n") {
		t.Errorf("log.tsv %q; want WALL W1 and WALL W2 last", l)
	}
}

// The rows run in order on W1, opened by h1, which has brain; d1 has dev. A words id with no
// entry, a wall that is not open, and a holder without brain each leave W1 as it was. One clear
// saves W1 as cleared with w2, renews brain only, and records CLEARED W1 w2 to both roles.
func TestWallClear(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	in := `{"question": "Keep the v1 layout?", "default": "keep it", "blocks": "design"}`
	if out, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(in)}, "wall", "open", "--holder", "h1"); code != exit.OK || out != "W1\n" {
		t.Fatalf("wall open: code %d, stdout %q, stderr %q", code, out, e)
	}
	if err := words.Append(d.Words(), words.Entry{ID: "w2", At: "2026-11-01T12:30:00Z", Text: "Clear it."}); err != nil {
		t.Fatal(err)
	}
	open := item.Wall{ID: "W1", By: "brain", Question: "Keep the v1 layout?", Default: "keep it", Blocks: "design", Status: "open"}
	cleared := open
	cleared.Status, cleared.Words = "cleared", "w2"
	leases := map[string]time.Time{"brain": testNow.Add(leaseTTL), "dev": testNow.Add(leaseTTL)}
	for i, tc := range []struct {
		words, holder string
		code          int
		want          item.Wall
	}{
		{"w9", "h1", exit.Refused, open},
		{"w2", "h1", exit.OK, cleared},
		{"w2", "h1", exit.Refused, cleared},
		{"w2", "d1", exit.Held, cleared},
	} {
		now := testNow.Add(time.Duration(i+1) * time.Hour)
		out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return now }}, "wall", "clear", "W1", "--words", tc.words, "--holder", tc.holder)
		if tc.holder == "h1" {
			leases["brain"] = now.Add(leaseTTL)
		}
		if code != tc.code || out != "" || (code == exit.OK) != (e == "") {
			t.Errorf("row %d: code %d, stdout %q, stderr %q; want %d", i, code, out, e, tc.code)
		}
		var w item.Wall
		if err := item.Load(d, "W1", &w); err != nil || w != tc.want {
			t.Errorf("row %d: W1 is %+v (err %v); want %+v", i, w, err, tc.want)
		}
		checkLease(t, run, "brain", leases["brain"])
		checkLease(t, run, "dev", leases["dev"])
	}
	for _, role := range []string{"brain", "dev"} {
		lines, _, err := events.For(d, role, 0)
		var clears []string
		for _, l := range lines {
			if strings.HasPrefix(l, "CLEARED ") {
				clears = append(clears, l)
			}
		}
		if err != nil || len(lines) == 0 || lines[len(lines)-1] != "CLEARED W1 w2" || len(clears) != 1 {
			t.Errorf("%s events %q (err %v); want one CLEARED W1 w2, last", role, lines, err)
		}
	}
	if l := readOr(t, d.Log()); !strings.HasSuffix(l, "2026-11-01T14:00:00Z\tCLEARED W1 w2\n") {
		t.Errorf("log.tsv %q; want CLEARED W1 w2 last, at the second row", l)
	}
}
