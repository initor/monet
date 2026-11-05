package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/store"
)

// wantWatchUsage is the watch usage line, typed out so that a change fails a test.
const wantWatchUsage = "usage: monet-source watch --role brain|dev [--interval 30s] [--once]"

// emitAll records each line in d as one event.
func emitAll(t *testing.T, d store.Dir, lines ...string) {
	t.Helper()
	for _, l := range lines {
		if _, err := events.Emit(d, l, testNow); err != nil {
			t.Fatal(err)
		}
	}
}

// A pass prints the lines after the cursor <run>/watch/<role>.json, which is 0
// when missing, then stores {"row": <n>} there, so each pass prints only the
// lines that are new since the last one.
func TestWatchResumesFromCursor(t *testing.T) {
	run := t.TempDir()
	d := store.Dir{Root: run}
	emitAll(t, d, "VERSION research v1", "WALL W1")
	once := func() string {
		out, e, code := runSource(t, &Ctx{Run: run}, "watch", "--role", "brain", "--once")
		if code != exit.OK {
			t.Fatalf("watch exit %d, stderr %q", code, e)
		}
		return out
	}
	if got := once(); got != "VERSION research v1\nWALL W1\n" {
		t.Fatalf("first pass %q", got)
	}
	var cur struct {
		Row int `json:"row"`
	}
	if err := store.DecodeStrict(filepath.Join(run, "watch", "brain.json"), &cur); err != nil || cur.Row != 2 {
		t.Fatalf("cursor %+v, err %v", cur, err)
	}
	emitAll(t, d, "RESPONSE research v1")
	if got := once(); got != "RESPONSE research v1\n" {
		t.Fatalf("second pass %q", got)
	}
	if got := once(); got != "" {
		t.Fatalf("third pass %q", got)
	}
}

// Each role reads its own routes through its own cursor, and a role other than
// brain or dev is a usage error.
func TestWatchRole(t *testing.T) {
	run := t.TempDir()
	emitAll(t, store.Dir{Root: run}, "VERSION research v1", "FINDINGS f1", "WALL W1")
	for _, tc := range []struct{ role, want string }{
		{"dev", "FINDINGS f1\nWALL W1\n"},
		{"brain", "VERSION research v1\nWALL W1\n"},
		{"dev", ""}, // the dev cursor is row 3, not the 2 lines it printed
	} {
		if out, e, code := runSource(t, &Ctx{Run: run}, "watch", "--role", tc.role, "--once"); code != exit.OK || out != tc.want {
			t.Errorf("%s: code %d, stdout %q, stderr %q", tc.role, code, out, e)
		}
	}
	if out, e, code := runSource(t, &Ctx{Run: run}, "watch", "--role", "planner", "--once"); code != exit.Usage || out != "" || e != "monet-source: "+wantWatchUsage+"\n" {
		t.Fatalf("planner: code %d, stdout %q, stderr %q", code, out, e)
	}
}

// watch takes no positional argument, needs --role, and needs a positive
// --interval, also with --once.
func TestWatchUsage(t *testing.T) {
	run := t.TempDir()
	positive := "watch: --interval must be positive"
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--once"}, wantWatchUsage},
		{[]string{"--role", "brain", "--once", "x"}, wantWatchUsage},
		{[]string{"--role", "brain", "--once", "--interval", "0s"}, positive},
		{[]string{"--role", "brain", "--once", "--interval", "-1s"}, positive},
	} {
		out, e, code := runSource(t, &Ctx{Run: run}, append([]string{"watch"}, tc.args...)...)
		if code != exit.Usage || out != "" || e != "monet-source: "+tc.want+"\n" {
			t.Errorf("%q: code %d, stdout %q, stderr %q", tc.args, code, out, e)
		}
	}
}

// opWriter records each write and flush in ops, and fails op number failAt
// (from 1), as a reader that goes away does.
type opWriter struct {
	ops    []string
	failAt int
}

func (w *opWriter) op(s string) error {
	w.ops = append(w.ops, s)
	if len(w.ops) == w.failAt {
		return errors.New("reader gone")
	}
	return nil
}

func (w *opWriter) Write(p []byte) (int, error) { return len(p), w.op(string(p)) }
func (w *opWriter) Flush() error                { return w.op("flush") }

// A pass flushes after each line, and a failed write or flush leaves the
// cursor as it was, so the next pass prints that line again instead of
// skipping it.
func TestWatchFlushesEachLine(t *testing.T) {
	run := t.TempDir()
	emitAll(t, store.Dir{Root: run}, "VERSION research v1", "WALL W1")
	all := []string{"VERSION research v1\n", "flush", "WALL W1\n", "flush"}
	for _, failAt := range []int{2, 3, 0} {
		w := &opWriter{failAt: failAt}
		err := cmdWatch(&Ctx{Run: run, Stdout: w, Stderr: io.Discard}, []string{"--role", "brain", "--once"})
		want := all
		if failAt > 0 {
			want = all[:failAt]
		}
		if (err != nil) != (failAt > 0) || !reflect.DeepEqual(w.ops, want) {
			t.Fatalf("failAt %d: err %v, ops %q", failAt, err, w.ops)
		}
	}
}

// A cursor past the events (kept as it was) or a failed cursor write, also on a pass with no new line, is exit 1.
func TestWatchFailedPass(t *testing.T) {
	run := t.TempDir()
	emitAll(t, store.Dir{Root: run}, "VERSION a", "RESPONSE r1")
	cursor, stale := filepath.Join(run, "watch", "brain.json"), `{"row": 5}`
	if err := store.WriteFileAtomic(cursor, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	brain := []string{"watch", "--role", "brain", "--once"}
	out, e, code := runSource(t, &Ctx{Run: run}, brain...)
	if b, err := os.ReadFile(cursor); code != exit.User || out != "" || !strings.Contains(e, "events row 5") || err != nil || string(b) != stale {
		t.Fatalf("stale cursor: code %d, stdout %q, stderr %q, cursor %q", code, out, e, b)
	}
	if err := os.Remove(cursor); err != nil || os.Chmod(filepath.Dir(cursor), 0o555) != nil {
		t.Fatal("cannot make the watch folder read-only")
	}
	t.Cleanup(func() { os.Chmod(filepath.Dir(cursor), 0o755) })
	if _, e, code := runSource(t, &Ctx{Run: run}, "watch", "--role", "dev", "--once"); code != exit.User || e == "" {
		t.Fatalf("idle dev pass, read-only watch folder: code %d, stderr %q", code, e)
	}
}

// stopWatch is the panic value that ends a watch loop from a fake watchSleep.
type stopWatch struct{}

// Without --once, watch passes again after --interval (default 30s). Each
// failed pass in a row prints its error and doubles the delay, up to 8 times,
// and a good pass resets the delay.
func TestWatchLoopBacksOff(t *testing.T) {
	run := t.TempDir()
	emitAll(t, store.Dir{Root: run}, "VERSION a")
	cursor := filepath.Join(run, "watch", "brain.json")
	// The unknown field fails the strict decode, so each pass fails.
	if err := store.WriteFileAtomic(cursor, []byte(`{"row": 0, "x": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := watchSleep
	t.Cleanup(func() { watchSleep = old })
	var slept []time.Duration
	stopAt := 5
	watchSleep = func(d time.Duration) {
		slept = append(slept, d)
		if len(slept) == 4 {
			os.Remove(cursor) // the next pass succeeds
		}
		if len(slept) == stopAt {
			panic(stopWatch{})
		}
	}
	var out, errb bytes.Buffer
	loop := func(args ...string) (r any) {
		defer func() {
			if p := recover(); p != nil {
				r = p
			}
		}()
		return cmdWatch(&Ctx{Run: run, Stdout: &out, Stderr: &errb}, append([]string{"--role", "brain"}, args...))
	}
	m := time.Minute
	if r := loop(); r != (stopWatch{}) || !reflect.DeepEqual(slept, []time.Duration{m, 2 * m, 4 * m, 4 * m, m / 2}) ||
		out.String() != "VERSION a\n" || strings.Count(errb.String(), "monet-source watch: ") != 4 || !strings.Contains(errb.String(), "unknown field") {
		t.Fatalf("end %v, slept %v, stdout %q, stderr %q", r, slept, out.String(), errb.String())
	}
	slept, stopAt = nil, 1
	if r := loop("--interval", "5s"); r != (stopWatch{}) || !reflect.DeepEqual(slept, []time.Duration{5 * time.Second}) || reflect.ValueOf(old).Pointer() != reflect.ValueOf(time.Sleep).Pointer() {
		t.Fatalf("--interval 5s: end %v, slept %v", r, slept)
	}
}
