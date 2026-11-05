package charter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, s string) string {
	p := filepath.Join(t.TempDir(), "charter.json")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The repo check runs before the strict decode, so its text wins.
func TestLoadRefusesRepo(t *testing.T) {
	_, err := Load(write(t, `{"version": 1, "personas": [{"id": "a"}], "repo": "x"}`))
	if err == nil || err.Error() != "charter: repo comes with step 2 (call 24)" {
		t.Fatalf("err %v", err)
	}
}

func TestLoadIsStrict(t *testing.T) {
	if c, err := Load("testdata/charter.json"); err != nil || c.QualityBar == "" || len(c.Grants) != 1 ||
		len(c.Invariants) != 2 || len(c.Phases) != 2 || c.Phases[0] != "research" || c.Phases[1] != "design" ||
		len(c.Personas) != 2 || c.Personas[0].Weight != "blocking" || c.Personas[1].Weight != "advisory" {
		t.Fatalf("fixture: %+v, %v", c, err)
	}
	if _, err := Load(write(t, `{"version": 1, "extra": "x"}`)); err == nil {
		t.Fatal("unknown key decoded")
	}
}

// Each row breaks one rule of the plan's A12 table in a fresh copy of the fixture.
func TestLintFields(t *testing.T) {
	for i, row := range []struct {
		edit func(c *Charter)
		want string
	}{
		{func(c *Charter) {}, ""},
		{func(c *Charter) { c.Version = 2 }, "version: want 1"},
		{func(c *Charter) { c.Goal = " " }, "goal: empty"},
		{func(c *Charter) { c.Context = "" }, "context: empty"},
		{func(c *Charter) { c.Deliverable = "\t" }, "deliverable: empty"},
		{func(c *Charter) { c.QualityBar = "" }, "quality_bar: empty"},
		{func(c *Charter) { c.Page = "http://example.com/x" }, "page: want an https link"},
		{func(c *Charter) { c.Page = "https:/x" }, "page: want an https link"},
		{func(c *Charter) { c.Page = "see https://x" }, "page: want an https link"},
		{func(c *Charter) { c.Invariants[1].ID = "I2a" }, "invariants: I2a: want an id like I1"},
		{func(c *Charter) { c.Invariants[1].ID = "i2" }, "invariants: i2: want an id like I1"},
		{func(c *Charter) { c.Invariants[1].ID = "xI1" }, "invariants: xI1: want an id like I1"},
		{func(c *Charter) { c.Invariants[1].ID = "I" }, "invariants: I: want an id like I1"},
		{func(c *Charter) { c.Invariants[1].ID = "J1" }, "invariants: J1: want an id like I1"},
		{func(c *Charter) { c.Invariants[1].ID = "I1" }, "invariants: I1: duplicate"},
		{func(c *Charter) { c.Invariants[0].Text = " " }, "invariants: I1: empty text"},
		{func(c *Charter) { c.Phases = nil }, "phases: empty"},
		{func(c *Charter) { c.Phases = []string{} }, "phases: empty"},
		{func(c *Charter) { c.Phases[1] = "review" }, "phases: review: want research or design"},
		{func(c *Charter) { c.Phases[1] = "research" }, "phases: research: duplicate"},
		{func(c *Charter) { c.Phases = []string{"design", "research"} }, "phases: want research first"},
		{func(c *Charter) { c.Phases = append(c.Phases, "implementation") }, "phases: implementation comes with step 2 (call 24)"},
	} {
		c, err := Load("testdata/charter.json")
		if err != nil {
			t.Fatal(err)
		}
		row.edit(c)
		got := strings.Join(Lint(c), "\n")
		if got != row.want {
			t.Errorf("row %d: got %q, want %q", i, got, row.want)
		}
	}
}

// Lint keeps the order of the table when several rules break at once.
func TestLintOrder(t *testing.T) {
	c := &Charter{Version: 0, Page: "x", Invariants: []Invariant{{ID: "I1"}}, Phases: []string{"implementation"}}
	want := []string{"version: want 1", "goal: empty", "context: empty", "deliverable: empty", "quality_bar: empty",
		"page: want an https link", "invariants: I1: empty text", "phases: want research first",
		"phases: implementation comes with step 2 (call 24)"}
	if got := Lint(c); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %q, want %q", got, want)
	}
}
