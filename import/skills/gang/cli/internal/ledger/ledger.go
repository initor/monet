// Package ledger keeps review verdicts in run/ledger.tsv, one row per
// (task, sha, lens) review. Rows are never edited. The verdict is computed
// from the counts and the two checks, never taken from the caller.
// A surviving mutant counts against the verdict like a blocker (Contract 4).
package ledger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"gang/internal/plan"
	"gang/internal/store"
)

type Lens string

const (
	LensReview      Lens = "review"
	LensAdversarial Lens = "adversarial"
)

type Verdict string

const (
	Clean      Verdict = "CLEAN"
	Changes    Verdict = "CHANGES"
	Incomplete Verdict = "INCOMPLETE"
)

type Row struct {
	Task, SHA                string
	Lens                     Lens
	Verdict                  Verdict
	Blockers, Majors         int
	Survivors                int
	GateRan, MutationChecked bool
	Evidence, At             string
}

// header is the ledger.tsv header (Contract 0 rule 10).
var header = []string{"task", "sha", "lens", "verdict", "blockers", "majors", "survivors", "gate_ran", "mutation_checked", "evidence", "at"}

var shaRE = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Compute is the only source of a verdict (Contract 4): CHANGES when any
// count is above 0, CLEAN when all are 0 and both checks ran, INCOMPLETE
// otherwise.
func Compute(blockers, majors, survivors int, gateRan, mutationChecked bool) Verdict {
	if blockers > 0 || majors > 0 || survivors > 0 {
		return Changes
	}
	if blockers == 0 && majors == 0 && survivors == 0 && gateRan && mutationChecked {
		return Clean
	}
	return Incomplete
}

// RequiredLenses is review, plus adversarial for a risk-high task.
func RequiredLenses(t *plan.Task) []Lens {
	if t.Risk == "high" {
		return []Lens{LensReview, LensAdversarial}
	}
	return []Lens{LensReview}
}

// Record appends one row. It computes the verdict itself and ignores r.Verdict.
func Record(d store.Dir, r Row) error {
	switch {
	case r.Task == "":
		return errors.New("ledger: empty task id")
	case !shaRE.MatchString(r.SHA):
		return fmt.Errorf("ledger: sha %q is not 40 lowercase hex characters", r.SHA)
	case r.Lens != LensReview && r.Lens != LensAdversarial:
		return fmt.Errorf("ledger: lens %q is not review or adversarial", r.Lens)
	case r.Blockers < 0 || r.Majors < 0 || r.Survivors < 0:
		return errors.New("ledger: blockers, majors, and survivors must be 0 or more")
	case r.Evidence == "":
		return errors.New("ledger: evidence path is empty")
	}
	if r.At == "" {
		r.At = time.Now().UTC().Format(time.RFC3339)
	}
	r.Verdict = Compute(r.Blockers, r.Majors, r.Survivors, r.GateRan, r.MutationChecked)
	path := d.Run("ledger.tsv")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return store.AppendTSV(path, header, []string{
		r.Task, r.SHA, string(r.Lens), string(r.Verdict),
		strconv.Itoa(r.Blockers), strconv.Itoa(r.Majors), strconv.Itoa(r.Survivors),
		strconv.FormatBool(r.GateRan), strconv.FormatBool(r.MutationChecked),
		r.Evidence, r.At,
	})
}

// rows reads every row in file order. The verdict is recomputed from the
// counts, so a hand-edited verdict column cannot make a row clean.
func rows(d store.Dir) ([]Row, error) {
	path := d.Run("ledger.tsv")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	cells, err := store.ReadTSV(path, header)
	if err != nil {
		return nil, err
	}
	out := make([]Row, 0, len(cells))
	for i, c := range cells {
		if len(c) != len(header) {
			return nil, fmt.Errorf("%s row %d: %d cells, want %d", path, i+1, len(c), len(header))
		}
		b, err1 := strconv.Atoi(c[4])
		m, err2 := strconv.Atoi(c[5])
		s, err3 := strconv.Atoi(c[6])
		g, err4 := parseBool(c[7])
		mc, err5 := parseBool(c[8])
		if err := errors.Join(err1, err2, err3, err4, err5); err != nil {
			return nil, fmt.Errorf("%s row %d: %w", path, i+1, err)
		}
		out = append(out, Row{
			Task: c[0], SHA: c[1], Lens: Lens(c[2]), Verdict: Compute(b, m, s, g, mc),
			Blockers: b, Majors: m, Survivors: s, GateRan: g, MutationChecked: mc, Evidence: c[9], At: c[10],
		})
	}
	return out, nil
}

// parseBool accepts only the two spellings Record writes.
func parseBool(s string) (bool, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("boolean %q is not true or false", s)
}

// Check is clean only when, for every lens, the newest row for (task, sha,
// lens) is CLEAN. missing lists the lenses that are not clean, in order.
func Check(d store.Dir, task, sha string, lenses []Lens) (ok bool, missing []Lens, err error) {
	if len(lenses) == 0 {
		return false, nil, errors.New("ledger: no lenses to check")
	}
	all, err := rows(d)
	if err != nil {
		return false, nil, err
	}
	newest := map[Lens]Verdict{}
	for _, r := range all {
		if r.Task == task && r.SHA == sha {
			newest[r.Lens] = r.Verdict
		}
	}
	for _, l := range lenses {
		if newest[l] != Clean {
			missing = append(missing, l)
		}
	}
	return len(missing) == 0, missing, nil
}

// Summary counts every row by verdict. All three verdicts are always present.
func Summary(d store.Dir) (map[Verdict]int, error) {
	all, err := rows(d)
	if err != nil {
		return nil, err
	}
	m := map[Verdict]int{Clean: 0, Changes: 0, Incomplete: 0}
	for _, r := range all {
		m[r.Verdict]++
	}
	return m, nil
}
