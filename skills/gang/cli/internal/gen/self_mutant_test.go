package gen

import (
	"os"
	"strings"
	"testing"
)

// Each implement and fix agent tries about 5 one-line mutants on the lines it
// changed, and pins each survivor with a test, after it writes its code and
// before the gate and the review. It undoes a mutant by hand, because git
// checkout would also drop its work, which is not committed yet.
func TestImplementPromptsRunASelfMutantPass(t *testing.T) {
	md, err := os.ReadFile("../../../reference/roles.md")
	if err != nil {
		t.Fatal(err)
	}
	prompts, err := ParsePrompts(md)
	if err != nil {
		t.Fatal(err)
	}
	// code is the step text that changes the code, which the pass must follow.
	for key, code := range map[string]string{
		"implement_new": "Then write the code until they pass.",
		"implement_fix": "For the review findings: fix each blocker",
	} {
		p := prompts[key]
		at := strings.Index(p, code)
		pass := strings.Index(p, ". Self mutant pass: ")
		gate := strings.Index(p, "gate run {{id}}")
		if at < 0 || pass < 0 || gate < 0 || !(at < pass && pass < gate) {
			t.Errorf("%s: the self mutant pass is a step after the code and before the gate: code at %d, pass at %d, gate at %d", key, at, pass, gate)
			continue
		}
		for _, want := range []string{
			"Pick 5 lines of the code that",
			"one-line mutant",
			"A mutant counts as killed only when the tests build and a test fails.",
			"Never undo a mutant with git checkout, git restore, or git stash",
			"pin it with a new or a stronger test in a file that SCOPE allows",
			"name it as equivalent in your report",
			"`git status --short` must list only the files that you meant to change",
			"each self mutant with its result and the test that kills it (or why it is equivalent)",
		} {
			if !strings.Contains(p, want) {
				t.Errorf("%s lacks %q", key, want)
			}
		}
	}
}
