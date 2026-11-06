package round

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// fullBrief is a BriefInput with every field set.
func fullBrief() BriefInput {
	return BriefInput{Check: "2-reviewer", Persona: "reviewer", Effort: "max", Charter: "# Charter\nShip the report.\n",
		PersonaBrief: "You review tables.", Verdict: "/run/rounds/2/verdicts/2-reviewer.json", Response: "F1 fixed: both rows added.\n",
		Sources: []string{"/src/a.md", "/src/b.md"}, Versions: []string{"/run/versions/draft-2"},
		Blockers: []Finding{{ID: "F1", Severity: "blocker", Problem: "Two rows drop.", Evidence: "report.md:12", Ask: "Add them."}}}
}

func TestBriefHoldsTheAllowlist(t *testing.T) {
	want := "# Check 2-reviewer by reviewer at max effort\n\n## Charter\n\n# Charter\nShip the report.\n\n" +
		"## Persona\n\nYou review tables.\n\n## Sources\n\nRead these files. Do not change them.\n\n- /src/a.md\n- /src/b.md\n\n" +
		"## Version\n\n- /run/versions/draft-2\n\n## Verdict\n\nWrite your verdict as JSON to /run/rounds/2/verdicts/2-reviewer.json, in this form:\n\n" +
		"```json\n" + VerdictSchema + "\n```\n\nSet check and persona to the values of this check. verdict is accept or block, and " +
		"severity is blocker, major, or minor. A block needs a blocker finding, and an accept holds none.\n\n" +
		"## Your blocker findings\n\n### F1 (blocker)\nProblem: Two rows drop.\nEvidence: report.md:12\nAsk: Add them.\n\n" +
		"## The answer of the developer\n\nF1 fixed: both rows added.\n"
	if got := Brief(fullBrief()); got != want {
		t.Fatalf("Brief(full)\n--- got\n%s\n--- want\n%s", got, want)
	}
	first := fullBrief()
	first.Blockers, first.Response = nil, ""
	got := Brief(first)
	if strings.Contains(got, "## Your blocker findings") || strings.Contains(got, "## The answer of the developer") || !strings.HasPrefix(want, got) {
		t.Fatalf("Brief(no blockers, no response) = %q; want the full brief up to its Verdict section", got)
	}
	answer := fullBrief()
	answer.Blockers = nil
	if got := Brief(answer); !strings.Contains(got, "## Your blocker findings\n") || !strings.HasSuffix(got, "## The answer of the developer\n\nF1 fixed: both rows added.\n") {
		t.Fatalf("Brief(response only) = %q; want both second check sections", got)
	}
}

func TestBriefOnlyItsInputs(t *testing.T) {
	marked := fullBrief()
	marked.Check, marked.Persona, marked.Effort, marked.Charter = "M-check", "M-persona", "M-effort", "M-charter"
	marked.PersonaBrief, marked.Verdict, marked.Response = "M-brief", "M-verdict", "M-response"
	got := Brief(marked)
	for _, m := range []string{"M-check", "M-persona", "M-effort", "M-charter", "M-brief", "M-verdict", "M-response"} {
		if !strings.Contains(got, m) {
			t.Fatalf("Brief holds no %s:\n%s", m, got)
		}
		got = strings.ReplaceAll(got, m, "")
	}
	empty := BriefInput{Sources: marked.Sources, Versions: marked.Versions, Blockers: marked.Blockers}
	if want := Brief(empty); got != want {
		t.Fatalf("Brief without its markers\n--- got\n%s\n--- want (empty strings)\n%s", got, want)
	}
}

func TestVerdictSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "verdict.json")
	if err := os.WriteFile(path, []byte(VerdictSchema), 0o644); err != nil {
		t.Fatal(err)
	}
	if v, err := ParseVerdict(path, "<check>", "<persona>"); err != nil || len(v.Findings) != 1 {
		t.Fatalf("ParseVerdict(VerdictSchema) = %+v, %v; want a valid verdict with one finding", v, err)
	}
	keys := func(b []byte) (top, finding []string) {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return slices.Sorted(maps.Keys(m)), slices.Sorted(maps.Keys(m["findings"].([]any)[0].(map[string]any)))
	}
	ref, err := json.Marshal(Verdict{Findings: []Finding{{}}})
	if err != nil {
		t.Fatal(err)
	}
	gotTop, gotFinding := keys([]byte(VerdictSchema))
	wantTop, wantFinding := keys(ref)
	if !reflect.DeepEqual(gotTop, wantTop) || !reflect.DeepEqual(gotFinding, wantFinding) {
		t.Fatalf("VerdictSchema keys %v %v; want the Verdict keys %v %v", gotTop, gotFinding, wantTop, wantFinding)
	}
}
