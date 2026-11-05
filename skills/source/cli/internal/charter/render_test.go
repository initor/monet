package charter

import (
	"os"
	"strings"
	"testing"
)

// The golden file is the page that the owner reads, so a layout change shows in review.
func TestRenderGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/charter.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Load("testdata/charter.json")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/charter.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := Render(c, Hash(data)); got != string(want) {
		t.Fatalf("Render of the fixture differs from testdata/charter.md\n--- got\n%s\n--- want\n%s", got, want)
	}
}

// Empty lists render as none, so a missing section never reads as a lost one.
func TestRenderEmptyLists(t *testing.T) {
	c := &Charter{Personas: []Card{{ID: "p", Weight: "blocking", Description: "d"}}}
	got := Render(c, "h")
	for _, s := range []string{
		"## Phases\n\n- none\n", "## Invariants\n\n- none\n", "## Grants\n\n- none\n",
		"- p (blocking): d\n  - sources: none\n  - phases: none\n",
	} {
		if !strings.Contains(got, s) {
			t.Errorf("no %q in\n%s", s, got)
		}
	}
	if !strings.HasSuffix(got, "\n\n<!-- charter sha256=h -->\n") {
		t.Errorf("last line of\n%s", got)
	}
}

// Pinned against the published SHA-256 of the empty input and of "abc".
func TestHash(t *testing.T) {
	for in, want := range map[string]string{
		"":    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"abc": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
	} {
		if got := Hash([]byte(in)); got != want {
			t.Errorf("Hash(%q) = %s, want %s", in, got, want)
		}
	}
}
