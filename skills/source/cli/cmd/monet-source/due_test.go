package main

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"source/internal/state"
)

// dueEvents gives REPORT_DUE from the last page write or else the seal, then DEV_SILENT from the last beat, each
// once 60 minutes have passed. REPORT_DUE needs the seal, DEV_SILENT needs a beat, and neither comes after delivery.
// Each time is RFC 3339 UTC nanoseconds.
func TestDueEvents(t *testing.T) {
	at := func(dt time.Duration) *time.Time { u := testNow.Add(dt); return &u }
	m, seal := time.Minute, &state.Seal{Words: "w1", At: testNow}
	due, due30, silent := "REPORT_DUE 2026-11-01T12:00:00Z", "REPORT_DUE 2026-11-01T12:30:00Z", "DEV_SILENT d1 2026-11-01T12:10:00Z"
	// far is past the time.Duration range (about 292 years), so no clock skew limit of any Duration reaches it.
	far := testNow.AddDate(300, 0, 0)
	sealed := state.State{Phase: "research", Sealed: seal}
	reported := state.State{Phase: "research", Sealed: seal, LastReport: at(30 * m)}
	beat := state.State{Phase: "research", Sealed: seal, Dev: "d1", LastBeat: at(10 * m)}
	// delivered sets every field of state.State, so the delivered guard cannot depend on one of them.
	delivered := state.State{Version: 1, Slug: "s", Created: testNow, Phase: phaseDelivered, Sealed: seal, Amends: []state.Seal{*seal},
		Briefs: map[string]string{"r": "h"}, BriefCards: map[string]string{"r": "h"}, Dev: "d1", LastBeat: at(10 * m),
		Versions: map[string]int{"design": 2}, Checked: map[string]int{"design": 1}, Accepted: map[string]int{"design": 1}, Round: 3,
		RoundOpen: true, Counts: map[string]map[string]int{"design": {"r": 1}}, Picks: map[string][]string{"design": {"r"}}, Items: 1,
		Walls: 1, Moves: 3, LastReport: at(30 * m), ReportRow: 1}
	for i, v := 0, reflect.ValueOf(delivered); i < v.NumField(); i++ {
		if v.Field(i).IsZero() {
			t.Fatalf("delivered leaves %s zero", v.Type().Field(i).Name)
		}
	}
	full := delivered
	full.Phase = "design"
	unwritten := full
	unwritten.LastReport, unwritten.ReportRow = nil, 0
	pst := time.FixedZone("PST", -8*3600)
	beatPST := testNow.Add(7).In(pst)
	nanos := state.State{Phase: "design", Sealed: &state.Seal{At: testNow.Add(5).In(pst)}, Dev: "d2", LastBeat: &beatPST}
	early := 59*m + 59*time.Second
	for _, tc := range []struct {
		name string
		s    state.State
		now  time.Duration
		want []string
	}{
		{"no seal", state.State{}, early, nil},
		{"no seal", state.State{}, 60 * m, nil},
		{"no seal", state.State{}, 70 * m, nil},
		{"no seal", state.State{}, 90 * m, nil},
		{"no seal, written and beat", state.State{Phase: "charter", LastReport: at(30 * m), Dev: "d1", LastBeat: at(10 * m)}, 90 * m, []string{silent}},
		{"kickoff beat", state.State{Dev: "d1", LastBeat: at(10*m + 7)}, 70*m + 6, nil},
		{"kickoff beat", state.State{Dev: "d1", LastBeat: at(10*m + 7)}, 70*m + 7, []string{"DEV_SILENT d1 2026-11-01T12:10:00.000000007Z"}},
		{"sealed", sealed, early, nil},
		{"sealed", sealed, 60*m - 1, nil},
		{"sealed", sealed, 60 * m, []string{due}},
		{"sealed", sealed, 70 * m, []string{due}},
		{"sealed", sealed, 90 * m, []string{due}},
		{"reported", reported, early, nil},
		{"reported", reported, 60 * m, nil},
		{"reported", reported, 70 * m, nil},
		{"reported", reported, 90 * m, []string{due30}},
		{"written before the seal", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: testNow.Add(30 * m)}, LastReport: at(3)}, 60*m + 3, []string{"REPORT_DUE 2026-11-01T12:00:00.000000003Z"}},
		{"amended", state.State{Phase: "research", Sealed: seal, Amends: []state.Seal{{Words: "w2", At: testNow.Add(30 * m)}}}, 60 * m, []string{due}},
		{"beat", beat, early, nil},
		{"beat", beat, 60 * m, []string{due}},
		{"beat", beat, 69*m + 59*time.Second, []string{due}},
		{"beat", beat, 70*m - 1, []string{due}},
		{"beat", beat, 70 * m, []string{due, silent}},
		{"beat", beat, 90 * m, []string{due, silent}},
		{"beat before the other clocks", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: testNow.Add(20 * m)}, Amends: []state.Seal{{Words: "w2", At: testNow.Add(25 * m)}}, LastReport: at(30 * m), Dev: "d1", LastBeat: at(10 * m)}, 70 * m, []string{silent}},
		{"delivered", delivered, early, nil},
		{"delivered", delivered, 60 * m, nil},
		{"delivered", delivered, 70 * m, nil},
		{"delivered", delivered, 90 * m, nil},
		{"delivered, sealed only", state.State{Phase: phaseDelivered, Sealed: seal}, 90 * m, nil},
		{"every field, not delivered", full, 90 * m, []string{due30, silent}},
		{"every field, not delivered", full, 70*m - 1, nil},
		{"every field, not written", unwritten, 60*m - 1, nil},
		{"written before the seal and the amend", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: testNow.Add(30 * m)}, Amends: []state.Seal{{Words: "w2", At: testNow.Add(40 * m)}}, LastReport: at(10 * m), Dev: "d1", LastBeat: at(20 * m)}, 80 * m, []string{"REPORT_DUE 2026-11-01T12:10:00Z", "DEV_SILENT d1 2026-11-01T12:20:00Z"}},
		{"written at the seal", state.State{Phase: "research", Sealed: seal, LastReport: at(0), Dev: "d1", LastBeat: at(10 * m)}, 70 * m, []string{due, silent}},
		{"amend, write and beat at once", state.State{Phase: "research", Sealed: seal, Amends: []state.Seal{{Words: "w2", At: testNow.Add(30 * m)}}, LastReport: at(30 * m), Dev: "d1", LastBeat: at(30 * m)}, 90 * m, []string{due30, "DEV_SILENT d1 2026-11-01T12:30:00Z"}},
		{"beat after the write", state.State{Phase: "research", Sealed: seal, LastReport: at(10 * m), Dev: "d1", LastBeat: at(30 * m)}, 70 * m, []string{"REPORT_DUE 2026-11-01T12:10:00Z"}},
		{"beat after the write", state.State{Phase: "research", Sealed: seal, LastReport: at(10 * m), Dev: "d1", LastBeat: at(30 * m)}, 90 * m, []string{"REPORT_DUE 2026-11-01T12:10:00Z", "DEV_SILENT d1 2026-11-01T12:30:00Z"}},
		{"beat at the write", state.State{Phase: "research", Sealed: seal, LastReport: at(30 * m), Dev: "d1", LastBeat: at(30 * m)}, 90 * m, []string{due30, "DEV_SILENT d1 2026-11-01T12:30:00Z"}},
		{"acceptance", state.State{Phase: phaseAcceptance, Sealed: seal, Dev: "d1", LastBeat: at(10 * m), LastReport: at(30 * m)}, 90 * m, []string{due30, silent}},
		{"acceptance", state.State{Phase: phaseAcceptance, Sealed: seal, Dev: "d1", LastBeat: at(10 * m), LastReport: at(30 * m)}, 70*m - 1, nil},
		{"clock behind the clocks", state.State{Phase: "research", Sealed: seal, LastReport: at(90 * m), Dev: "d1", LastBeat: at(90 * m)}, 30 * m, nil},
		{"clock behind the write", state.State{Phase: "research", Sealed: seal, LastReport: &far, Dev: "d1", LastBeat: at(0)}, 80 * m, []string{"DEV_SILENT d1 2026-11-01T12:00:00Z"}},
		{"write at now", state.State{Phase: "research", Sealed: seal, LastReport: at(80 * m)}, 80 * m, nil},
		{"clock behind the seal", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: far}, LastReport: at(0), Dev: "d1", LastBeat: &far}, 80 * m, []string{due}},
		{"clock just behind the write", state.State{Phase: "research", Sealed: seal, LastReport: at(80*m + 1), Dev: "d1", LastBeat: at(0)}, 80 * m, []string{"DEV_SILENT d1 2026-11-01T12:00:00Z"}},
		{"clock just behind the seal", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: testNow.Add(80*m + 1)}, LastReport: at(0), Dev: "d1", LastBeat: at(80*m + 1)}, 80 * m, []string{due}},
		{"written just before the seal", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: testNow.Add(1)}, LastReport: at(0)}, 60 * m, []string{due}},
		{"written just after the seal", state.State{Phase: "research", Sealed: seal, LastReport: at(1)}, 60 * m, nil},
		{"silent past the duration range", state.State{Phase: "research", Sealed: seal, Dev: "d1", LastBeat: at(0)}, time.Duration(1<<63 - 1), []string{due, "DEV_SILENT d1 2026-11-01T12:00:00Z"}},
		{"acceptance, not written", state.State{Phase: phaseAcceptance, Sealed: seal, Dev: "d1", LastBeat: at(10 * m)}, 90 * m, []string{due, silent}},
		{"written just before the seal, long silent", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: testNow.Add(1)}, LastReport: at(0)}, time.Duration(1<<63 - 1), []string{due}},
		{"sealed just before now", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: testNow.Add(80*m - 1)}, LastReport: at(0), Dev: "d1", LastBeat: at(80*m - 1)}, 80 * m, []string{due}},
		{"written just before now", state.State{Phase: "research", Sealed: seal, LastReport: at(80*m - 1), Dev: "d1", LastBeat: at(0)}, 80 * m, []string{"DEV_SILENT d1 2026-11-01T12:00:00Z"}},
		{"sealed at now", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: testNow.Add(80 * m)}, LastReport: at(0)}, 80 * m, []string{due}},
		{"written long after the seal", state.State{Phase: "research", Sealed: &state.Seal{Words: "w1", At: testNow.AddDate(-300, 0, 0)}, LastReport: at(0)}, 60 * m, []string{due}},
		{"written just after the seal, due", state.State{Phase: "research", Sealed: seal, LastReport: at(1)}, 60*m + 1, []string{"REPORT_DUE 2026-11-01T12:00:00.000000001Z"}},
		{"PST nanoseconds", nanos, 2 * time.Hour, []string{"REPORT_DUE 2026-11-01T12:00:00.000000005Z", "DEV_SILENT d2 2026-11-01T12:00:00.000000007Z"}},
	} {
		if got := dueEvents(tc.s, testNow.Add(tc.now)); !slices.Equal(got, tc.want) {
			t.Errorf("%s at +%v: got %q, want %q", tc.name, tc.now, got, tc.want)
		}
	}
}
