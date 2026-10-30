package prq

import (
	"os"
	"slices"
	"testing"

	"gang/internal/store"
)

func TestPutReplacesTheTaskRow(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	open := Row{Task: "T1", Branch: "gang/demo/T1", HeadSHA: "abc1234", State: StateOpen, OpenedAt: "2026-10-29T10:00:00Z"}
	other := Row{Task: "T2", Branch: "a\tb", State: StateClosed}
	merged := open
	merged.PR, merged.State, merged.Fixes = 7, StateMerged, 2
	for _, r := range []Row{open, other, merged} {
		if err := Put(d, r); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(d.Run("prs.tsv"))
	want := "task\tpr\tbranch\thead_sha\tstate\topened_at\tfixes\n" +
		"T1\t7\tgang/demo/T1\tabc1234\tmerged\t2026-10-29T10:00:00Z\t2\n" +
		"T2\t0\ta b\t\tclosed\t\t0\n"
	if err != nil || string(raw) != want {
		t.Fatalf("prs.tsv = %q, want %q", raw, want)
	}
	rows, err := Read(d)
	if err != nil || !slices.Equal(rows, []Row{merged, {Task: "T2", Branch: "a b", State: StateClosed}}) {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
}

func TestReadWithoutAFile(t *testing.T) {
	rows, err := Read(store.Dir{Root: t.TempDir()})
	if err != nil || len(rows) != 0 {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
}

func TestOpenFindsTheOpenRow(t *testing.T) {
	merged := Row{Task: "T1", State: StateMerged}
	closed := Row{Task: "T3", State: StateClosed}
	open := Row{Task: "T2", State: StateOpen}
	if got, ok := Open([]Row{merged, open}); !ok || got != open {
		t.Fatalf("Open = %+v, %v; want %+v, true", got, ok, open)
	}
	if got, ok := Open([]Row{merged, closed}); ok {
		t.Fatalf("Open = %+v, true; want no open row", got)
	}
}
