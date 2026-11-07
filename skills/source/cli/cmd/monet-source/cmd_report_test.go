package main

import (
	"cmp"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
)

const wantReportUsage = "monet-source: usage: monet-source report [done --at <time> --row <n> --holder <session>]\n"

// report prints buildPacket as two-space JSON that decodes strictly into the same packet. An argument is exit 64,
// and a run that buildPacket cannot read is exit 2.
func TestReportCommand(t *testing.T) {
	run := newLoopRun(t)
	submitFile(t, run, "a.md", "one\n")
	want, err := buildPacket(store.Dir{Root: run})
	if err != nil {
		t.Fatal(err)
	}
	out, e, code := runSource(t, &Ctx{Run: run}, "report")
	var got packet
	dec := json.NewDecoder(strings.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); code != exit.OK || err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("report: code %d, stderr %q, decode %v, got %+v, want %+v", code, e, err, got, want)
	}
	if indented, err := json.MarshalIndent(want, "", "  "); err != nil || out != string(indented)+"\n" {
		t.Fatalf("report printed %q, want %q (err %v)", out, indented, err)
	}
	if out, e, code := runSource(t, &Ctx{Run: run}, "report", "x"); code != exit.Usage || out != "" || e != wantReportUsage {
		t.Fatalf("report x: code %d, stdout %q, stderr %q", code, out, e)
	}
	if out, e, code := runSource(t, &Ctx{Run: t.TempDir()}, "report"); code != exit.Refused || out != "" || !strings.Contains(e, "report: ") {
		t.Fatalf("report on an empty dir: code %d, stdout %q, stderr %q", code, out, e)
	}
}

// report done applies R30 under the brain lease, stores the page write in UTC, and writes no log row and no event.
func TestReportDone(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	submitFile(t, run, "a.md", "one\n")
	submitFile(t, run, "a.md", "two\n")
	counts := func() [2]int {
		t.Helper()
		logs, err1 := store.ReadTSV(d.Log(), logHeader)
		evs, err2 := store.ReadTSV(d.Events(), events.Header)
		if err := cmp.Or(err1, err2); err != nil {
			t.Fatal(err)
		}
		return [2]int{len(logs), len(evs)}
	}
	before := counts()
	if before[0] != 3 {
		t.Fatalf("log.tsv has %d rows, want 3", before[0])
	}
	at := func(dt time.Duration) string { return testNow.Add(dt).Format(time.RFC3339Nano) }
	pst := testNow.In(time.FixedZone("PST", -8*3600)).Format(time.RFC3339)
	if _, e, code := runSource(t, &Ctx{Run: run}, "report", "done", "--at", at(-time.Hour), "--row", "1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("report done an hour before now: code %d, stderr %q", code, e)
	}
	if s, err := state.Load(d); err != nil || s.LastReport == nil || !s.LastReport.Equal(testNow.Add(-time.Hour)) || s.ReportRow != 1 || counts() != before {
		t.Fatalf("report done an hour before now: last_report %v, report_row %d, err %v; want --at %s, 1", s.LastReport, s.ReportRow, err, at(-time.Hour))
	}
	for i, tc := range []struct {
		args    string
		code    int
		stderr  string
		wantRow int
	}{
		{"--at " + at(0) + " --row 3 --holder h1", exit.OK, "", 3},
		{"--at " + at(-time.Minute) + " --row 2 --holder h1", exit.Refused, "is before last_report " + at(0), 3},
		{"--at " + at(time.Minute) + " --row 2 --holder h1", exit.Refused, "is after now " + at(0), 3},
		{"--at " + at(0) + " --row 4 --holder h1", exit.Refused, "--row 4 is above the 3 rows of log.tsv", 3},
		{"--at yesterday --row 2 --holder h1", exit.Usage, wantReportUsage, 3},
		{"--at " + at(0) + " --holder h1", exit.Usage, wantReportUsage, 3},
		{"--at " + at(0) + " --row -1 --holder h1", exit.Usage, wantReportUsage, 3},
		{"--at " + at(0) + " --row 2 --holder h1 x", exit.Usage, wantReportUsage, 3},
		{"--at " + at(0) + " --row 2 --holder d1", exit.Held, "lock held: brain", 3},
		{"--at " + pst + " --row 2 --holder h1", exit.OK, "", 2},
	} {
		out, e, code := runSource(t, &Ctx{Run: run}, append([]string{"report", "done"}, strings.Fields(tc.args)...)...)
		if code != tc.code || out != "" || (tc.stderr == "") != (e == "") || !strings.Contains(e, tc.stderr) {
			t.Fatalf("row %d %q: code %d, stdout %q, stderr %q; want %d, %q", i, tc.args, code, out, e, tc.code, tc.stderr)
		}
		s, err := state.Load(d)
		if err != nil || s.LastReport == nil || !s.LastReport.Equal(testNow) || s.LastReport.Location() != time.UTC || s.ReportRow != tc.wantRow {
			t.Fatalf("row %d %q: last_report %v, report_row %d, err %v; want %v, %d", i, tc.args, s.LastReport, s.ReportRow, err, testNow, tc.wantRow)
		}
		if got := counts(); got != before {
			t.Fatalf("row %d %q: log and event rows %v, want %v", i, tc.args, got, before)
		}
		if i == 0 {
			var p packet
			out, e, code := runSource(t, &Ctx{Run: run}, "report")
			if err := json.Unmarshal([]byte(out), &p); code != exit.OK || err != nil || len(p.Log) != 0 || p.LogNext != 3 {
				t.Fatalf("report after report done: code %d, stderr %q, log %+v, log_next %d, err %v", code, e, p.Log, p.LogNext, err)
			}
		}
	}
}
