package charter

import (
	"os"
	"path/filepath"
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
