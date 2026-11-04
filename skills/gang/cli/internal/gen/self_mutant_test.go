package gen

import (
	"os"
	"strings"
	"testing"
)

// Each implement and fix agent tries about 5 one-line mutants on the lines it
// changed, and pins each survivor with a test, before the gate and the review.
// It undoes a mutant by hand, because git checkout would also drop its work,
// which is not committed yet.
func TestImplementPromptsRunASelfMutantPass(t *testing.T) {
	md, err := os.ReadFile("../../../reference/roles.md")
	if err != nil {
		t.Fatal(err)
	}
	prompts, err := ParsePrompts(md)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"implement_new", "implement_fix"} {
		p := prompts[key]
		pass, gate := strings.Index(p, ". Self mutant pass: "), strings.Index(p, "gate run {{id}}")
		if pass < 0 || gate < 0 || pass > gate {
			t.Errorf("%s: the self mutant pass is a step before the gate: pass at %d, gate at %d", key, pass, gate)
			continue
		}
		for _, want := range []string{
			"Pick 5 lines of the code that",
			"one-line mutant",
			"Never undo a mutant with git checkout, git restore, or git stash",
			"pin it with a new or a stronger test",
			"each self mutant with its result and the test that kills it",
		} {
			if !strings.Contains(p, want) {
				t.Errorf("%s lacks %q", key, want)
			}
		}
	}
}
