package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/exit"
	"source/internal/store"
)

// decision add renews the dev lease and appends [at, dev, text, reason] to
// decisions.tsv. A missing text or reason is exit 64, another holder exit 4,
// and a failed call writes no row.
func TestDecisionAdd(t *testing.T) {
	run := newRun(t, "demo")
	for _, args := range []string{"init --slug demo --holder h1", "dev hello --holder d1"} {
		if _, e, code := runSource(t, &Ctx{Run: run}, strings.Fields(args)...); code != exit.OK {
			t.Fatalf("%s: code %d, stderr %q", args, code, e)
		}
	}
	add := func(at time.Duration, code int, stderr string, args ...string) {
		t.Helper()
		out, e, got := runSource(t, &Ctx{Run: run, Now: func() time.Time { return testNow.Add(at) }}, append([]string{"decision", "add"}, args...)...)
		if got != code || out != "" || (stderr == "") != (e == "") || !strings.Contains(e, stderr) {
			t.Errorf("%q: code %d, stdout %q, stderr %q, want %d with %q", args, got, out, e, code, stderr)
		}
	}
	rows := func(want ...[]string) {
		t.Helper()
		got, err := store.ReadTSV(store.Dir{Root: run}.Path("decisions.tsv"), []string{"at", "by", "decision", "reason"})
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("decisions.tsv: %q, err %v, want %q", got, err, want)
		}
	}
	const usage = "usage: monet-source decision add <text> --reason <reason> --holder <session>"
	add(0, exit.Usage, usage, "use the v2 layout", "--holder", "d1")
	add(0, exit.Usage, usage, "use the v2 layout", "--reason", "", "--holder", "d1")
	add(0, exit.Usage, usage, "--reason", "the reader asked", "--holder", "d1")
	add(0, exit.Usage, usage, "", "--reason", "the reader asked", "--holder", "d1")
	add(0, exit.Usage, usage, "a", "b", "--reason", "the reader asked", "--holder", "d1")
	add(0, exit.Usage, "--holder <session> is required", "use the v2 layout", "--reason", "the reader asked")
	add(0, exit.Held, "lock held: dev is not held by h1", "use the v2 layout", "--reason", "the reader asked", "--holder", "h1")
	rows()
	checkLease(t, run, "dev", testNow.Add(12*time.Hour))
	add(0, exit.OK, "", "use the v2 layout", "--reason", "the reader asked", "--holder", "d1")
	first := []string{testNow.Format(time.RFC3339Nano), "dev", "use the v2 layout", "the reader asked"}
	rows(first)
	add(time.Hour, exit.OK, "", "--holder", "d1", "--reason", "it reads faster", "keep one page")
	second := []string{testNow.Add(time.Hour).Format(time.RFC3339Nano), "dev", "keep one page", "it reads faster"}
	rows(first, second)
	checkLease(t, run, "dev", testNow.Add(13*time.Hour))
	pst := &Ctx{Run: run, Now: func() time.Time { return testNow.Add(2 * time.Hour).In(time.FixedZone("PST", -8*3600)) }}
	if err := logDecision(pst, store.Dir{Root: run}, "brain", "go back", "a gap"); err != nil {
		t.Fatal(err)
	}
	rows(first, second, []string{testNow.Add(2 * time.Hour).Format(time.RFC3339Nano), "brain", "go back", "a gap"})
}
