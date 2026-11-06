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
