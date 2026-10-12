package wave

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
	"gang/internal/testkit"
)

type waveFixture struct {
	repo, origin string
	d            store.Dir
	p            *plan.Plan
}

// newWaveFixture: a repo with a bare origin, and an integration branch where
// gang merge has landed T1 (wave 1). T2 (wave 2) is not merged.
func newWaveFixture(t *testing.T) waveFixture {
	t.Helper()
	repo := testkit.Repo(t)
	origin := testkit.Origin(t, repo)
	p := testkit.Plan()
	d := testkit.GangDir(t, repo, p)
	testkit.Git(t, repo, "branch", p.Repo.Integration, "main")
	testkit.MergeTask(t, repo, p.Repo.Integration, "T1", map[string]string{"a/T1.go": "package a\n"})
	return waveFixture{repo: repo, origin: origin, d: d, p: p}
}

func remoteRef(t *testing.T, bare, branch string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", bare, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func wantExit(t *testing.T, err error, code int, msg string) {
	t.Helper()
	var e *exit.Error
	if !errors.As(err, &e) || e.Code != code || !strings.Contains(e.Msg, msg) {
		t.Fatalf("err = %v, want exit %d containing %q", err, code, msg)
	}
}

func TestPushCreatesThenFastForwards(t *testing.T) {
	f := newWaveFixture(t)
	tip := testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang-demo")
	got, err := Push(f.repo, f.d, f.p, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != tip || remoteRef(t, f.origin, "gang-demo") != tip {
		t.Fatalf("pushed %s, remote %s, want %s", got, remoteRef(t, f.origin, "gang-demo"), tip)
	}
	next := testkit.CommitOn(t, f.repo, "gang-demo", "T2: build\n\nTask: T2", map[string]string{"b/T2.go": "package b\n"})
	if _, err := Push(f.repo, f.d, f.p, false); err != nil {
		t.Fatal(err)
	}
	if remoteRef(t, f.origin, "gang-demo") != next {
		t.Fatal("second push did not fast-forward")
	}
	if _, err := os.Stat(f.d.Run("waves.tsv")); !os.IsNotExist(err) {
		t.Fatal("a push with no wave PR wrote waves.tsv")
	}
}

func TestPushRefusesASecretDeletedLater(t *testing.T) {
	f := newWaveFixture(t)
	key := "AK" + "IA" + strings.Repeat("Q", 16)
	testkit.CommitOn(t, f.repo, "gang-demo", "add env", map[string]string{"a/app.env": "A=1\nKEY=" + key + "\n"})
	testkit.CommitOn(t, f.repo, "gang-demo", "drop key", map[string]string{"a/app.env": "A=1\n"})
	_, err := Push(f.repo, f.d, f.p, false)
	wantExit(t, err, exit.Refused, "a/app.env:2:aws-access-key")
	if remoteRef(t, f.origin, "gang-demo") != "" {
		t.Fatal("a refused push reached origin")
	}
}

// A merge whose tree adds a line neither parent has must still be scanned.
func TestPushScansMergeResolutions(t *testing.T) {
	f := newWaveFixture(t)
	side := testkit.CommitOn(t, f.repo, "side", "side", map[string]string{"s.txt": "side\n"})
	before := testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang-demo")
	evil := testkit.CommitOn(t, f.repo, "gang-demo", "tmp", map[string]string{
		"s.txt": "side\n", "a/tok.txt": "sk-" + "ant-" + strings.Repeat("z", 24) + "\n",
	})
	tree := testkit.Git(t, f.repo, "rev-parse", evil+"^{tree}")
	merge := testkit.Git(t, f.repo, "commit-tree", tree, "-p", before, "-p", side, "-m", "merge side")
	testkit.Git(t, f.repo, "update-ref", "refs/heads/gang-demo", merge)
	_, err := Push(f.repo, f.d, f.p, true)
	wantExit(t, err, exit.Refused, "a/tok.txt:1:anthropic-key")
}

// git log -p prints "Binary files differ" for a file with a NUL byte or a
// -diff attribute, and a patch never holds the commit message. The scan must
// still see a secret in each.
func TestPushScansBinaryFilesAndMessages(t *testing.T) {
	key := "AK" + "IA" + strings.Repeat("Q", 16)
	t.Run("a file with a NUL byte", func(t *testing.T) {
		f := newWaveFixture(t)
		testkit.CommitOn(t, f.repo, "gang-demo", "add blob", map[string]string{"a/blob.bin": "\x00\x01\nKEY=" + key + "\n"})
		_, err := Push(f.repo, f.d, f.p, true)
		wantExit(t, err, exit.Refused, "a/blob.bin:2:aws-access-key")
	})
	t.Run("a file with a -diff attribute", func(t *testing.T) {
		f := newWaveFixture(t)
		if err := os.WriteFile(filepath.Join(f.repo, ".gitattributes"), []byte("*.dat -diff\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		testkit.CommitOn(t, f.repo, "gang-demo", "add data", map[string]string{"a/keys.dat": "KEY=" + key + "\n"})
		_, err := Push(f.repo, f.d, f.p, true)
		wantExit(t, err, exit.Refused, "a/keys.dat:1:aws-access-key")
	})
	t.Run("a commit message", func(t *testing.T) {
		f := newWaveFixture(t)
		sha := testkit.CommitOn(t, f.repo, "gang-demo", "add config\n\nIt used KEY="+key+" once.", map[string]string{"a/c.txt": "c\n"})
		_, err := Push(f.repo, f.d, f.p, true)
		wantExit(t, err, exit.Refused, "commit "+sha[:7]+" message:3:aws-access-key")
	})
}

func TestPushRefusesNonFastForward(t *testing.T) {
	f := newWaveFixture(t)
	other := testkit.CommitOn(t, f.repo, "other", "other", map[string]string{"o.txt": "o\n"})
	testkit.Git(t, f.repo, "push", "-q", "origin", other+":refs/heads/gang-demo")
	_, err := Push(f.repo, f.d, f.p, false)
	wantExit(t, err, exit.Refused, "not a fast-forward")
	if remoteRef(t, f.origin, "gang-demo") != other {
		t.Fatal("origin changed")
	}
}

func TestPushDryRunPrintsCommand(t *testing.T) {
	f := newWaveFixture(t)
	tip := testkit.Git(t, f.repo, "rev-parse", "refs/heads/gang-demo")
	got, err := Push(f.repo, f.d, f.p, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := "git -C " + f.repo + " push origin " + tip + ":refs/heads/gang-demo"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if remoteRef(t, f.origin, "gang-demo") != "" {
		t.Fatal("dry run pushed")
	}
}

func TestPushNeedsIntegrationBranch(t *testing.T) {
	f := newWaveFixture(t)
	testkit.Git(t, f.repo, "update-ref", "-d", "refs/heads/gang-demo")
	_, err := Push(f.repo, f.d, f.p, false)
	wantExit(t, err, exit.Refused, "integration branch gang-demo not found")
}

func TestWavesTSVRoundTrip(t *testing.T) {
	f := newWaveFixture(t)
	rows := []Row{{Wave: 1, PR: 7, HeadSHA: "abc", State: StateMerged, OpenedAt: "2026-10-11T00:00:00Z"}, {Wave: 2, PR: 9, HeadSHA: "def", State: StateOpen, OpenedAt: "2026-10-12T00:00:00Z"}}
	if err := write(f.d, rows); err != nil {
		t.Fatal(err)
	}
	if err := SetState(f.d, 2, StateClosed); err != nil {
		t.Fatal(err)
	}
	got, err := Read(f.d)
	if err != nil {
		t.Fatal(err)
	}
	rows[1].State = StateClosed
	if len(got) != 2 || got[0] != rows[0] || got[1] != rows[1] {
		t.Fatalf("got %+v", got)
	}
	recs, err := store.ReadTSV(f.d.Run("waves.tsv"), []string{"wave", "pr", "head_sha", "state", "opened_at"})
	if err != nil || len(recs) != 2 {
		t.Fatalf("waves.tsv does not hold the Contract 4 header: %v", err)
	}
	wantExit(t, SetState(f.d, 5, StateMerged), exit.Refused, "wave 5 has no row")
}

func TestDoneFiltersToPlanTasks(t *testing.T) {
	f := newWaveFixture(t)
	testkit.CommitOn(t, f.repo, "gang-demo", "repair\n\nTask: F01", map[string]string{"r": "r\n"})
	testkit.CommitOn(t, f.repo, "gang-demo", "a trailer-like line in prose: Task: T2 is next", map[string]string{"n": "n\n"})
	done, err := Done(f.repo, f.p)
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 || !done["T1"] {
		t.Fatalf("done = %v, want only T1 (F01 is not a plan task, and T2 is not on a line of its own)", done)
	}
	testkit.Git(t, f.repo, "update-ref", "-d", "refs/heads/gang-demo")
	if done, err = Done(f.repo, f.p); err != nil || len(done) != 0 {
		t.Fatalf("no integration branch: done = %v, err = %v", done, err)
	}
}
