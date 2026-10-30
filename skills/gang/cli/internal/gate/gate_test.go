package gate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"gang/internal/exit"
	"gang/internal/lock"
	"gang/internal/store"
)

// TestMain doubles as the detached runner: Start spawns this test binary
// with `--dir <root> gate _exec <id>`, the same argv the gang binary gets.
// The runner shortens its clock so a 1-minute timeout takes 1 second.
func TestMain(m *testing.M) {
	if len(os.Args) == 6 && os.Args[1] == "--dir" && os.Args[3] == "gate" && os.Args[4] == "_exec" {
		minute = time.Second
		retryEvery = 100 * time.Millisecond
		if err := Exec(store.Dir{Root: os.Args[2]}, os.Args[5]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func exe(t *testing.T) string {
	t.Helper()
	p, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func start(t *testing.T, d store.Dir, j Job) string {
	t.Helper()
	id, err := Start(d, exe(t), j)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func wait(t *testing.T, d store.Dir, id string) JobStatus {
	t.Helper()
	st, err := Wait(d, id, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func job(cwd string, cmds ...string) Job {
	return Job{Task: "A1", Kind: KindTask, Commands: cmds, Cwd: cwd, TimeoutMin: 5, Slots: 2}
}

// waitState polls until the job reaches state or 10 seconds pass.
func waitState(t *testing.T, d store.Dir, id, state string) {
	t.Helper()
	for range 100 {
		if st, _ := Wait(d, id, 0); st.State == state {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("%s never reached %s", id, state)
}

func TestJobIDsAndPass(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	cwd := t.TempDir()
	ids := []string{
		start(t, d, job(cwd, "true")),
		start(t, d, job(cwd, "true")),
		start(t, d, Job{Kind: KindHealth, Commands: []string{"true"}, Cwd: cwd}),
		start(t, d, Job{Kind: KindWave, Commands: []string{"true"}, Cwd: cwd}),
	}
	if want := []string{"A1-1", "A1-2", "health-1", "wave-1"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids %v, want %v", ids, want)
	}
	for _, id := range ids {
		if st := wait(t, d, id); st.State != StatePassed || st.ExitCode != 0 || len(st.Failed) != 0 || st.Started == "" || st.Ended == "" {
			t.Fatalf("%s: %+v", id, st)
		}
	}
	b, err := os.ReadFile(d.Run("jobs", "A1-1", "job.json"))
	if err != nil {
		t.Fatal(err)
	}
	var j Job
	if err := json.Unmarshal(b, &j); err != nil || j.ID != "A1-1" || j.Cwd != cwd || j.Commands[0] != "true" {
		t.Fatalf("job.json %s (%v)", b, err)
	}
}

// status.json keys are Contract 4 plus Contract 0 rule 12:
// {"id","state","exit_code","started","ended","failed","pid"}.
func TestStatusFileShape(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	id := start(t, d, job(t.TempDir(), "true"))
	wait(t, d, id)
	b, err := os.ReadFile(d.Run("jobs", id, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if want := []string{"ended", "exit_code", "failed", "id", "pid", "started", "state"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys %v, want %v", keys, want)
	}
	if pid, ok := raw["pid"].(float64); !ok || pid <= 0 {
		t.Fatalf("pid %v, want the runner's pid", raw["pid"])
	}
}

func TestStopsAtFirstFailingCommand(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	cwd := t.TempDir()
	id := start(t, d, job(cwd, "echo one", "exit 3", "touch after"))
	st := wait(t, d, id)
	if st.State != StateFailed || st.ExitCode != 3 || !reflect.DeepEqual(st.Failed, []string{"exit 3"}) {
		t.Fatalf("%+v", st)
	}
	if _, err := os.Stat(filepath.Join(cwd, "after")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a command after the failure ran")
	}
	log, _ := os.ReadFile(d.Run("jobs", id, "out.log"))
	if !strings.Contains(string(log), "$ echo one\none\n$ exit 3\n") {
		t.Fatalf("out.log %q", log)
	}
}

func TestEnvIsPinnedAndRelativeValuesResolve(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	cwd := t.TempDir()
	j := job(cwd, `printf '%s\n%s\n%s\n' "$FOO" "$KC" "$PWD" > env.out`)
	j.Env = map[string]string{"FOO": "bar", "KC": "./k/c"}
	if st := wait(t, d, start(t, d, j)); st.State != StatePassed {
		t.Fatalf("%+v", st)
	}
	b, err := os.ReadFile(filepath.Join(cwd, "env.out"))
	if err != nil {
		t.Fatal(err)
	}
	want := "bar\n" + filepath.Join(cwd, "k", "c") + "\n" + cwd + "\n"
	if string(b) != want {
		t.Fatalf("env.out %q, want %q", b, want)
	}
}

func TestTimeoutKillsTheProcessGroup(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	cwd := t.TempDir()
	j := job(cwd, "sleep 30 & echo $! > child.pid; wait")
	j.TimeoutMin = 1 // one second in the test runner
	began := time.Now()
	st := wait(t, d, start(t, d, j))
	if st.State != StateTimeout || time.Since(began) > 15*time.Second {
		t.Fatalf("%+v after %s", st, time.Since(began))
	}
	b, err := os.ReadFile(filepath.Join(cwd, "child.pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	for range 50 {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("background child %d outlived the timeout", pid)
}

func TestSlotContention(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	cwd := t.TempDir()
	first := job(cwd, "sleep 2")
	first.Slots = 1
	second := job(cwd, "true")
	second.Slots = 1
	id1 := start(t, d, first)
	waitState(t, d, id1, StateRunning)
	id2 := start(t, d, second)
	time.Sleep(500 * time.Millisecond)
	if st, _ := Wait(d, id2, 0); st.State != StateQueued {
		t.Fatalf("second job is %s while the only slot is busy", st.State)
	}
	st2 := wait(t, d, id2)
	st1 := wait(t, d, id1)
	if st1.State != StatePassed || st2.State != StatePassed {
		t.Fatalf("%+v %+v", st1, st2)
	}
	ended1, _ := time.Parse(time.RFC3339Nano, st1.Ended)
	started2, _ := time.Parse(time.RFC3339Nano, st2.Started)
	if started2.Before(ended1) {
		t.Fatalf("second job started %s, before the first ended %s", st2.Started, st1.Ended)
	}
}

func TestWaitsForLocksAndReleasesLeases(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	locks := d.Run("locks")
	if _, err := lock.Acquire(locks, "gpu", "someone", time.Hour, time.Now()); err != nil {
		t.Fatal(err)
	}
	j := job(t.TempDir(), "true")
	j.Locks = []string{"gpu"}
	id := start(t, d, j)
	time.Sleep(500 * time.Millisecond)
	if st, _ := Wait(d, id, 0); st.State != StateQueued {
		t.Fatalf("job is %s while gpu is held", st.State)
	}
	if err := lock.Release(locks, "gpu", "someone"); err != nil {
		t.Fatal(err)
	}
	if st := wait(t, d, id); st.State != StatePassed {
		t.Fatalf("%+v", st)
	}
	left, _ := filepath.Glob(filepath.Join(locks, "*.json"))
	if len(left) != 0 {
		t.Fatalf("leases left after the job: %v", left)
	}
}

// killGroupFrom kills the process group whose leader wrote its pid to path.
func killGroupFrom(t *testing.T, path string) {
	t.Helper()
	t.Cleanup(func() {
		if b, err := os.ReadFile(path); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && pid > 0 {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
	})
}

// Contract 0 rule 12: a runner killed with SIGKILL never writes a terminal
// status, so Wait must report error instead of running forever.
func TestWaitReportsDeadRunner(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	cwd := t.TempDir()
	killGroupFrom(t, filepath.Join(cwd, "cmd.pid"))
	j := job(cwd, "echo $$ > cmd.pid; exec sleep 30")
	j.Slots = 1
	id := start(t, d, j)
	waitState(t, d, id, StateRunning)
	st, err := Wait(d, id, 0)
	if err != nil || st.PID <= 0 {
		t.Fatalf("running status %+v %v, want a runner pid", st, err)
	}
	if err := syscall.Kill(st.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	got, err := Wait(d, id, 10*time.Second)
	if err != nil || got.State != StateError || got.ExitCode != -1 || len(got.Failed) != 1 {
		t.Fatalf("after SIGKILL: %+v %v, want error", got, err)
	}
	b, _ := os.ReadFile(d.Run("jobs", id, "status.json"))
	var saved JobStatus
	if err := json.Unmarshal(b, &saved); err != nil || saved.State != StateError {
		t.Fatalf("status.json %s, want state error saved", b)
	}
	// The dead runner's slot lease is taken over, so the next job runs.
	next := job(cwd, "true")
	next.Slots = 1
	if st := wait(t, d, start(t, d, next)); st.State != StatePassed {
		t.Fatalf("next job %+v", st)
	}
}

func TestWaitReturnsEarlyAndRejectsUnknownJobs(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	id := start(t, d, job(t.TempDir(), "sleep 3"))
	// The job outlives the early Wait below. Wait for it to end before the
	// t.TempDir cleanups, which run after this one, remove the dirs it writes.
	t.Cleanup(func() { _, _ = Wait(d, id, 30*time.Second) })
	st, err := Wait(d, id, 200*time.Millisecond)
	if err != nil || (st.State != StateQueued && st.State != StateRunning) {
		t.Fatalf("%+v %v", st, err)
	}
	if _, err := Wait(d, "A1-99", 0); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unknown job: %v", err)
	}
	if _, err := Wait(d, "../x", 0); err == nil {
		t.Fatal("a path-like job id was accepted")
	}
}

func TestStartRejectsBadJobs(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	for name, j := range map[string]Job{
		"no commands":  {Task: "A1", Kind: KindTask, Cwd: t.TempDir()},
		"relative cwd": {Task: "A1", Kind: KindTask, Commands: []string{"true"}, Cwd: "wt"},
		"no task":      {Kind: KindTask, Commands: []string{"true"}, Cwd: t.TempDir()},
		"bad kind":     {Kind: "nightly", Commands: []string{"true"}, Cwd: t.TempDir()},
	} {
		if _, err := Start(d, exe(t), j); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Every job here is task T1 at head h1, and T1-1 is a task job.
func TestPRStatusOnlyForAFinishedJobAtTheHead(t *testing.T) {
	t.Parallel()
	d := store.Dir{Root: t.TempDir()}
	for _, w := range []struct{ id, kind, state string }{
		{"pr-T1-1", KindPR, StatePassed}, {"pr-T1-2", KindPR, StateFailed}, {"pr-T1-3", KindPR, StateTimeout},
		{"pr-T1-4", KindPR, StateRunning}, {"pr-T1-5", KindPR, StateError}, {"pr-T1-6", KindPR, StateQueued},
		{"T1-1", KindTask, StatePassed},
	} {
		j := Job{ID: w.id, Task: "T1", Kind: w.kind, Head: "h1"}
		if err := errors.Join(writeJSON(d.Run("jobs", w.id, "job.json"), j), writeStatus(d, JobStatus{ID: w.id, State: w.state})); err != nil {
			t.Fatal(err)
		}
	}
	// pr-T1-7: not JSON. pr-T1-8: no status.json. pr-T1-10: status.json unreadable. pr-T1-11: passed, no head.
	if err := errors.Join(
		store.WriteFileAtomic(d.Run("jobs", "pr-T1-7", "job.json"), []byte("{"), 0o644),
		writeJSON(d.Run("jobs", "pr-T1-8", "job.json"), Job{ID: "pr-T1-8", Task: "T1", Kind: KindPR, Head: "h1"}),
		writeJSON(d.Run("jobs", "pr-T1-10", "job.json"), Job{ID: "pr-T1-10", Task: "T1", Kind: KindPR, Head: "h1"}),
		os.MkdirAll(d.Run("jobs", "pr-T1-10", "status.json"), 0o755),
		writeJSON(d.Run("jobs", "pr-T1-11", "job.json"), Job{ID: "pr-T1-11", Task: "T1", Kind: KindPR}),
		writeStatus(d, JobStatus{ID: "pr-T1-11", State: StatePassed}),
	); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		id, task, head, want string
		err                  error
	}{
		{"pr-T1-1", "T1", "h1", "success", nil},
		{"pr-T1-2", "T1", "h1", "failure", nil},
		{"pr-T1-3", "T1", "h1", "failure", nil},
		{"pr-T1-4", "T1", "h1", "", exit.New(exit.Pending, "gate job pr-T1-4 is still running: post after it ends")},
		{"pr-T1-5", "T1", "h1", "", exit.New(exit.Refused, "gate job pr-T1-5 could not run: run the gate again")},
		{"pr-T1-6", "T1", "h1", "", exit.New(exit.Pending, "gate job pr-T1-6 is still queued: post after it ends")},
		{"pr-T1-8", "T1", "h1", "", exit.New(exit.Refused, "gate job pr-T1-8 could not run: run the gate again")},
		{"pr-T1-10", "T1", "h1", "", &os.PathError{Op: "read", Path: d.Run("jobs", "pr-T1-10", "status.json"), Err: syscall.EISDIR}},
		{"pr-T1-1", "T1", "h2", "", exit.New(exit.Refused, "job pr-T1-1 did not gate head h2 of T1")},
		{"pr-T1-1", "T1", "0123456789", "", exit.New(exit.Refused, "job pr-T1-1 did not gate head 0123456 of T1")},
		{"pr-T1-11", "T1", "", "", exit.New(exit.Refused, "job pr-T1-11 did not gate head  of T1")},
		{"pr-T1-1", "T2", "h1", "", exit.New(exit.Refused, "job pr-T1-1 did not gate head h1 of T2")},
		{"T1-1", "T1", "h1", "", exit.New(exit.Refused, "job T1-1 did not gate head h1 of T1")},
		{"pr-T1-9", "T1", "h1", "", exit.New(exit.Refused, "unknown job pr-T1-9")},
		{"pr-T1-7", "T1", "h1", "", exit.New(exit.Refused, "unknown job pr-T1-7")},
		{"../jobs/pr-T1-1", "T1", "h1", "", exit.New(exit.Refused, "unknown job ../jobs/pr-T1-1")},
	} {
		if got, err := PRStatus(d, c.id, c.task, c.head); got != c.want || !reflect.DeepEqual(err, c.err) {
			t.Errorf("PRStatus(%s, %s, %s) = %q, %#v, want %q, %#v", c.id, c.task, c.head, got, err, c.want, c.err)
		}
	}
}
