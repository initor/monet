package ledger

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"gang/internal/plan"
	"gang/internal/store"
)

const (
	shaA = "1111111111111111111111111111111111111111"
	shaB = "2222222222222222222222222222222222222222"
)

func clean(task, sha string, lens Lens) Row {
	return Row{Task: task, SHA: sha, Lens: lens, GateRan: true, MutationChecked: true, Evidence: "run/tasks/" + task + "/review.md", At: "2026-10-11T09:00:00Z"}
}

func mustRecord(t *testing.T, d store.Dir, r Row) {
	t.Helper()
	if err := Record(d, r); err != nil {
		t.Fatal(err)
	}
}

// The table is Contract 4's verdict rule, case by case.
func TestCompute(t *testing.T) {
	cases := []struct {
		blockers, majors, survivors int
		gate, mutation              bool
		want                        Verdict
	}{
		{0, 0, 0, true, true, Clean},
		{1, 0, 0, true, true, Changes},
		{0, 2, 0, true, true, Changes},
		{0, 0, 1, true, true, Changes},
		{3, 1, 2, false, false, Changes},
		{0, 0, 0, false, true, Incomplete},
		{0, 0, 0, true, false, Incomplete},
		{0, 0, 0, false, false, Incomplete},
		{-1, 0, 0, true, true, Incomplete},
		{0, 0, -1, true, true, Incomplete},
	}
	for _, c := range cases {
		if got := Compute(c.blockers, c.majors, c.survivors, c.gate, c.mutation); got != c.want {
			t.Errorf("Compute(%d, %d, %d, %v, %v) = %s, want %s", c.blockers, c.majors, c.survivors, c.gate, c.mutation, got, c.want)
		}
	}
}

func TestRecordWritesContractRowAndComputesVerdict(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	r := clean("A1", shaA, LensReview)
	r.Blockers = 1
	r.Survivors = 2
	r.Verdict = Clean // a claimed verdict is ignored
	mustRecord(t, d, r)
	b, err := os.ReadFile(d.Run("ledger.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	wantHeader := "task\tsha\tlens\tverdict\tblockers\tmajors\tsurvivors\tgate_ran\tmutation_checked\tevidence\tat"
	if lines[0] != wantHeader {
		t.Fatalf("header = %q, want %q", lines[0], wantHeader)
	}
	wantRow := "A1\t" + shaA + "\treview\tCHANGES\t1\t0\t2\ttrue\ttrue\trun/tasks/A1/review.md\t2026-10-11T09:00:00Z"
	if len(lines) != 2 || lines[1] != wantRow {
		t.Fatalf("rows = %q, want one row %q", lines[1:], wantRow)
	}
}

func TestRecordRejectsBadInput(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	bad := map[string]func(r *Row){
		"empty task":   func(r *Row) { r.Task = "" },
		"short sha":    func(r *Row) { r.SHA = "1111111" },
		"upper sha":    func(r *Row) { r.SHA = strings.ToUpper("abcdefabcdefabcdefabcdefabcdefabcdefabcd") },
		"unknown lens": func(r *Row) { r.Lens = "style" },
		"negative":     func(r *Row) { r.Majors = -1 },
		"survivors":    func(r *Row) { r.Survivors = -1 },
		"no evidence":  func(r *Row) { r.Evidence = "" },
	}
	for name, mutate := range bad {
		r := clean("A1", shaA, LensReview)
		mutate(&r)
		if err := Record(d, r); err == nil {
			t.Errorf("%s: Record accepted %+v", name, r)
		}
	}
	if _, err := os.Stat(d.Run("ledger.tsv")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a rejected row reached the ledger: %v", err)
	}
}

func TestCheckNewestRowWins(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	changes := clean("A1", shaA, LensReview)
	changes.Majors = 1
	mustRecord(t, d, changes)
	mustRecord(t, d, clean("A1", shaA, LensReview))
	if ok, missing, err := Check(d, "A1", shaA, []Lens{LensReview}); err != nil || !ok || len(missing) != 0 {
		t.Fatalf("CHANGES then CLEAN: ok=%v missing=%v err=%v, want clean", ok, missing, err)
	}
	mustRecord(t, d, changes)
	ok, missing, err := Check(d, "A1", shaA, []Lens{LensReview})
	if err != nil || ok || !reflect.DeepEqual(missing, []Lens{LensReview}) {
		t.Fatalf("CLEAN then CHANGES: ok=%v missing=%v err=%v, want not clean on review", ok, missing, err)
	}
}

// A surviving mutant alone makes the row CHANGES, so the SHA is not clean.
func TestSurvivorMakesRowChanges(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	r := clean("A1", shaA, LensReview)
	r.Survivors = 1
	mustRecord(t, d, r)
	ok, missing, err := Check(d, "A1", shaA, []Lens{LensReview})
	if err != nil || ok || !reflect.DeepEqual(missing, []Lens{LensReview}) {
		t.Fatalf("ok=%v missing=%v err=%v, want not clean", ok, missing, err)
	}
	if got, _ := Summary(d); got[Changes] != 1 {
		t.Fatalf("summary %v, want one CHANGES row", got)
	}
}

func TestCheckIsKeyedByExactSHA(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	mustRecord(t, d, clean("A1", shaA, LensReview))
	if ok, _, _ := Check(d, "A1", shaB, []Lens{LensReview}); ok {
		t.Fatal("a clean row for one SHA verified another SHA")
	}
	if ok, _, _ := Check(d, "B2", shaA, []Lens{LensReview}); ok {
		t.Fatal("a clean row for one task verified another task")
	}
}

func TestCheckNeedsEveryRequiredLens(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	high := &plan.Task{ID: "H3", Risk: "high"}
	mustRecord(t, d, clean("H3", shaA, LensReview))
	ok, missing, err := Check(d, "H3", shaA, RequiredLenses(high))
	if err != nil || ok || !reflect.DeepEqual(missing, []Lens{LensAdversarial}) {
		t.Fatalf("review only: ok=%v missing=%v err=%v, want missing adversarial", ok, missing, err)
	}
	mustRecord(t, d, clean("H3", shaA, LensAdversarial))
	if ok, _, err := Check(d, "H3", shaA, RequiredLenses(high)); err != nil || !ok {
		t.Fatalf("both lenses clean: ok=%v err=%v", ok, err)
	}
}

func TestCheckEmptyLedgerAndNoLenses(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	ok, missing, err := Check(d, "A1", shaA, []Lens{LensReview})
	if err != nil || ok || !reflect.DeepEqual(missing, []Lens{LensReview}) {
		t.Fatalf("empty ledger: ok=%v missing=%v err=%v", ok, missing, err)
	}
	if _, _, err := Check(d, "A1", shaA, nil); err == nil {
		t.Fatal("Check with no lenses must fail closed")
	}
}

func TestSanitizedEvidenceKeepsRowsReadable(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	r := clean("A1", shaA, LensReview)
	r.Evidence = "a\tb\nc"
	mustRecord(t, d, r)
	if ok, _, err := Check(d, "A1", shaA, []Lens{LensReview}); err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

// Rule 10 writes booleans as true or false. Any other spelling is a
// malformed row, and Check fails closed on it.
func TestCheckRejectsOtherBooleanSpellings(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	mustRecord(t, d, clean("A1", shaA, LensReview))
	path := d.Run("ledger.tsv")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\t0\ttrue\ttrue\t") {
		t.Fatalf("row does not spell booleans as true: %q", b)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), "\ttrue\ttrue\t", "\t1\tTRUE\t", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, _, err := Check(d, "A1", shaA, []Lens{LensReview}); err == nil || ok {
		t.Fatalf("ok=%v err=%v, want an error for TRUE", ok, err)
	}
}

func TestRequiredLenses(t *testing.T) {
	if got := RequiredLenses(&plan.Task{Risk: "low"}); !reflect.DeepEqual(got, []Lens{LensReview}) {
		t.Errorf("low risk: %v", got)
	}
	if got := RequiredLenses(&plan.Task{Risk: "high"}); !reflect.DeepEqual(got, []Lens{LensReview, LensAdversarial}) {
		t.Errorf("high risk: %v", got)
	}
}

func TestSummary(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	got, err := Summary(d)
	if err != nil || !reflect.DeepEqual(got, map[Verdict]int{Clean: 0, Changes: 0, Incomplete: 0}) {
		t.Fatalf("empty: %v %v", got, err)
	}
	mustRecord(t, d, clean("A1", shaA, LensReview))
	changes := clean("B2", shaB, LensReview)
	changes.Blockers = 2
	mustRecord(t, d, changes)
	incomplete := clean("B2", shaB, LensAdversarial)
	incomplete.MutationChecked = false
	mustRecord(t, d, incomplete)
	mustRecord(t, d, clean("B2", shaB, LensReview))
	got, err = Summary(d)
	if err != nil || !reflect.DeepEqual(got, map[Verdict]int{Clean: 2, Changes: 1, Incomplete: 1}) {
		t.Fatalf("got %v %v", got, err)
	}
}
