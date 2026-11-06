package main

import (
	"cmp"
	"os"
	"reflect"
	"testing"
	"time"

	"source/internal/events"
	"source/internal/store"
)

// record emits each line once and logs it at the clock of c in UTC with nanoseconds.
func TestRecordWritesEventAndLog(t *testing.T) {
	d, bad := store.Dir{Root: realTempDir(t)}, store.Dir{Root: realTempDir(t)}
	os.WriteFile(bad.Events(), []byte("bad\n"), 0o644)
	at := func(now time.Time) *Ctx { return &Ctx{Run: d.Root, Now: func() time.Time { return now }} }
	if err := cmp.Or(record(at(testNow), d, "VERSION research v1"), record(at(testNow.Add(5).In(time.FixedZone("PST", -8*3600))), d, "PHASE 1 design")); err != nil {
		t.Fatal(err)
	}
	s, o := testNow.Format(time.RFC3339Nano), "2026-11-01T12:00:00.000000005Z"
	ev, eerr := store.ReadTSV(d.Events(), events.Header)
	lg, lerr := store.ReadTSV(d.Log(), logHeader)
	want := [][][]string{{{"VERSION research v1", s}, {"PHASE 1 design", o}}, {{s, "VERSION research v1"}, {o, "PHASE 1 design"}}}
	if got := [][][]string{ev, lg}; eerr != nil || lerr != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("events and log %q (%v, %v), want %q", got, eerr, lerr, want)
	}
	err := record(at(testNow), bad, "VERSION research v1")
	if _, serr := os.Stat(bad.Log()); err == nil || !os.IsNotExist(serr) {
		t.Errorf("a broken events.tsv: err %v, log.tsv stat %v; want an error and no log.tsv", err, serr)
	}
}

// recordOnce logs a line only on the call whose emit appended it, at the clock of c in UTC.
func TestRecordOnceLogsOnce(t *testing.T) {
	d, bad := store.Dir{Root: realTempDir(t)}, store.Dir{Root: realTempDir(t)}
	os.WriteFile(bad.Events(), []byte("bad\n"), 0o644)
	c := &Ctx{Run: d.Root, Now: func() time.Time { return testNow.Add(5).In(time.FixedZone("PST", -8*3600)) }}
	first, err1 := recordOnce(c, d, "REPORT_DUE x")
	second, err2 := recordOnce(c, d, "REPORT_DUE x")
	ev, eerr := store.ReadTSV(d.Events(), events.Header)
	lg, lerr := store.ReadTSV(d.Log(), logHeader)
	s := "2026-11-01T12:00:00.000000005Z"
	want := [][][]string{{{"REPORT_DUE x", s}}, {{s, "REPORT_DUE x"}}}
	if got := [][][]string{ev, lg}; !first || second || cmp.Or(err1, err2, eerr, lerr) != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("calls %v, %v (%v, %v); events and log %q (%v, %v), want true, false and %q", first, second, err1, err2, got, eerr, lerr, want)
	}
	ok, err := recordOnce(c, bad, "REPORT_DUE x")
	if _, serr := os.Stat(bad.Log()); ok || err == nil || !os.IsNotExist(serr) {
		t.Errorf("a broken events.tsv: %v, err %v, log.tsv stat %v; want false, an error and no log.tsv", ok, err, serr)
	}
}
