package prq

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
	"gang/internal/testkit"
)

func queueFixture(t *testing.T) (repo string, d store.Dir, p *plan.Plan) {
	t.Helper()
	repo, p = testkit.Repo(t), testkit.Plan()
	testkit.Origin(t, repo)
	d = testkit.GangDir(t, repo, p)
	testkit.Git(t, repo, "branch", p.Repo.Integration, "main")
	land(t, repo, d, "T1", map[string]string{"a/T1.go": "package a\n"})
	return repo, d, p
}

// land merges task id onto gang-demo and records a CLEAN review of its tip.
func land(t *testing.T, repo string, d store.Dir, id string, files map[string]string) {
	t.Helper()
	tip := testkit.MergeTask(t, repo, "gang-demo", id, files)
	testkit.LedgerRow(t, d, id, tip, "review", "CLEAN")
}

func wantExit(t *testing.T, err error, code int, msg string) {
	t.Helper()
	var e *exit.Error
	if !errors.As(err, &e) || e.Code != code || !strings.Contains(e.Msg, msg) {
		t.Fatalf("err = %v, want exit %d containing %q", err, code, msg)
	}
}

// shipOnOrigin lands a commit on origin/main that only a fetch can see: the
// push moves the local tracking ref, so the ref goes back to where it was.
func shipOnOrigin(t *testing.T, repo, msg string, files map[string]string) {
	t.Helper()
	old := testkit.Git(t, repo, "rev-parse", "refs/remotes/origin/main")
	testkit.CommitOn(t, repo, "ship", msg, files)
	testkit.Git(t, repo, "push", "-q", "origin", "ship:main")
	testkit.Git(t, repo, "update-ref", "refs/remotes/origin/main", old)
}

func TestPrepareChecksTheQueue(t *testing.T) {
	for _, c := range []struct {
		name, id, refusal string // refusal "" means Prepare succeeds
		rows              []Row
	}{
		{"the next task", "T1", "", nil},
		{"a PR is open", "T2", "PR #7 for T1 is open", []Row{{Task: "T1", PR: 7, State: StateOpen}}},
		{"another task is next", "T2", "the next task to ship is T1, not T2", nil},
		{"nothing left", "T1", "nothing left to ship", []Row{{Task: "T1", PR: 7, State: StateMerged}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, d, p := queueFixture(t)
			for _, r := range c.rows {
				if err := Put(d, r); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Prepare(repo, d, p, c.id)
			if c.refusal != "" {
				wantExit(t, err, exit.Refused, c.refusal)
			} else if err != nil || got.ID != "T1" || got.Subject != "T1: work" || got.Branch != "gang/demo/T1" || got.Task == nil || got.Task.ID != "T1" {
				t.Fatalf(`Prepare = %+v, %v, want T1, "T1: work", gang/demo/T1, task T1`, got, err)
			}
		})
	}
}

func TestPrepareReturnsAStrayError(t *testing.T) {
	repo, d, p := queueFixture(t)
	testkit.CommitOn(t, repo, p.Repo.Integration, "chore: no task", nil)
	if _, err := Prepare(repo, d, p, "T1"); err == nil || !strings.Contains(err.Error(), "chore: no task") {
		t.Fatalf("Prepare error = %v, want one that names the stray", err)
	}
}

// T2 is cut beside T1, as in a 0.4.0 wave. T1 ships on origin behind the local ref, so only a fetch skips it.
func TestPrepareReplaysOnlyTheTask(t *testing.T) {
	repo, d, p := queueFixture(t)
	tip := testkit.CommitOn(t, repo, "gang/T2", "T2: work\n\nTask: T2", map[string]string{"b/T2.go": "package b\n"})
	testkit.GangMerge(t, repo, p.Repo.Integration, "T2", "Build T2", tip)
	testkit.LedgerRow(t, d, "T2", tip, "review", "CLEAN")
	shipOnOrigin(t, repo, "feat: ship t1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n", "x.txt": "x\n"})
	c, err := Prepare(repo, d, p, "T2")
	if err != nil {
		t.Fatal(err)
	}
	shipped := testkit.Git(t, repo, "rev-parse", "ship")
	for name, v := range map[string][2]string{
		"base":    {c.Base, shipped},
		"parents": {testkit.Git(t, repo, "rev-list", "--parents", "-n", "1", c.SHA), c.SHA + " " + shipped},
		"tree":    {testkit.Git(t, repo, "ls-tree", "-r", "--name-only", c.SHA), "README.md\na/T1.go\nb/T2.go\nx.txt"},
		"message": {testkit.Git(t, repo, "log", "-1", "--format=%B", c.SHA), c.Message},
	} {
		if v[0] != v[1] {
			t.Errorf("%s = %q, want %q", name, v[0], v[1])
		}
	}
}

func TestPrepareRefusesWhatCannotShip(t *testing.T) {
	for _, c := range []struct {
		name, id string
		code     int
		msg      string
		setup    func(t *testing.T, repo string, d store.Dir)
	}{
		{"already on the base", "T1", exit.Refused, "T1 is already on origin/main", func(t *testing.T, repo string, _ store.Dir) {
			shipOnOrigin(t, repo, "chore: same a", map[string]string{"a/T1.go": "package a\n"})
		}},
		{"conflict", "T1", exit.Conflict, "T1 does not replay onto origin/main: a/T1.go", func(t *testing.T, repo string, _ store.Dir) {
			shipOnOrigin(t, repo, "chore: other a", map[string]string{"a/T1.go": "package other\n"})
		}},
		{"unreviewed", "T2", exit.Refused, "T2 tip ", func(t *testing.T, repo string, _ store.Dir) {
			testkit.MergeTask(t, repo, "gang-demo", "T2", map[string]string{"b/T2.go": "package b\n"})
			shipOnOrigin(t, repo, "feat: ship t1\n\nTask: T1", map[string]string{"x.txt": "x\n"})
		}},
		{"not in the plan", "Z9", exit.Refused, "Z9 is not a task of the plan", func(t *testing.T, repo string, d store.Dir) {
			testkit.CommitOn(t, repo, "gang-demo", "feat: z\n\nTask: Z9", map[string]string{"z.txt": "z\n"})
			if err := Put(d, Row{Task: "T1", PR: 7, State: StateMerged}); err != nil {
				t.Fatal(err)
			}
		}},
		{"secret", "T2", exit.Refused, "k.pem:1:private-key", func(t *testing.T, repo string, d store.Dir) {
			land(t, repo, d, "T2", map[string]string{"k.pem": "-----BEGIN " + "EC PRIVATE KEY-----\n"})
			shipOnOrigin(t, repo, "feat: ship t1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n"})
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, d, p := queueFixture(t)
			c.setup(t, repo, d)
			_, err := Prepare(repo, d, p, c.id)
			wantExit(t, err, c.code, c.msg)
		})
	}
}

func TestReviewedNeedsEveryRequiredLens(t *testing.T) {
	_, d, p := queueFixture(t)
	p.Tasks[0].Risk = "high"
	tip := strings.Repeat("a", 40)
	testkit.LedgerRow(t, d, "T1", tip, "review", "CLEAN")
	_, err := reviewed(d, p, "T1", tip)
	wantExit(t, err, exit.Refused, "T1 tip aaaaaaa is not ledger CLEAN: missing [adversarial]")
	testkit.LedgerRow(t, d, "T1", tip, "adversarial", "CLEAN")
	if _, err := reviewed(d, p, "T1", tip); err != nil {
		t.Fatalf("reviewed with both lenses = %v, want nil", err)
	}
}

func TestReviewedReturnsALedgerReadError(t *testing.T) {
	_, d, p := queueFixture(t)
	if err := store.WriteFileAtomic(d.Run("ledger.tsv"), []byte("task\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := reviewed(d, p, "T1", strings.Repeat("a", 40)); err == nil || errors.As(err, new(*exit.Error)) {
		t.Fatalf("reviewed on an unreadable ledger = %v, want a plain read error", err)
	}
}

func TestPrepareStopsWhenTheFetchFails(t *testing.T) {
	repo, d, p := queueFixture(t)
	testkit.Git(t, repo, "remote", "set-url", "origin", t.TempDir()+"/gone.git")
	if _, err := Prepare(repo, d, p, "T1"); err == nil || !strings.HasPrefix(err.Error(), "fetch origin: ") || errors.As(err, new(*exit.Error)) {
		t.Fatalf("Prepare error = %v, want one that starts with %q", err, "fetch origin: ")
	}
}

// Each row pushes a head onto origin/gang/demo/T1. shipOnOrigin moves the
// base to "ship", and the local origin/main stays at the old base. A gone row
// then deletes the branch inside origin, which leaves its tracking ref stale.
// A local tag survives every row: only gone tracking refs are pruned.
func TestPrepareReusesOnlyAnEqualBranch(t *testing.T) {
	const another = "origin/gang/demo/T1 exists at %s with another change or base: gang never moves it"
	for _, c := range []struct {
		name, tree, parents, msg, want string
		gone                           bool
	}{
		{"equal", "gang-demo", "-p ship", "pushed", "", false},
		{"another tree", "origin/main", "-p ship", "pushed", another, false},
		{"another base", "gang-demo", "-p origin/main", "pushed", another, false},
		{"another base, deleted on origin", "gang-demo", "-p origin/main", "pushed", "", true},
		{"two parents", "gang-demo", "-p ship -p gang-demo", "pushed", another, false},
		{"secret on the pushed head", "gang-demo", "-p ship", "-----BEGIN " + "EC PRIVATE KEY-----", "private-key", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, d, p := queueFixture(t)
			shipOnOrigin(t, repo, "chore: empty", nil)
			head := testkit.Git(t, repo, append([]string{"commit-tree", c.tree + "^{tree}", "-m", c.msg}, strings.Fields(c.parents)...)...)
			testkit.Git(t, repo, "push", "-q", "origin", head+":refs/heads/gang/demo/T1")
			if c.gone {
				origin := testkit.Git(t, repo, "remote", "get-url", "origin")
				testkit.Git(t, origin, "update-ref", "-d", "refs/heads/gang/demo/T1")
			}
			testkit.Git(t, repo, "tag", "local-only")
			got, err := Prepare(repo, d, p, "T1")
			testkit.Git(t, repo, "rev-parse", "-q", "--verify", "refs/tags/local-only")
			if c.want != "" {
				wantExit(t, err, exit.Refused, strings.Replace(c.want, "%s", head[:7], 1))
			} else if err != nil || (got.SHA == head) == c.gone {
				t.Fatalf("Prepare = %q, %v, want the pushed head %s unless it is gone (gone = %v)", got.SHA, err, head, c.gone)
			}
		})
	}
}

func TestReuseReturnsAGitError(t *testing.T) {
	if err := (&Prepared{Branch: "gang/demo/T1"}).reuse(t.TempDir(), ""); err == nil || errors.As(err, new(*exit.Error)) {
		t.Fatalf("reuse outside a repo = %v, want a plain git error", err)
	}
}

func TestBodyNamesGoalRequirementsReviewAndSize(t *testing.T) {
	for _, c := range []struct{ risk, ids, reqs, lenses, src, lines string }{
		{"", "R1 R2", "R1, R2", "[review]", "package b\n", "1"},
		{"high", "", "-", "[review adversarial]", "package b\n\nvar B = 1\n", "3"},
	} {
		repo, d, p := queueFixture(t)
		land(t, repo, d, "T2", map[string]string{"b/T2.go": c.src, "go.sum": "x\n"})
		shipOnOrigin(t, repo, "feat: ship t1\n\nTask: T1", map[string]string{"a/T1.go": "package a\n", "x.txt": "base\n"})
		prep, err := Prepare(repo, d, p, "T2")
		if err != nil {
			t.Fatal(err)
		}
		prep.Task.Risk, prep.Task.Requirements = c.risk, strings.Fields(c.ids)
		got, err := Body(repo, p, prep)
		want := "Task T2 of gang run `demo`: Make T2 pass its test.\n\n- Requirements: " + c.reqs + "\n- Review: CLEAN " + c.lenses + " at " + prep.Tip[:7] + "\n- Changed lines: " + c.lines + "\n\nOpened by `gang pr open`.\n"
		if err != nil || got != want {
			t.Errorf("Body = %q, %v, want %q", got, err, want)
		}
		if _, err := Body(t.TempDir(), p, prep); err == nil {
			t.Error("Body outside a repo returned no error")
		}
	}
}

func TestPublishOpensTheNextTask(t *testing.T) {
	repo, d, p := queueFixture(t)
	shipOnOrigin(t, repo, "chore: move the base", map[string]string{"x.txt": "x\n"}) // the replay differs from the task tip
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "pr-create", "https://github.com/acme/widget/pull/17\n")
	now := time.Date(2026, 10, 29, 5, 0, 0, 0, time.FixedZone("PDT", -7*3600)) // 12:00:00Z
	if url, err := Publish(repo, d, p, "T1", now, false); err != nil || url != "https://github.com/acme/widget/pull/17" {
		t.Fatalf("Publish = %q, %v", url, err)
	}
	tip := testkit.Git(t, testkit.Git(t, repo, "remote", "get-url", "origin"), "rev-parse", "refs/heads/gang/demo/T1")
	rows, err := Read(d)
	if want := (Row{Task: "T1", PR: 17, Branch: "gang/demo/T1", HeadSHA: tip, State: StateOpen, OpenedAt: "2026-10-29T12:00:00Z"}); err != nil || len(rows) != 1 || rows[0] != want {
		t.Fatalf("rows = %+v, %v, want %+v", rows, err, want)
	}
	call := "pr create --repo github.com/acme/widget --base main --head gang/demo/T1 --title T1: work --body-file " + d.Run("pr-body-T1.md")
	if calls := testkit.Calls(t, state); len(calls) != 1 || calls[0] != call {
		t.Fatalf("gh calls = %q, want one %q", calls, call)
	}
	body, err := os.ReadFile(state + "/body.md")
	if err != nil || !strings.HasPrefix(string(body), "Task T1 of gang run `demo`") || !strings.HasSuffix(string(body), "`gang pr open`.\n") {
		t.Fatalf("body = %q, %v", body, err)
	}
	if _, err := os.Stat(d.Run("pr-body-T1.md")); !os.IsNotExist(err) {
		t.Fatalf("the body file is left behind: %v", err)
	}
}

func TestPublishDryRunPushesAndRecordsNothing(t *testing.T) {
	repo, d, p := queueFixture(t)
	p.Repo.GHAccount = "" // a dry run needs no account
	state, _ := testkit.FakeGH(t)
	got, err := Publish(repo, d, p, "T1", time.Time{}, true)
	if err != nil || !strings.HasPrefix(got, "git -C "+repo+" push origin ") || !strings.Contains(got, ":refs/heads/gang/demo/T1\n\nT1: work\n\nTask T1 of gang run") {
		t.Fatalf("Publish = %q, %v", got, err)
	}
	if _, err := os.Stat(d.Run("prs.tsv")); !os.IsNotExist(err) || len(testkit.Calls(t, state)) != 0 || testkit.Git(t, repo, "ls-remote", "origin", "refs/heads/gang/*") != "" {
		t.Fatal("a dry run wrote prs.tsv, called gh, or pushed")
	}
	_, err = Publish(repo, d, p, "T2", time.Time{}, true)
	wantExit(t, err, exit.Refused, "the next task to ship is T1, not T2")
}

// Contract 0.9: with no GitHub account only a dry run is allowed, and the check comes before any git call (this dir is no repo).
func TestPublishWithoutGHAccountNeedsDryRun(t *testing.T) {
	_, err := Publish(t.TempDir(), store.Dir{Root: t.TempDir()}, &plan.Plan{}, "T1", time.Time{}, false)
	wantExit(t, err, exit.Refused, "works only with --dry-run")
}
