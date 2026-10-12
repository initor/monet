package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"gang/internal/gate"
	"gang/internal/plan"
	"gang/internal/store"
)

// gatePkgDir is this package's directory, read before any test changes the cwd.
var gatePkgDir, _ = os.Getwd()

// useBuiltGang builds the gang binary and points executable at it, so gate
// jobs run the real `gang --dir <dir> gate _exec <id>`.
func useBuiltGang(t *testing.T) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gang")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = gatePkgDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	old := executable
	executable = func() (string, error) { return bin, nil }
	t.Cleanup(func() { executable = old })
}

func gatePlan() *plan.Plan {
	a1, b2 := rsTask("A1"), rsTask("B2")
	a1.Verify.Commands = []string{"echo verify-ran"}
	b2.Verify.Commands = []string{"false"}
	p := rsPlan(a1, b2)
	p.Commands = plan.Commands{
		Gate:     []string{"echo gate-ran"},
		Health:   []string{"echo health-at $(git rev-parse HEAD)"},
		WaveGate: []string{"echo wave-gate-ran"},
	}
	return p
}

func TestGateRunTaskAndWait(t *testing.T) {
	useBuiltGang(t)
	repo, dir := rsSetup(t, gatePlan())
	c := rsCtx(dir, repo)
	t.Chdir(rsWorktree(t, repo, "w1"))

	out, stderr, code := runGang(t, c, "gate", "run", "A1")
	if code != 0 || out != "A1-1\n" {
		t.Fatalf("gate run A1: code=%d out=%q stderr=%q", code, out, stderr)
	}
	out, _, code = runGang(t, c, "gate", "wait", "A1-1", "--timeout", "60s")
	gi, vi := strings.Index(out, "gate-ran"), strings.Index(out, "verify-ran")
	if code != 0 || gi < 0 || vi < gi {
		t.Fatalf("wait A1-1: code=%d out=%q, want gate then verify output", code, out)
	}

	if out, _, _ = runGang(t, c, "gate", "run", "B2"); out != "B2-1\n" {
		t.Fatalf("gate run B2: %q", out)
	}
	if _, stderr, code = runGang(t, c, "gate", "wait", "B2-1", "--timeout", "60s"); code != 1 {
		t.Fatalf("wait B2-1: code=%d stderr=%q, want 1", code, stderr)
	}
	if _, _, code = runGang(t, c, "gate", "wait", "Z9-1"); code != 2 {
		t.Fatalf("wait unknown job: code=%d, want 2", code)
	}

	// Contract 0 rule 16: a repair id's gate is plan.commands.health.
	if out, _, _ = runGang(t, c, "gate", "run", "F01"); out != "F01-1\n" {
		t.Fatalf("gate run F01: %q", out)
	}
	out, _, code = runGang(t, c, "gate", "wait", "F01-1", "--timeout", "60s")
	if code != 0 || !strings.Contains(out, "health-at ") || strings.Contains(out, "gate-ran") {
		t.Fatalf("wait F01-1: code=%d out=%q, want health output only", code, out)
	}
}

func TestGateWaitStillRunning(t *testing.T) {
	useBuiltGang(t)
	p := gatePlan()
	p.Commands.Gate = []string{"sleep 2"}
	repo, dir := rsSetup(t, p)
	c := rsCtx(dir, repo)
	t.Chdir(rsWorktree(t, repo, "w1"))
	if _, _, code := runGang(t, c, "gate", "run", "A1"); code != 0 {
		t.Fatal("gate run failed")
	}
	if _, stderr, code := runGang(t, c, "gate", "wait", "A1-1", "--timeout", "100ms"); code != 75 {
		t.Fatalf("early wait: code=%d stderr=%q, want 75", code, stderr)
	}
	if _, _, code := runGang(t, c, "gate", "wait", "A1-1", "--timeout", "60s"); code != 0 {
		t.Fatalf("late wait: code=%d, want 0", code)
	}
}

// Contract 0 rule 12: gate wait reports state error when the runner pid in
// status.json is dead. That gate could not run, so it exits 2, not the 1 of
// a red gate (TestGateRunTaskAndWait pins 1 for a failing command).
func TestGateWaitDeadRunner(t *testing.T) {
	useBuiltGang(t)
	p := gatePlan()
	p.Commands.Gate = []string{"echo $$ > cmd.pid; exec sleep 30"}
	repo, dir := rsSetup(t, p)
	c := rsCtx(dir, repo)
	wt := rsWorktree(t, repo, "w1")
	t.Chdir(wt)
	t.Cleanup(func() {
		if b, err := os.ReadFile(filepath.Join(wt, "cmd.pid")); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
	})
	if _, _, code := runGang(t, c, "gate", "run", "A1"); code != 0 {
		t.Fatal("gate run failed")
	}
	d := store.Dir{Root: dir}
	var st gate.JobStatus
	for range 100 {
		if st, _ = gate.Wait(d, "A1-1", 0); st.State == gate.StateRunning {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if st.State != gate.StateRunning || st.PID <= 0 {
		t.Fatalf("status %+v, want running with a pid", st)
	}
	if err := syscall.Kill(st.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runGang(t, c, "gate", "wait", "A1-1", "--timeout", "10s"); code != 2 || !strings.Contains(stderr, "could not run") {
		t.Fatalf("wait after SIGKILL: code=%d stderr=%q, want 2 and a could not run message", code, stderr)
	}
}

func TestGateRunHealthDetachesAtIntegrationTip(t *testing.T) {
	useBuiltGang(t)
	repo, dir := rsSetup(t, gatePlan())
	c := rsCtx(dir, repo)
	wt := rsWorktree(t, repo, "w1")
	t.Chdir(wt)
	if _, _, code := runGang(t, c, "claim", "A1"); code != 0 {
		t.Fatal("claim failed")
	}
	tip := rsEmptyCommit(t, repo, "refs/heads/integration", "merge X1: moved")

	out, stderr, code := runGang(t, c, "gate", "run", "--health")
	if code != 0 || out != "health-1\n" {
		t.Fatalf("gate run --health: code=%d out=%q stderr=%q", code, out, stderr)
	}
	if got := gitT(t, wt, "rev-parse", "HEAD"); got != tip {
		t.Fatalf("worktree HEAD %s, want integration tip %s", got, tip)
	}
	if got := gitT(t, wt, "branch", "--show-current"); got != "" {
		t.Fatalf("worktree still on %q", got)
	}
	out, _, code = runGang(t, c, "gate", "wait", "health-1", "--timeout", "60s")
	if code != 0 || !strings.Contains(out, "health-at "+tip) {
		t.Fatalf("wait health-1: code=%d out=%q", code, out)
	}
}

// From the main checkout, --health runs in the gang-owned run/wt/health at
// the integration tip, and the main checkout's HEAD never moves.
func TestGateRunHealthFromRepoRoot(t *testing.T) {
	useBuiltGang(t)
	repo, dir := rsSetup(t, gatePlan())
	c := rsCtx(dir, repo)
	wt := store.Dir{Root: dir}.Run("wt", "health")
	head := gitT(t, repo, "rev-parse", "HEAD")
	t.Chdir(repo)
	for i, id := range []string{"health-1", "health-2"} {
		tip := rsEmptyCommit(t, repo, "refs/heads/integration", "merge X1: moved")
		out, stderr, code := runGang(t, c, "gate", "run", "--health")
		if code != 0 || out != id+"\n" {
			t.Fatalf("run %d: code=%d out=%q stderr=%q", i, code, out, stderr)
		}
		if got := gitT(t, wt, "rev-parse", "HEAD"); got != tip {
			t.Fatalf("run %d: health worktree at %s, want %s", i, got, tip)
		}
		out, _, code = runGang(t, c, "gate", "wait", id, "--timeout", "60s")
		if code != 0 || !strings.Contains(out, "health-at "+tip) {
			t.Fatalf("run %d: code=%d out=%q", i, code, out)
		}
		if gitT(t, repo, "branch", "--show-current") != "main" || gitT(t, repo, "rev-parse", "HEAD") != head {
			t.Fatalf("run %d: gate run --health moved the main checkout", i)
		}
	}
}

// gang owns run/wt/health, so every run starts from a clean tree at the tip.
// Each job edits README.md, adds junk.txt, and bumps the ignored cache/runs.
// The trunk then changes README.md, which a plain switch refuses to overwrite.
func TestGateRunHealthResetsItsWorktree(t *testing.T) {
	useBuiltGang(t)
	p := gatePlan()
	p.Commands.Health = []string{
		`test -z "$(git status --porcelain)"`,
		"echo health-at $(git rev-parse HEAD) cache=$(cat cache/runs 2>/dev/null)",
		"echo local edit >> README.md && echo junk > junk.txt && mkdir -p cache && echo hit >> cache/runs",
	}
	repo, dir := rsSetup(t, p)
	info := filepath.Join(repo, ".git", "info")
	if err := os.MkdirAll(info, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(info, "exclude"), []byte("cache/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := rsCtx(dir, repo)
	t.Chdir(repo)
	for i, cache := range []string{"", "hit"} {
		id := "health-" + strconv.Itoa(i+1)
		tip := rsTaskCommit(t, repo, "integration", "X"+strconv.Itoa(i+1), "README.md", "trunk "+id+"\n")
		out, stderr, code := runGang(t, c, "gate", "run", "--health")
		if code != 0 || out != id+"\n" {
			t.Fatalf("run %d: code=%d out=%q stderr=%q", i, code, out, stderr)
		}
		out, _, code = runGang(t, c, "gate", "wait", id, "--timeout", "60s")
		if code != 0 || !strings.Contains(out, "health-at "+tip+" cache="+cache+"\n") {
			t.Fatalf("run %d: code=%d out=%q, want a clean tree at %s with cache=%q", i, code, out, tip, cache)
		}
	}
}

func TestGateRunWaveUsesItsOwnWorktree(t *testing.T) {
	useBuiltGang(t)
	repo, dir := rsSetup(t, gatePlan())
	c := rsCtx(dir, repo)
	wt := store.Dir{Root: dir}.Run("wt", "wave")
	t.Chdir(repo)

	tip := rsEmptyCommit(t, repo, "refs/heads/integration", "first")
	for i, id := range []string{"wave-1", "wave-2"} {
		if i == 1 {
			tip = rsEmptyCommit(t, repo, "refs/heads/integration", "second")
		}
		out, stderr, code := runGang(t, c, "gate", "run", "--wave")
		if code != 0 || out != id+"\n" {
			t.Fatalf("run %d: code=%d out=%q stderr=%q", i, code, out, stderr)
		}
		if got := gitT(t, wt, "rev-parse", "HEAD"); got != tip {
			t.Fatalf("run %d: wave worktree at %s, want %s", i, got, tip)
		}
		out, _, code = runGang(t, c, "gate", "wait", id, "--timeout", "60s")
		wi, hi := strings.Index(out, "wave-gate-ran"), strings.Index(out, "health-at "+tip)
		if code != 0 || wi < 0 || hi < wi {
			t.Fatalf("run %d: code=%d out=%q, want wave gate then health", i, code, out)
		}
	}
	if got := gitT(t, repo, "branch", "--show-current"); got != "main" {
		t.Fatalf("main checkout moved to %q", got)
	}
}

func TestGateRunWaveRefusesAPlainDirectory(t *testing.T) {
	useBuiltGang(t)
	repo, dir := rsSetup(t, gatePlan())
	c := rsCtx(dir, repo)
	if err := os.MkdirAll(store.Dir{Root: dir}.Run("wt", "wave"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := gitT(t, repo, "rev-parse", "HEAD")
	rsEmptyCommit(t, repo, "refs/heads/integration", "moved")
	if _, stderr, code := runGang(t, c, "gate", "run", "--wave"); code != 1 {
		t.Fatalf("code=%d stderr=%q, want 1", code, stderr)
	}
	if gitT(t, repo, "branch", "--show-current") != "main" || gitT(t, repo, "rev-parse", "HEAD") != head {
		t.Fatal("gate run --wave moved the main checkout")
	}
}

func TestGateRunRefusals(t *testing.T) {
	repo, dir := rsSetup(t, gatePlan())
	c := rsCtx(dir, repo)
	t.Chdir(repo)
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"gate", "run", "A1"}, 2},
		{[]string{"gate", "run"}, 64},
		{[]string{"gate", "run", "A1", "--health"}, 64},
		{[]string{"gate", "run", "--health", "--wave"}, 64},
		{[]string{"gate", "wait"}, 64},
		{[]string{"gate", "rerun"}, 64},
	} {
		_, stderr, code := runGang(t, c, tc.args...)
		if code != tc.code {
			t.Errorf("%v: code=%d stderr=%q, want %d", tc.args, code, stderr, tc.code)
		}
		if tc.code == 2 && !strings.Contains(stderr, "NO-WORKTREE:") {
			t.Errorf("%v: stderr %q", tc.args, stderr)
		}
	}
	t.Chdir(rsWorktree(t, repo, "w1"))
	if _, _, code := runGang(t, c, "gate", "run", "Z9"); code != 2 {
		t.Errorf("unknown task: code=%d, want 2", code)
	}
}
