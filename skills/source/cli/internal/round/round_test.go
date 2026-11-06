package round

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"source/internal/charter"
)

func TestPanel(t *testing.T) {
	ch := &charter.Charter{Personas: []charter.Card{{ID: "r", Weight: "blocking", Phases: []string{"research"}},
		{ID: "rd", Weight: "blocking", Phases: []string{"research", "design"}}, {ID: "a", Weight: "advisory"}, {ID: "b", Weight: "blocking"}}}
	for _, tc := range [][3]string{{"research", "", "r rd"}, {"design", "b", "rd b"}, {"research", "a b", "r rd a b"}, {"research", "b a", "r rd a b"},
		{"research", "a r", "pick r: not a persona with no phases"}, {"design", "zz b", "pick zz: not a persona with no phases"}} {
		cards, err := Panel(ch, tc[0], strings.Fields(tc[1]))
		var ids []string
		for _, c := range cards {
			ids = append(ids, c.ID)
		}
		if got := strings.Join(ids, " "); err == nil && got != tc[2] || err != nil && (err.Error() != tc[2] || cards != nil) {
			t.Errorf("Panel(%s, %q) = %q, %v; want %s", tc[0], tc[1], got, err, tc[2])
		}
	}
}

func TestEffort(t *testing.T) {
	for blocks := 0; blocks <= 3; blocks++ {
		for _, final := range []bool{false, true} {
			if got, want := Effort(blocks, final), map[bool]string{true: "max", false: "medium"}[final || blocks == 1]; got != want {
				t.Errorf("Effort(%d, %v) = %s, want %s", blocks, final, got, want)
			}
		}
	}
}

func TestRoundSaveLoad(t *testing.T) {
	run, two := t.TempDir(), []Check{{ID: "2-r", Persona: "r", Weight: "blocking", Effort: "max", Brief: "/b",
		BriefHash: "h", Verdict: "/v", Result: "block", Item: "D1"}, {ID: "2-a", Persona: "a"}}
	for _, tc := range []struct {
		r    Round
		want string
	}{{Round{N: 2, Phase: "design", Final: true, Version: 3, Versions: []string{"/a"}, Checks: two, Outcome: "FIX"},
		`{"round":2,"phase":"design","final":true,"version":3,"versions":["/a"],"checks":[{"id":"2-r","persona":"r","weight":"blocking","effort":"max","brief":"/b","brief_sha256":"h","verdict":"/v","result":"block","item":"D1"},{"id":"2-a","persona":"a","weight":"","effort":"","brief":"","brief_sha256":"","verdict":""}],"outcome":"FIX"}`},
		{Round{N: 3}, `{"round":3,"phase":"","version":0,"versions":null,"checks":null}`}} {
		var buf bytes.Buffer
		err := Save(run, tc.r)
		raw, _ := os.ReadFile(filepath.Join(run, "rounds", strconv.Itoa(tc.r.N), "round.json"))
		got, lerr := Load(run, tc.r.N)
		if err != nil || json.Compact(&buf, raw) != nil || buf.String() != tc.want || lerr != nil || !reflect.DeepEqual(got, tc.r) {
			t.Errorf("save %v, file %s, load %+v, %v; want %s", err, raw, got, lerr, tc.want)
		}
	}
	if err := os.WriteFile(Path(run, 3, "round.json"), []byte(`{"round":3,"zz":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(run, 3); err == nil {
		t.Error("Load of a round.json with an unknown key: no error")
	}
}
