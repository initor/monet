package testkit

import (
	"testing"
	"time"

	"gang/internal/store"
)

// LedgerRow appends a run/ledger.tsv row in the Contract 0.10 format. CLEAN
// rows have 0 blockers and both checks run, CHANGES rows have a blocker, and
// INCOMPLETE rows skipped the checks.
func LedgerRow(t *testing.T, d store.Dir, task, sha, lens, verdict string) {
	t.Helper()
	blockers, ran := "0", "true"
	switch verdict {
	case "CHANGES":
		blockers = "1"
	case "INCOMPLETE":
		ran = "false"
	}
	header := []string{"task", "sha", "lens", "verdict", "blockers", "majors", "survivors", "gate_ran", "mutation_checked", "evidence", "at"}
	row := []string{task, sha, lens, verdict, blockers, "0", "0", ran, ran, "review.md", time.Now().UTC().Format(time.RFC3339)}
	if err := store.AppendTSV(d.Run("ledger.tsv"), header, row); err != nil {
		t.Fatal(err)
	}
}
