package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
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

// A watch that runs for hours with no page write and no beat records REPORT_DUE and DEV_SILENT in its first late
// pass, once each in events.tsv and log.tsv, and only brain prints them. A pass of either role checks the clocks,
// and a page write starts a new REPORT_DUE key.
func TestWatchTimeEvents(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	runAt := func(dt time.Duration, args ...string) string {
		t.Helper()
		now := testNow.Add(dt)
		out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return now }}, args...)
		if code != exit.OK {
			t.Fatalf("%q at +%v: code %d, stderr %q", args, dt, code, e)
		}
		return out
	}
	// lines reads the lines of events.tsv and of log.tsv.
	lines := func() (evs, logs []string) {
		t.Helper()
		er, err1 := store.ReadTSV(d.Events(), events.Header)
		lr, err2 := store.ReadTSV(d.Log(), logHeader)
		if err1 != nil || err2 != nil {
			t.Fatal(err1, err2)
		}
		for _, r := range er {
			evs = append(evs, r[0])
		}
		for _, r := range lr {
			logs = append(logs, r[1])
		}
		return evs, logs
	}
	brain, dev := []string{"watch", "--role", "brain", "--once"}, []string{"watch", "--role", "dev", "--once"}
	due, silent, due2 := "REPORT_DUE 2026-11-01T12:00:00Z", "DEV_SILENT d1 2026-11-01T12:00:00Z", "REPORT_DUE 2026-11-01T16:21:00Z"
	_, logs0 := lines()
	if got := runAt(59*time.Minute+59*time.Second, brain...); got != "" {
		t.Fatalf("brain pass at +59m59s printed %q", got)
	}
	if evs, _ := lines(); len(evs) != 0 {
		t.Fatalf("brain pass at +59m59s recorded %q", evs)
	}
	for k := 61; k <= 260; k++ {
		want := ""
		if k == 61 {
			want = due + "\n" + silent + "\n"
		}
		if got := runAt(time.Duration(k)*time.Minute, brain...); got != want {
			t.Fatalf("brain pass at +%dm printed %q, want %q", k, got, want)
		}
	}
	if evs, logs := lines(); !reflect.DeepEqual(evs, []string{due, silent}) || !reflect.DeepEqual(logs, slices.Concat(logs0, []string{due, silent})) {
		t.Fatalf("after 200 passes: events %q, log %q", evs, logs)
	}
	_, logs := lines()
	runAt(261*time.Minute, "report", "done", "--at", testNow.Add(261*time.Minute).Format(time.RFC3339), "--row", strconv.Itoa(len(logs)), "--holder", "h1")
	if got := runAt(321*time.Minute, dev...); got != "" {
		t.Fatalf("dev pass printed %q", got)
	}
	if evs, _ := lines(); !reflect.DeepEqual(evs, []string{due, silent, due2}) {
		t.Fatalf("the dev pass left events %q, want the new REPORT_DUE recorded", evs)
	}
	if got := runAt(321*time.Minute, brain...); got != due2+"\n" {
		t.Fatalf("brain pass after report done printed %q", got)
	}
	if evs, logs := lines(); !reflect.DeepEqual(evs, []string{due, silent, due2}) || !reflect.DeepEqual(logs, slices.Concat(logs0, []string{due, silent, due2})) {
		t.Fatalf("after report done: events %q, log %q", evs, logs)
	}
	w3, row := 330*time.Minute+30*time.Second+7, strconv.Itoa(len(logs0)+3)
	runAt(w3, "report", "done", "--at", testNow.Add(w3).Format(time.RFC3339Nano), "--row", row, "--holder", "h1")
	if got := runAt(w3+time.Hour-1, brain...); got != "" {
		t.Fatalf("brain pass 1 ns before the third clock printed %q", got)
	}
	if got := runAt(w3+time.Hour, brain...); got != "REPORT_DUE 2026-11-01T17:30:30.000000007Z\n" {
		t.Fatalf("brain pass on the third clock printed %q", got)
	}
	if store.WriteFileAtomic(d.State(), []byte(`{"version": 1, "slug": "demo", "phase": "delivered", "sealed": {"words": "w1", "at": "2026-11-01T12:00:00Z", "hash": "h"}, "dev": "d1", "last_beat": "2026-11-01T20:00:00Z"}`), 0o644) != nil || runAt(10*time.Hour, brain...) != "" {
		t.Fatal("a brain pass 2 hours after a beat of a delivered run printed a time event")
	}
}

// A pass fails when a due event cannot reach events.tsv or log.tsv, and when state.json does not decode.
func TestWatchTimeEventFailures(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	late := func() time.Time { return testNow.Add(2 * time.Hour) }
	emitAll(t, d, "VERSION research v1")
	if err := os.Chmod(d.Events(), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(d.Events(), 0o644) })
	if out, e, code := runSource(t, &Ctx{Run: run, Now: late}, "watch", "--role", "brain", "--once"); code != exit.User || out != "" || !strings.Contains(e, "events.tsv") {
		t.Fatalf("read-only events.tsv: code %d, stdout %q, stderr %q", code, out, e)
	}
	if err := os.Chmod(d.Events(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(d.Log(), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(d.Log(), 0o644) })
	if out, e, code := runSource(t, &Ctx{Run: run, Now: late}, "watch", "--role", "brain", "--once"); code != exit.User || out != "" || !strings.Contains(e, "log.tsv") {
		t.Fatalf("read-only log.tsv: code %d, stdout %q, stderr %q", code, out, e)
	}
	if err := store.WriteFileAtomic(d.State(), []byte(`{"x": 1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, e, code := runSource(t, &Ctx{Run: run, Now: late}, "watch", "--role", "dev", "--once"); code != exit.User || out != "" || !strings.Contains(e, "unknown field") {
		t.Fatalf("bad state.json: code %d, stdout %q, stderr %q", code, out, e)
	}
}

// A beat after the seal comes due later: the pass at +91m records DEV_SILENT of that beat, although REPORT_DUE of
// the seal is already in events.tsv.
func TestWatchDevSilentAfterReportDue(t *testing.T) {
	run := newLoopRun(t)
	at := func(dt time.Duration, args ...string) string {
		t.Helper()
		now := testNow.Add(dt)
		out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return now }}, args...)
		if code != exit.OK {
			t.Fatalf("%q at +%v: code %d, stderr %q", args, dt, code, e)
		}
		return out
	}
	brain := []string{"watch", "--role", "brain", "--once"}
	at(30*time.Minute, "dev", "beat", "--holder", "d1")
	if got := at(61*time.Minute, brain...); got != "REPORT_DUE 2026-11-01T12:00:00Z\n" {
		t.Fatalf("pass at +61m printed %q", got)
	}
	if got := at(91*time.Minute, brain...); got != "DEV_SILENT d1 2026-11-01T12:30:00Z\n" {
		t.Fatalf("pass at +91m printed %q", got)
	}
}

// On a run made by init and dev hello alone, with no charter.json and no log.tsv, a pass 60 minutes later records DEV_SILENT.
func TestWatchDevSilentBeforeTheSeal(t *testing.T) {
	if out, e, code := runSource(t, &Ctx{Run: runDev(t, []devStep{{args: "hello --holder d1", dev: "d1", lease: testNow.Add(leaseTTL)}}), Now: func() time.Time { return testNow.Add(time.Hour) }}, "watch", "--role", "brain", "--once"); code != exit.OK || out != "DEV_SILENT d1 2026-11-01T12:00:00Z\n" {
		t.Fatalf("init and dev hello, then watch at +60m: code %d, stdout %q, stderr %q", code, out, e)
	}
}
