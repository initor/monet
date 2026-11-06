package state

import (
	"cmp"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/store"
)

func TestUpdateKeepsConcurrentWrites(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	os.WriteFile(d.State(), []byte(`{"version":1,"slug":"demo"}`), 0o644)
	errs := make(chan error, 20)
	for range 20 {
		go func() { errs <- Update(d, func(s *State) error { s.Phase += "x"; return nil }) }()
	}
	var err error
	for err == nil && len(errs) < 20 { // each write is atomic, so no Load beside the updates fails
		_, err = Load(d)
	}
	for range 20 {
		err = cmp.Or(err, <-errs)
	}
	if s, lerr := Load(d); err != nil || lerr != nil || len(s.Phase) != 20 {
		t.Fatalf("phase %q, err %v, then %v: a Load or an update failed, or an update was lost", s.Phase, err, lerr)
	}
}

func TestUpdateWritesOnlyOnSuccess(t *testing.T) {
	for _, tc := range [][3]string{ // state.json before ("-" is none, and none after; "/" is a directory), the error's tail ("^" is its start), state.json after
		{"-", "^no run state: run init first", ""}, {"/", "^read <run>/state.json: is a directory", ""},
		{`{"slug":5}`, "State.slug of type string", `{"slug":5}`},
		{`{"version":1,"slug":"demo","created":"0000-01-01T00:00:00Z"}`, "^<nil>", "{\n  \"version\": 1,\n  \"slug\": \"demo\",\n  \"created\": \"0001-01-01T00:00:00Z\",\n  \"phase\": \"\"\n}\n"},
		{`{"slug":"other"}`, `^fn: slug "other", mutex <nil>`, `{"slug":"other"}`},
		{`{"slug":"demo","extra":1}`, `^<run>/state.json: json: unknown field "extra"`, `{"slug":"demo","extra":1}`},
		{`{"slug":"demo"}{}`, "^<run>/state.json: trailing data after the JSON value", `{"slug":"demo"}{}`},
		{`{"version":1.5}`, "cannot unmarshal number 1.5 into Go struct field State.version of type int", `{"version":1.5}`},
		{"", "^<run>/state.json: EOF", ""},
		{`{"slug":`, "^<run>/state.json: unexpected EOF", `{"slug":`},
		{`{"slug":"demo","created":"9999-01-01T00:00:00Z"}`, "year outside of range [0,9999]", `{"slug":"demo","created":"9999-01-01T00:00:00Z"}`},
	} {
		d := store.Dir{Root: t.TempDir()}
		if tc[0] != "-" && (tc[0] != "/" || os.Mkdir(d.State(), 0o755) != nil) {
			os.WriteFile(d.State(), []byte(tc[0]), 0o644)
		}
		err := Update(d, func(s *State) error {
			s.Created = s.Created.AddDate(1, 0, 0) // a change to write, and from 9999 a write that fails
			if _, err := os.Stat(d.Path("locks", "state.mutex")); err != nil || s.Slug != "demo" {
				return fmt.Errorf("fn: slug %q, mutex %v", s.Slug, err)
			}
			return nil
		})
		if got, rerr := os.ReadFile(d.State()); !strings.HasSuffix("^"+strings.ReplaceAll(fmt.Sprint(err), d.Root, "<run>"), tc[1]) || string(got) != tc[2] || os.IsNotExist(rerr) != (tc[0] == "-") {
			t.Errorf("before %q: err %v, want %s; after %q (%v), want %q", tc[0], err, tc[1], got, rerr, tc[2])
		}
	}
}

func TestLoadKeepsPermissionError(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	if os.WriteFile(d.State(), []byte(`{"slug":"demo"}`), 0o000); os.Geteuid() == 0 {
		t.Skip("root can read a mode 000 file")
	}
	if _, err := Load(d); !os.IsPermission(err) {
		t.Fatalf("err %v, want the permission error of the read: only a missing file is no run state", err)
	}
}

var loopNames = []string{"dev", "last_beat", "versions", "checked", "accepted", "round", "round_open", "counts", "picks", "items", "walls", "moves"} // typed out, so a renamed tag fails

func TestStateLoopFieldsRoundTrip(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	at := time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
	in := State{Version: 1, Slug: "demo", Dev: "d1", LastBeat: &at, Versions: map[string]int{"research": 2},
		Checked: map[string]int{"research": 2}, Accepted: map[string]int{"research": 1}, Round: 3, RoundOpen: true,
		Counts: map[string]map[string]int{"research": {"reviewer": 1}}, Picks: map[string][]string{"design": {"x"}},
		Items: 1, Walls: 2, Moves: 4}
	err := store.WriteJSON(d.State(), in)
	if got, lerr := Load(d); err != nil || lerr != nil || !reflect.DeepEqual(got, in) {
		t.Fatalf("got %+v, write err %v, load err %v", got, err, lerr)
	}
	data, _ := os.ReadFile(d.State())
	for _, name := range loopNames {
		if !strings.Contains(string(data), `"`+name+`":`) {
			t.Errorf("state.json %s has no field %q", data, name)
		}
	}
}

func TestStateLoopFieldsOmitted(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	err := store.WriteJSON(d.State(), State{Version: 1, Slug: "demo"})
	data, rerr := os.ReadFile(d.State())
	for _, name := range loopNames {
		if err != nil || rerr != nil || strings.Contains(string(data), `"`+name+`"`) {
			t.Errorf("state.json %s holds %q (err %v, %v): each loop field needs omitempty", data, name, err, rerr)
		}
	}
}
