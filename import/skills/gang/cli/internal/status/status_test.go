package status

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gang/internal/ledger"
	"gang/internal/testkit"
)

func TestWriteSummarizesRunState(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

	// Before anything happens.
	lines, err := Write(repo, d, p, now)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"merged 0/2 tasks", "ledger CLEAN 0 CHANGES 0 INCOMPLETE 0", "gates open 0", "wave PR none", "driver free"}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("empty run: %q", lines)
	}

	// T1 merged (git), two ledger rows, one open and one answered gate,
	// wave 1 open, and a live driver lease.
	tip := testkit.CommitOn(t, repo, p.Repo.Integration, "T1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
	testkit.LedgerRow(t, d, "T1", tip, "review", "CLEAN")
	testkit.LedgerRow(t, d, "T2", "abc", "review", "CHANGES")
	write(t, d.Run("gates.md"), "## G1 T2 open\nQuestion: which db?\nDefault: sqlite\n\n## G2 T1 answered\nQuestion: q\nDefault: d\nAnswer: a\n")
	write(t, d.Run("waves.tsv"), "wave\tpr\thead_sha\tstate\topened_at\n1\t7\t"+tip+"\topen\t2026-10-11T00:00:00Z\n")
	expires := now.Add(2 * time.Hour)
	write(t, d.Run("locks", "driver.json"), fmt.Sprintf(`{"name":"driver","holder":"s1","pid":1,"host":"h","expires":%q}`, expires.Format(time.RFC3339)))

	sum, err := ledger.Summary(d)
	if err != nil {
		t.Fatal(err)
	}
	lines, err = Write(repo, d, p, now)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{
		"merged 1/2 tasks",
		fmt.Sprintf("ledger CLEAN %d CHANGES %d INCOMPLETE %d", sum[ledger.Clean], sum[ledger.Changes], sum[ledger.Incomplete]),
		"gates open 1",
		"wave PR 1 #7 open",
		"driver s1 until " + expires.Format(time.RFC3339),
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("got %q\nwant %q", lines, want)
	}
	if sum[ledger.Clean] != 1 || sum[ledger.Changes] != 1 {
		t.Fatalf("ledger fixture not read as written: %v", sum)
	}

	// The lease expires: the driver is free.
	lines, _ = Write(repo, d, p, expires)
	if lines[4] != "driver free" {
		t.Fatalf("expired lease: %q", lines[4])
	}
}

func TestWritePicksLatestClosedWave(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	write(t, d.Run("waves.tsv"), "wave\tpr\thead_sha\tstate\topened_at\n1\t7\ta\tmerged\tx\n2\t9\tb\tmerged\ty\n")
	lines, err := Write(repo, d, p, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if lines[3] != "wave PR 2 #9 merged" {
		t.Fatalf("got %q", lines[3])
	}
}

func TestStatusMDKeepsTheCoordinatorLog(t *testing.T) {
	repo := testkit.Repo(t)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	if _, err := Write(repo, d, p, now); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(d.Run("status.md"))
	log := "## Log\n\n- 12:05 split T2 into T2a and T2b\n- 12:30 wave 1 PR ready\n"
	write(t, d.Run("status.md"), string(b[:strings.Index(string(b), "## Log")])+log)

	if _, err := Write(repo, d, p, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(d.Run("status.md"))
	got := string(b)
	for _, want := range []string{"# gang status: demo\n", "Updated 2026-10-11T13:00:00Z.", "- merged 0/2 tasks\n", "| T1 | 1 | no |\n", log} {
		if !strings.Contains(got, want) {
			t.Errorf("status.md lacks %q:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, log) || strings.Count(got, "## Log") != 1 {
		t.Errorf("log not kept once at the end:\n%s", got)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
