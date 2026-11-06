package round

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// oneByteEdits returns s, each deletion of one byte, and each substitution or
// insertion of one byte from 0x20 to 0x7e.
func oneByteEdits(s string) []string {
	out := []string{s}
	for i := range len(s) + 1 {
		out = append(out, s[:i]+s[min(i+1, len(s)):])
		for b := byte(0x20); b < 0x7f; b++ {
			out = append(out, s[:i]+string(b)+s[i:], s[:i]+string(b)+s[min(i+1, len(s)):])
		}
	}
	return out
}

// writeVerdict writes v as JSON to p and returns p.
func writeVerdict(t *testing.T, p string, v any) string {
	data, err := json.Marshal(v)
	if err = cmp.Or(err, os.WriteFile(p, data, 0o644)); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseVerdictValueSets(t *testing.T) {
	p := filepath.Join(t.TempDir(), "verdict.json")
	ok := func(v string, f ...Finding) bool {
		_, err := ParseVerdict(writeVerdict(t, p, Verdict{Persona: "p", Check: "1-p", Verdict: v, Findings: f}), "1-p", "p")
		return err == nil
	}
	blocker, major, valid := Finding{ID: "F0", Severity: "blocker"}, Finding{ID: "F1", Severity: "major"}, map[string]bool{"blocker": true, "major": true, "minor": true}
	far := []string{"", "info", "critical", "high", "pass", "reject"}
	for _, v := range slices.Concat(far, oneByteEdits("accept"), oneByteEdits("block"), oneByteEdits("blocker"), oneByteEdits("major"), oneByteEdits("minor")) {
		if ok(v, blocker) != (v == "block") || ok(v, major) != (v == "accept") || ok("block", blocker, Finding{ID: "F1", Severity: v}) != valid[v] {
			t.Fatalf("%q: read wrong as a verdict or a severity", v)
		}
	}
	if !reflect.DeepEqual(severities, valid) || !reflect.DeepEqual(verdicts, map[string]bool{"accept": true, "block": true}) {
		t.Errorf("value sets: %v, %v", verdicts, severities)
	}
}

// TestParseVerdictLists reads every findings list of length 0 to 3, each finding
// a blocker, a major, a minor, a bad severity, a minor with no id, or an empty
// finding, as an accept and as a block, against the rules of R17 in order. It
// reads each list alone and after 100 valid minor findings, so that the deciding
// finding also comes last in a long list.
func TestParseVerdictLists(t *testing.T) {
	p, sevs, valid := filepath.Join(t.TempDir(), "verdict.json"), []string{"blocker", "major", "minor", "high", "minor", ""}, map[string]bool{"blocker": true, "major": true, "minor": true}
	for _, pre := range []int{0, 100} {
		for n, size := 0, 1; n <= 3; n, size = n+1, size*6 {
			for code := range size {
				var fs []Finding
				for i := range pre {
					fs = append(fs, Finding{ID: fmt.Sprintf("F%d", i+1), Severity: "minor"})
				}
				bad, blocker := "", false
				for i, c := pre, code; i < pre+n; i, c = i+1, c/6 {
					f := Finding{Severity: sevs[c%6]}
					if c%6 < 4 {
						f.ID = fmt.Sprintf("F%d", i+1)
					}
					if fs, blocker = append(fs, f), blocker || f.Severity == "blocker"; bad == "" && f.ID == "" {
						bad = fmt.Sprintf("finding %d: no id", i+1)
					} else if bad == "" && !valid[f.Severity] {
						bad = "finding " + f.ID + ": severity: want blocker, major, or minor"
					}
				}
				for _, v := range []string{"accept", "block"} {
					rule := ""
					if v == "accept" && blocker {
						rule = "accept: holds a blocker finding"
					} else if v == "block" && !blocker {
						rule = "block: no blocker finding"
					}
					in, want, wantV := Verdict{Persona: "p", Check: "1-p", Verdict: v, Findings: fs}, cmp.Or(bad, rule, "<nil>"), Verdict{}
					if want == "<nil>" {
						wantV = in
					}
					if got, err := ParseVerdict(writeVerdict(t, p, in), "1-p", "p"); fmt.Sprint(err) != want || !reflect.DeepEqual(got, wantV) {
						t.Errorf("%s %+v after %d minors: got %v; want %s", v, fs[pre:], pre, err, want)
					}
				}
			}
		}
	}
	for _, id := range oneByteEdits("F1") {
		in := Verdict{Persona: "p", Check: "1-p", Verdict: "block", Findings: []Finding{{ID: id, Severity: "blocker"}}}
		if got, err := ParseVerdict(writeVerdict(t, p, in), "1-p", "p"); err != nil || !reflect.DeepEqual(got, in) {
			t.Errorf("id %q: got %+v, %v", id, got, err)
		}
	}
}

func TestParseVerdictRules(t *testing.T) {
	p, pre := filepath.Join(t.TempDir(), "verdict.json"), `{"persona":"p","check":"1-p",`
	_, rerr := os.ReadFile(p)
	if _, err := ParseVerdict(p, "1-p", "p"); !reflect.DeepEqual(err, rerr) {
		t.Errorf("missing file: err %#v, want %#v", err, rerr)
	}
	minor := Verdict{Persona: "p", Check: "1-p", Verdict: "accept", Findings: []Finding{{ID: "F1", Severity: "minor", Problem: "pr", Evidence: "ev", Ask: "as"}}}
	for _, tc := range [][2]string{
		{`"verdict":"accept","findings":[{"id":"F1","severity":"minor","problem":"pr","evidence":"ev","ask":"as"}]}`, "<nil>"},
		{`"verdict":"block","findings":[{"id":"F1","severity":"major"}]}`, "block: no blocker finding"},
		{`"verdict":"accept","findings":[{"id":"F1","severity":"minor"},{"id":"F2","severity":"blocker"}]}`, "accept: holds a blocker finding"},
		{`"verdict":"block","findings":[{"id":"F1","severity":"blocker"},{"severity":"minor"}]}`, "finding 2: no id"},
		{`"verdict":"accept","zz":1}`, `<path>: json: unknown field "zz"`},
		{`"verdict":"pass"}`, "verdict: want accept or block"},
		{`"verdict":"block","findings":[{"id":"F1","severity":"high"}]}`, "finding F1: severity: want blocker, major, or minor"},
		{`"findings":[]}`, "verdict: want accept or block"},
		{`"verdict":"accept","findings":[{"id":"F1"}]}`, "finding F1: severity: want blocker, major, or minor"},
		{`"verdict":"block","findings":[{"severity":"blocker"}]}`, "finding 1: no id"},
		{`"verdict":"block","findings":[{"id":"F1","severity":"blocker"},{"severity":"high"},{"id":"F3","severity":"minor"}]}`, "finding 2: no id"},
		{`"verdict":"block","findings":[{"id":"F1","severity":"blocker"},{"id":"F2","severity":"high"},{"id":"F3","severity":"minor"}]}`, "finding F2: severity: want blocker, major, or minor"},
		{`"verdict":"block","findings":[]}`, "block: no blocker finding"},
		{`"verdict":"block"}`, "block: no blocker finding"},
	} {
		got, err := ParseVerdict(writeVerdict(t, p, json.RawMessage(pre+tc[0])), "1-p", "p")
		if want := strings.ReplaceAll(tc[1], "<path>", p); fmt.Sprint(err) != want || !reflect.DeepEqual(got, map[bool]Verdict{true: minor}[err == nil]) {
			t.Errorf("%s: got %+v, %v; want %s", tc[0], got, err, want)
		}
	}
	for doc, want := range map[string]string{"": p + ": EOF", "null": "check : want 1-p", `{"check":"1-p","verdict":"accept"}`: "persona : want p",
		`{"persona":"","check":"1-p","verdict":"block","findings":[{"id":"F1","severity":"blocker"}]}`: "persona : want p"} {
		if err := os.WriteFile(p, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := ParseVerdict(p, "1-p", "p"); fmt.Sprint(err) != want || !reflect.DeepEqual(got, Verdict{}) {
			t.Errorf("document %q: got %+v, %v; want %s", doc, got, err, want)
		}
	}
	if b, _ := json.Marshal(Verdict{Findings: []Finding{{}}}); string(b) != `{"persona":"","check":"","verdict":"","findings":[{"id":"","severity":"","problem":"","evidence":"","ask":""}]}` {
		t.Errorf("JSON keys: %s", b)
	}
	if b, _ := json.Marshal(Verdict{}); string(b) != `{"persona":"","check":"","verdict":"","findings":null}` {
		t.Errorf("JSON keys of a zero Verdict: %s", b)
	}
}

func TestParseVerdictRefusesReuse(t *testing.T) {
	for _, v := range []string{"accept", "block"} {
		in := Verdict{Persona: "reviewer", Check: "1-reviewer", Verdict: v, Findings: map[string][]Finding{"block": {{ID: "F1", Severity: "blocker"}}}[v]}
		p, q := writeVerdict(t, filepath.Join(t.TempDir(), "verdict.json"), in), filepath.Join(t.TempDir(), "file.json")
		for _, tc := range [][3]string{{"1-reviewer", "reviewer", "<nil>"}, {"2-reviewer", "reviewer", "check 1-reviewer: want 2-reviewer"},
			{"1-reviewer", "reader", "persona reviewer: want reader"}} {
			if got, err := ParseVerdict(p, tc[0], tc[1]); fmt.Sprint(err) != tc[2] || err != nil && !reflect.DeepEqual(got, Verdict{}) {
				t.Errorf("%s verdict for check %s by %s: %v, want %s", v, tc[0], tc[1], err, tc[2])
			}
		}
		for _, e := range oneByteEdits("1-reviewer") {
			if _, err := ParseVerdict(p, e, "reviewer"); (err == nil) != (e == "1-reviewer") {
				t.Errorf("%s verdict for check %q: %v", v, e, err)
			}
		}
		for _, e := range oneByteEdits("reviewer") {
			if _, err := ParseVerdict(p, "1-reviewer", e); (err == nil) != (e == "reviewer") {
				t.Errorf("%s verdict by %q: %v", v, e, err)
			}
		}
		// The file side: the file holds an empty value, each edit of the check or the persona, or the other id.
		for _, side := range []struct{ kind, id string }{{"check", "1-reviewer"}, {"persona", "reviewer"}} {
			for _, e := range append(oneByteEdits(side.id), "", "reviewer", "1-reviewer") {
				f, want, wantV := in, side.kind+" "+e+": want "+side.id, Verdict{}
				if side.kind == "check" {
					f.Check = e
				} else {
					f.Persona = e
				}
				if e == side.id {
					want, wantV = "<nil>", in
				}
				if got, err := ParseVerdict(writeVerdict(t, q, f), "1-reviewer", "reviewer"); fmt.Sprint(err) != want || !reflect.DeepEqual(got, wantV) {
					t.Errorf("%s verdict with file %s %q: got %+v, %v; want %s", v, side.kind, e, got, err, want)
				}
			}
		}
	}
}
