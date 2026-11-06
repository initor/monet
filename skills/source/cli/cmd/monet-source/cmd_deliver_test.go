package main

import (
	"path/filepath"
	"strings"
	"testing"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/item"
	"source/internal/lock"
	"source/internal/state"
	"source/internal/store"
)

// On an acceptance run with an open item D1 and an open wall W1, deliver gives 2 before accept, while the
// final round is open, after a final FIX, and when state.json still has a round open after a final ACCEPT.
// After a final ACCEPT, another holder gives 4, then h1 gets 0:
// the deliverable folders and the waiting ids, the phase delivered, DELIVERED 4 last for both roles, and
// brain still held by h1. A second deliver gives 2. Each refusal writes nothing.
func TestDeliver(t *testing.T) {
	run := newAcceptanceRun(t)
	d := store.Dir{Root: run}
	in := `{"question": "Keep the v1 layout?", "default": "keep it", "blocks": "design"}`
	if out, e, code := runSource(t, &Ctx{Run: run, Stdin: strings.NewReader(in)}, "wall", "open", "--holder", "h1"); code != exit.OK || out != "W1\n" {
		t.Fatalf("wall open: code %d, stdout %q, stderr %q", code, out, e)
	}
	if err := item.Save(d, "D1", item.Item{ID: "D1", Phase: "design", Persona: "reviewer", Round: 2, Status: "open"}); err != nil {
		t.Fatal(err)
	}
	refused := func(when, holder string, code int, msg string) {
		t.Helper()
		files := readOr(t, d.State()) + readOr(t, d.Events()) + readOr(t, d.Log())
		out, e, got := runSource(t, &Ctx{Run: run}, "deliver", "--holder", holder)
		if got != code || out != "" || e != "monet-source: "+msg+"\n" || readOr(t, d.State())+readOr(t, d.Events())+readOr(t, d.Log()) != files {
			t.Errorf("%s: code %d, stdout %q, stderr %q; want %d, %q, and no write", when, got, out, e, code, msg)
		}
	}
	accept := func(n int) {
		t.Helper()
		if _, e, code := runSource(t, &Ctx{Run: run}, "accept", "--holder", "h1"); code != exit.OK {
			t.Fatalf("accept %d: code %d, stderr %q", n, code, e)
		}
	}
	none := "deliver: no accepted final check"
	refused("before accept", "h1", exit.Refused, none)
	accept(3)
	refused("final round open", "h1", exit.Refused, none)
	closeRound(t, run, 3, "FIX") // no verdict file, so FIX
	refused("after a final FIX", "h1", exit.Refused, none)
	accept(4)
	writeVerdict(t, run, 4, "reviewer", "accept")
	closeRound(t, run, 4, "ACCEPT")
	setOpen := func(open bool) {
		t.Helper()
		if err := state.Update(d, func(s *state.State) error { s.RoundOpen = open; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	setOpen(true) // round.json says ACCEPT, but state.json still has the round open
	refused("open after a final ACCEPT", "h1", exit.Refused, none)
	setOpen(false)
	refused("another holder", "h2", exit.Held, "lock held: brain is not held by h2")
	out, e, code := runSource(t, &Ctx{Run: run}, "deliver", "--holder", "h1")
	want := "deliverable " + filepath.Join(run, "artifacts", "research", "v1") + "\ndeliverable " + filepath.Join(run, "artifacts", "design", "v1") + "\nwaiting D1\nwaiting W1\n"
	if code != exit.OK || out != want || e != "" {
		t.Fatalf("deliver: code %d, stdout %q, stderr %q; want 0 and %q", code, out, e, want)
	}
	if s, err := state.Load(d); err != nil || s.Phase != phaseDelivered {
		t.Errorf("phase %q (err %v); want delivered", s.Phase, err)
	}
	for _, role := range []string{"brain", "dev"} {
		if lines, _, err := events.For(d, role, 0); err != nil || len(lines) == 0 || lines[len(lines)-1] != "DELIVERED 4" {
			t.Errorf("%s events %q (err %v); want DELIVERED 4 last", role, lines, err)
		}
	}
	if !strings.HasSuffix(readOr(t, d.Log()), "\tDELIVERED 4\n") {
		t.Errorf("log.tsv %q; want DELIVERED 4 last", readOr(t, d.Log()))
	}
	if l, ok, err := lock.Read(d.Locks(), "brain"); err != nil || !ok || l.Holder != "h1" {
		t.Errorf("brain lease %+v, held %v (err %v); want h1", l, ok, err)
	}
	refused("a second deliver", "h1", exit.Refused, none)
}
