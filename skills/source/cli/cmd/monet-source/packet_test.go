package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"source/internal/artifact"
	"source/internal/exit"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

// jsonNames returns the json tags of the fields of the struct v, in field order.
func jsonNames(v any) []string {
	var names []string
	for typ, i := reflect.TypeOf(v), 0; i < typ.NumField(); i++ {
		names = append(names, typ.Field(i).Tag.Get("json"))
	}
	return names
}

// buildPacket fills the summary and the waiting items (drafts included) of an item run with a wall, keeps
// last_write nil and diagram none, gives a nil round and the kickoff phase before the first round and the seal,
// and fails with no charter. The packet types carry the JSON names of R27.
func TestPacketSummary(t *testing.T) {
	run := newItemRun(t)
	in := `{"question": "q", "default": "d", "blocks": "b"}`
	if _, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(in)}, "wall", "open", "--holder", "h1"); code != exit.OK {
		t.Fatalf("wall open: code %d, stderr %q", code, e)
	}
	got, err := buildPacket(store.Dir{Root: run})
	want := packet{Slug: "demo", Page: "https://example.com/source/run", Goal: "Explain how the run store keeps state safe.",
		Deliverable: "One explainer page.", Phase: "research", Round: &packetRound{2, "research", false, 2, "ACCEPT"},
		Counts: map[string]map[string]int{"research": {"reviewer": 2}}, Diagram: "none",
		Waiting: packetWaiting{Items: []packetItem{{"D1", "reviewer", "research", "draft", "", "", ""}}, Walls: []packetWall{{"W1", "q", "d", "b"}}}}
	logAt := testNow.Format(time.RFC3339Nano)
	for _, l := range []string{"charter sealed with words w1", "VERSION research v1", "FINDINGS 1 FIX", "VERSION research v2", "FINDINGS 2 ACCEPT", "WALL W1"} {
		want.Log = append(want.Log, packetLine{logAt, l})
	}
	want.Decisions, want.LogNext, want.Changed = []packetDecision{}, 6, logAt
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, err %v; want %+v", got, err, want)
	}

	// An open D1, an open final round 2 on v3, a page write, and a move to design show in the packet; the round and
	// the item keep their own phase.
	full := `{"default": "keep v1", "way_back": "phase back research", "affects": "design"}`
	if _, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(full)}, "item", "open", "D1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("item open: code %d, stderr %q", code, e)
	}
	r, err := round.Load(run, 2)
	if err != nil {
		t.Fatal(err)
	}
	r.Outcome, r.Final, r.Version = "", true, 3
	at := testNow.Add(time.Hour)
	if err := round.Save(run, r); err != nil {
		t.Fatal(err)
	}
	if err := state.Update(store.Dir{Root: run}, func(s *state.State) error { s.LastReport, s.Phase = &at, "design"; return nil }); err != nil {
		t.Fatal(err)
	}
	want.Phase, want.Round, want.LastWrite = "design", &packetRound{2, "research", true, 3, "open"}, &at
	want.Waiting.Items = []packetItem{{"D1", "reviewer", "research", "open", "keep v1", "phase back research", "design"}}
	want.Log, want.LogNext = append(want.Log, packetLine{logAt, "ITEM D1"}), 7
	if got, err := buildPacket(store.Dir{Root: run}); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, err %v; want %+v", got, err, want)
	}

	// The round shows from the first round on.
	if err := state.Update(store.Dir{Root: run}, func(s *state.State) error { s.Round = 1; return nil }); err != nil {
		t.Fatal(err)
	}
	if got, err := buildPacket(store.Dir{Root: run}); err != nil || got.Round == nil || got.Round.N != 1 {
		t.Fatalf("round 1 packet %+v, err %v; want round 1", got.Round, err)
	}

	kick := newRun(t, "demo")
	if _, e, code := runSource(t, &Ctx{Run: kick}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK {
		t.Fatalf("init: code %d, stderr %q", code, e)
	}
	if _, err := buildPacket(store.Dir{Root: kick}); err == nil {
		t.Fatal("no charter: got no error")
	}
	writeCharter(t, kick)
	p, err := buildPacket(store.Dir{Root: kick})
	data, _ := json.Marshal(p)
	if err != nil || p.Phase != "kickoff" || p.Round != nil || p.Changed != "" || !strings.Contains(string(data), `"round":null`) ||
		!strings.Contains(string(data), `"last_write":null`) || strings.Contains(string(data), "delivered") || strings.Contains(string(data), "verdicts") {
		t.Fatalf("kickoff packet %s, err %v; want phase kickoff, a null round and last_write, no delivered or verdicts", data, err)
	}

	for _, tc := range []struct {
		v    any
		want string
	}{
		{packet{}, "slug page goal deliverable phase round counts changed waiting decisions log log_next diagram last_write delivered,omitempty verdicts,omitempty"},
		{packetRound{}, "n phase final version outcome"},
		{packetItem{}, "id persona phase status default way_back affects"},
		{packetWall{}, "id question default blocks"},
		{packetWaiting{}, "items walls"},
		{packetDecision{}, "at by decision reason"},
		{packetLine{}, "at line"},
	} {
		if got := strings.Join(jsonNames(tc.v), " "); got != tc.want {
			t.Errorf("%T names %q; want %q", tc.v, got, tc.want)
		}
	}
}

// buildPacket gives every decisions.tsv row, the log.tsv rows from report_row on (none past the end), log_next as
// the row count, and changed as the at of the last row.
func TestPacketLog(t *testing.T) {
	run := newLoopRun(t)
	d := store.Dir{Root: run}
	submitFile(t, run, "a.md", "one\n")
	submitFile(t, run, "a.md", "two\n")
	later := &Ctx{Run: run, Now: func() time.Time { return testNow.Add(time.Hour) }}
	if _, e, code := runSource(t, later, "decision", "add", "keep one page", "--reason", "it reads faster", "--holder", "d1"); code != exit.OK {
		t.Fatalf("decision add: code %d, stderr %q", code, e)
	}
	at := testNow.Format(time.RFC3339Nano)
	all := []packetLine{{at, "charter sealed with words w1"}, {at, "VERSION research v1"}, {at, "VERSION research v2"}}
	decs := []packetDecision{{testNow.Add(time.Hour).Format(time.RFC3339Nano), "dev", "keep one page", "it reads faster"}, {at, "brain", "b", "r"}}
	if err := store.AppendTSV(d.Path("decisions.tsv"), decisionHeader, []string{at, "brain", "b", "r"}); err != nil {
		t.Fatal(err)
	}
	check := func(row, next int, changed string, log []packetLine) {
		t.Helper()
		if err := state.Update(d, func(s *state.State) error { s.ReportRow = row; return nil }); err != nil {
			t.Fatal(err)
		}
		got, err := buildPacket(d)
		if err != nil || !reflect.DeepEqual(got.Decisions, decs) || !reflect.DeepEqual(got.Log, log) || got.LogNext != next || got.Changed != changed {
			t.Errorf("report_row %d: decisions %+v, log %+v, log_next %d, changed %q, err %v; want %+v, %+v, %d, %q",
				row, got.Decisions, got.Log, got.LogNext, got.Changed, err, decs, log, next, changed)
		}
	}
	check(0, 3, at, all)
	check(2, 3, at, all[2:])
	check(9, 3, at, []packetLine{})
	last := testNow.Add(2 * time.Hour).Format(time.RFC3339Nano)
	if err := store.AppendTSV(d.Log(), logHeader, []string{last, "WALL W1"}); err != nil {
		t.Fatal(err)
	}
	check(3, 4, last, []packetLine{{last, "WALL W1"}})
}

// hasMermaid agrees with a line-prefix reference for every one-byte edit of the fence, on a line of its own,
// between two lines, and after text on its line.
func TestHasMermaidLine(t *testing.T) {
	fence, p := "```mermaid", filepath.Join(t.TempDir(), "a.md")
	for _, l := range oneByteEdits(fence) {
		for _, body := range []string{l + "\n", "x\n" + l + "\ny\n", "x " + l + "\n"} {
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			want := slices.ContainsFunc(strings.Split(body, "\n"), func(s string) bool { return strings.HasPrefix(s, fence) })
			if got, err := hasMermaid(p); err != nil || got != want {
				t.Fatalf("%q: got %v, err %v", body, got, err)
			}
		}
	}
	if err := os.WriteFile(p, []byte("x\n"+fence), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := hasMermaid(p); err != nil || !got {
		t.Errorf("fence on a last line with no newline: got %v, err %v; want true", got, err)
	}
	if err := os.WriteFile(p, []byte(strings.Repeat("a", 1<<17)+"\n"+fence+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := hasMermaid(p); err != nil || !got {
		t.Errorf("fence after a 128 KiB line: got %v, err %v; want true", got, err)
	}
	if _, err := hasMermaid(filepath.Join(t.TempDir(), "none.md")); err == nil {
		t.Error("missing file: got no error")
	}
}

// newestDiagram gives none with no version, the first diagram file in manifest order of the version with the
// newest at, and none when that version has no diagram, whatever the older versions hold. buildPacket names it.
func TestPacketDiagram(t *testing.T) {
	run, src := t.TempDir(), t.TempDir()
	file := func(name, body string) string {
		p := filepath.Join(src, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	plain, diagram := "no diagram\n", "x\n```mermaid\ngraph LR\n```\n"
	step := func(phase string, v int, at time.Time, paths []string, want string) {
		t.Helper()
		if paths != nil {
			if _, err := artifact.Freeze(filepath.Join(run, "artifacts", phase, "v"+strconv.Itoa(v)), phase, v, at, paths); err != nil {
				t.Fatal(err)
			}
		}
		if got, err := newestDiagram(run); err != nil || got != want {
			t.Errorf("after %s v%d: got %q, err %v; want %q", phase, v, got, err, want)
		}
	}
	step("research", 0, testNow, nil, "none")
	step("research", 1, testNow, []string{file("a.md", plain), file("b.md", diagram)}, filepath.Join(run, "artifacts", "research", "v1", "b.md"))
	step("design", 1, testNow.Add(time.Hour), []string{file("c.md", plain)}, "none")
	step("research", 2, testNow.Add(2*time.Hour), []string{file("e.md", diagram), file("d.md", diagram)}, filepath.Join(run, "artifacts", "research", "v2", "e.md"))

	loop := newLoopRun(t)
	submitFile(t, loop, "a.md", diagram)
	if p, err := buildPacket(store.Dir{Root: loop}); err != nil || p.Diagram != filepath.Join(loop, "artifacts", "research", "v1", "a.md") {
		t.Errorf("packet diagram %q, err %v; want research v1 a.md", p.Diagram, err)
	}
}

// After a final ACCEPT of reviewer, the packet has no delivered or verdicts; after deliver, delivered holds the
// two version folders of the final round and verdicts the persona and result of its check.
func TestPacketDelivered(t *testing.T) {
	run := newAcceptanceRun(t)
	d := store.Dir{Root: run}
	if _, e, code := runSource(t, &Ctx{Run: run}, "accept", "--holder", "h1"); code != exit.OK {
		t.Fatalf("accept: code %d, stderr %q", code, e)
	}
	writeVerdict(t, run, 3, "reviewer", "accept")
	closeRound(t, run, 3, "ACCEPT")
	p, err := buildPacket(d)
	data, _ := json.Marshal(p)
	if err != nil || strings.Contains(string(data), `"delivered"`) || strings.Contains(string(data), `"verdicts"`) {
		t.Fatalf("before deliver: %s, err %v; want no delivered or verdicts", data, err)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "deliver", "--holder", "h1"); code != exit.OK {
		t.Fatalf("deliver: code %d, stderr %q", code, e)
	}
	p, err = buildPacket(d)
	versions := []string{filepath.Join(run, "artifacts", "research", "v1"), filepath.Join(run, "artifacts", "design", "v1")}
	if err != nil || !reflect.DeepEqual(p.Delivered, versions) || !reflect.DeepEqual(p.Verdicts, []packetVerdict{{"reviewer", "accept"}}) {
		t.Fatalf("after deliver: delivered %q, verdicts %+v, err %v; want %q and [{reviewer accept}]", p.Delivered, p.Verdicts, err, versions)
	}
	if got := strings.Join(jsonNames(packetVerdict{}), " "); got != "persona result" {
		t.Errorf("packetVerdict names %q; want %q", got, "persona result")
	}
}
