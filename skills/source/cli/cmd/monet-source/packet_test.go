package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

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
	if err != nil || p.Phase != "kickoff" || p.Round != nil || !strings.Contains(string(data), `"round":null`) ||
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
	} {
		if got := strings.Join(jsonNames(tc.v), " "); got != tc.want {
			t.Errorf("%T names %q; want %q", tc.v, got, tc.want)
		}
	}
}
