package prq

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/store"
	"gang/internal/testkit"
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

func TestCurrentFindsTheOpenRow(t *testing.T) {
	merged := Row{Task: "T1", State: StateMerged}
	closed := Row{Task: "T3", State: StateClosed}
	open := Row{Task: "T2", State: StateOpen}
	if got, ok := Current([]Row{merged, open}); !ok || got != open {
		t.Fatalf("Current = %+v, %v; want %+v, true", got, ok, open)
	}
	if got, ok := Current([]Row{merged, closed}); ok {
		t.Fatalf("Current = %+v, true; want no open row", got)
	}
}

func prView(state, head string, checks ...string) string {
	return fmt.Sprintf(`{"state":%q,"mergeable":"MERGEABLE","headRefOid":%q,"statusCheckRollup":[%s]}`, state, head, strings.Join(checks, ","))
}

func TestSetStateKeepsANewerHead(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	for _, head := range []string{"h1", "h2"} {
		if err := Put(d, Row{Task: "T1", PR: 7, HeadSHA: head, State: StateOpen}); err != nil {
			t.Fatal(err)
		}
	}
	err := SetState(d, "T1", StateMerged)
	rows, readErr := Read(d)
	if want := []Row{{Task: "T1", PR: 7, HeadSHA: "h2", State: StateMerged}}; err != nil || readErr != nil || !slices.Equal(rows, want) {
		t.Fatalf("rows = %+v, errs = %v, %v, want %+v", rows, err, readErr, want)
	}
	if err := SetState(d, "T9", StateMerged); err == nil || err.Error() != "prs.tsv has no row for T9" {
		t.Fatalf("SetState of a missing row = %v", err)
	}
}

func TestLiveRefusesUnlessTheRecordedPRIsOpen(t *testing.T) {
	d, p := store.Dir{Root: t.TempDir()}, testkit.Plan()
	state, _ := testkit.FakeGH(t)
	if err := write(d, []Row{{Task: "T0", PR: 6, State: StateMerged}, {Task: "T1", PR: 7, HeadSHA: "h1", State: StateOpen}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ id, view, refusal string }{
		{"T2", prView("OPEN", "h1"), "T2 has no open PR"},
		{"T1", prView("CLOSED", "h1"), "PR #7 of T1 is CLOSED on GitHub"},
		{"T1", prView("OPEN", "h2"), "PR #7 head moved: h2, recorded h1"},
		{"T1", prView("OPEN", "0123456789"), "PR #7 head moved: 0123456, recorded h1"},
	} {
		testkit.Answer(t, state, "pr-view.json", c.view)
		_, _, err := Live(d, p, c.id)
		wantExit(t, err, exit.Refused, c.refusal)
	}
	testkit.Answer(t, state, "pr-view.json", prView("OPEN", "h1"))
	row, st, err := Live(d, p, "T1")
	if err != nil || row.PR != 7 || st.HeadSHA != "h1" {
		t.Fatalf("Live = %+v, %+v, %v, want PR 7 at h1", row, st, err)
	}
	p.Repo.GHAccount = ""
	_, _, err = Live(d, p, "T1")
	wantExit(t, err, exit.Refused, "repo.gh_account is empty: gang makes no GitHub call for T1")
	if got := testkit.Calls(t, state); len(got) != 4 || !strings.HasPrefix(got[3], "pr view 7 --repo github.com/acme/widget ") {
		t.Fatalf("gh calls = %q, want one for each Live that reached GitHub, the last for PR 7 of github.com/acme/widget", got)
	}
}
