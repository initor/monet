// Package gate runs gate commands as detached background jobs. A job takes a
// slot lease, then its lock leases, runs its commands in order with sh -c,
// and records the outcome in run/jobs/<id>/status.json.
package gate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gang/internal/exit"
	"gang/internal/lock"
	"gang/internal/store"
)

// Job is run/jobs/<id>/job.json.
type Job struct {
	ID         string            `json:"id"`
	Task       string            `json:"task"`
	Kind       string            `json:"kind"`
	Commands   []string          `json:"commands"`
	Cwd        string            `json:"cwd"`
	Locks      []string          `json:"locks"`
	Env        map[string]string `json:"env"`
	TimeoutMin int               `json:"timeout_min"`
	Slots      int               `json:"slots"`
	Head       string            `json:"head,omitempty"` // a PR job: the PR head that its worktree holds
}

// JobStatus is run/jobs/<id>/status.json (Contract 4, plus the runner pid
// from Contract 0 rule 12). PID is 0 until the runner starts.
type JobStatus struct {
	ID       string   `json:"id"`
	State    string   `json:"state"`
	ExitCode int      `json:"exit_code"`
	Started  string   `json:"started"`
	Ended    string   `json:"ended"`
	Failed   []string `json:"failed"`
	PID      int      `json:"pid"`
}

const (
	KindTask   = "task"
	KindHealth = "health"
	KindWave   = "wave"
	KindPR     = "pr"

	StateQueued  = "queued"
	StateRunning = "running"
	StatePassed  = "passed"
	StateFailed  = "failed"
	StateTimeout = "timeout"
	StateError   = "error"
)

// The test runner shortens these.
var (
	minute     = time.Minute            // the unit of Job.TimeoutMin
	retryEvery = 5 * time.Second        // wait between slot or lock attempts
	renewEvery = time.Minute            // lease renewal cadence
	leaseTTL   = 5 * time.Minute        // lease length, renewed every renewEvery
	pollEvery  = 200 * time.Millisecond // Wait's status poll
)

func validID(id string) bool {
	return id != "" && id != "." && id != ".." && id == filepath.Base(id)
}

func prefix(j Job) (string, error) {
	switch j.Kind {
	case KindTask:
		if !validID(j.Task) {
			return "", fmt.Errorf("gate: bad task id %q", j.Task)
		}
		return j.Task, nil
	case KindPR: // the task's prefix, so a PR job never shares a number with a task job
		pre, err := prefix(Job{Kind: KindTask, Task: j.Task})
		return "pr-" + pre, err
	case KindHealth, KindWave:
		return j.Kind, nil
	}
	return "", fmt.Errorf("gate: unknown kind %q", j.Kind)
}

// nextID claims the next free <prefix>-<n> directory under jobs. Mkdir is
// atomic, so two callers never get the same id.
func nextID(jobs, pre string) (string, error) {
	entries, err := os.ReadDir(jobs)
	if err != nil {
		return "", err
	}
	n := 0
	for _, e := range entries {
		if rest, ok := strings.CutPrefix(e.Name(), pre+"-"); ok {
			if k, err := strconv.Atoi(rest); err == nil && k > n {
				n = k
			}
		}
	}
	for {
		n++
		id := fmt.Sprintf("%s-%d", pre, n)
		err := os.Mkdir(filepath.Join(jobs, id), 0o755)
		if err == nil {
			return id, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return store.WriteFileAtomic(path, append(b, '\n'), 0o644)
}

func writeStatus(d store.Dir, st JobStatus) error {
	if st.Failed == nil {
		st.Failed = []string{}
	}
	return writeJSON(d.Run("jobs", st.ID, "status.json"), st)
}

func readStatus(d store.Dir, id string) (JobStatus, error) {
	if !validID(id) {
		return JobStatus{}, fmt.Errorf("gate: bad job id %q", id)
	}
	b, err := os.ReadFile(d.Run("jobs", id, "status.json"))
	if err != nil {
		return JobStatus{}, err
	}
	var st JobStatus
	return st, json.Unmarshal(b, &st)
}

func stamp() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// Start writes job.json and a queued status.json, then spawns
// `exe --dir <root> gate _exec <id>` in its own session with output to
// out.log, and returns the job id without waiting.
func Start(d store.Dir, exe string, j Job) (string, error) {
	if len(j.Commands) == 0 {
		return "", errors.New("gate: no commands to run")
	}
	if !filepath.IsAbs(j.Cwd) {
		return "", fmt.Errorf("gate: cwd %q is not absolute", j.Cwd)
	}
	pre, err := prefix(j)
	if err != nil {
		return "", err
	}
	jobs := d.Run("jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		return "", err
	}
	if j.ID, err = nextID(jobs, pre); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(jobs, j.ID, "job.json"), j); err != nil {
		return "", err
	}
	if err := writeStatus(d, JobStatus{ID: j.ID, State: StateQueued}); err != nil {
		return "", err
	}
	log, err := os.OpenFile(filepath.Join(jobs, j.ID, "out.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", err
	}
	defer log.Close()
	cmd := exec.Command(exe, "--dir", d.Root, "gate", "_exec", j.ID)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = writeStatus(d, JobStatus{ID: j.ID, State: StateError, ExitCode: -1, Ended: stamp(), Failed: []string{err.Error()}})
		return "", err
	}
	go func() { _ = cmd.Wait() }() // reaps the runner when this process outlives it
	return j.ID, nil
}

// leases tracks what one job holds and renews it in the background.
type leases struct {
	dir, holder string
	mu          sync.Mutex
	held        []string
	lost        chan string
}

func (l *leases) try(name string) (bool, error) {
	_, err := lock.AcquirePID(l.dir, name, l.holder, os.Getpid(), leaseTTL, time.Now())
	if errors.Is(err, lock.ErrHeld) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	l.mu.Lock()
	l.held = append(l.held, name)
	l.mu.Unlock()
	return true, nil
}

func (l *leases) renewLoop(stop <-chan struct{}) {
	tick := time.NewTicker(renewEvery)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			l.mu.Lock()
			names := slices.Clone(l.held)
			l.mu.Unlock()
			for _, n := range names {
				if err := lock.Renew(l.dir, n, l.holder, leaseTTL, time.Now()); err != nil {
					select {
					case l.lost <- n:
					default:
					}
				}
			}
		}
	}
}

func (l *leases) releaseAll() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, n := range l.held {
		_ = lock.Release(l.dir, n, l.holder)
	}
	l.held = nil
}

// Exec runs job id to a terminal state. It is the body of `gang gate _exec`.
// It returns an error only when it cannot record a status.
func Exec(d store.Dir, id string) error {
	if !validID(id) {
		return fmt.Errorf("gate: bad job id %q", id)
	}
	b, err := os.ReadFile(d.Run("jobs", id, "job.json"))
	if err != nil {
		return err
	}
	var j Job
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	log, err := os.OpenFile(d.Run("jobs", id, "out.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()

	st := JobStatus{ID: id, State: StateQueued, PID: os.Getpid()}
	if err := writeStatus(d, st); err != nil {
		return err
	}
	finish := func(state string, code int, failed ...string) error {
		st.State, st.ExitCode, st.Ended, st.Failed = state, code, stamp(), failed
		fmt.Fprintf(log, "gang: job %s %s\n", id, state)
		return writeStatus(d, st)
	}

	l := &leases{dir: d.Run("locks"), holder: id, lost: make(chan string, 1)}
	stop := make(chan struct{})
	go l.renewLoop(stop)
	defer func() {
		close(stop)
		l.releaseAll()
	}()

	// waitFor retries take every retryEvery until it succeeds.
	waitFor := func(what string, take func() (bool, error)) error {
		for said := false; ; said = true {
			ok, err := take()
			if err != nil || ok {
				return err
			}
			if !said {
				fmt.Fprintf(log, "gang: waiting for %s\n", what)
			}
			select {
			case name := <-l.lost:
				return fmt.Errorf("lease lost: %s", name)
			case <-time.After(retryEvery):
			}
		}
	}
	slots := max(j.Slots, 1)
	err = waitFor("a gate slot", func() (bool, error) {
		for k := 1; k <= slots; k++ {
			if ok, err := l.try(fmt.Sprintf("slot-%d", k)); ok || err != nil {
				return ok, err
			}
		}
		return false, nil
	})
	names := slices.Clone(j.Locks)
	sort.Strings(names) // one global order, so two jobs cannot deadlock
	for _, name := range names {
		if err != nil {
			break
		}
		err = waitFor("lock "+name, func() (bool, error) { return l.try(name) })
	}
	if err != nil {
		return finish(StateError, -1, err.Error())
	}

	st.State, st.Started = StateRunning, stamp()
	if err := writeStatus(d, st); err != nil {
		return err
	}
	timeout := j.TimeoutMin
	if timeout <= 0 {
		timeout = 60
	}
	deadline := time.Now().Add(time.Duration(timeout) * minute)
	env := buildEnv(j)
	for _, line := range j.Commands {
		fmt.Fprintf(log, "$ %s\n", line)
		code, state, err := runOne(line, j.Cwd, env, log, deadline, l.lost)
		if err != nil {
			fmt.Fprintf(log, "gang: %v\n", err)
		}
		if state != StatePassed {
			return finish(state, code, line)
		}
	}
	return finish(StatePassed, 0)
}

// buildEnv is the runner's environment plus the job env and PWD. A value
// that starts with ./ or ../ is resolved against the job cwd.
func buildEnv(j Job) []string {
	env := os.Environ()
	keys := make([]string, 0, len(j.Env))
	for k := range j.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := j.Env[k]
		if strings.HasPrefix(v, "./") || strings.HasPrefix(v, "../") {
			v = filepath.Join(j.Cwd, v)
		}
		env = append(env, k+"="+v)
	}
	return append(env, "PWD="+j.Cwd)
}

// runOne runs one command in its own process group. On timeout or a lost
// lease it kills the whole group. It also kills leftovers after a normal exit.
func runOne(line, cwd string, env []string, log *os.File, deadline time.Time, lost <-chan string) (int, string, error) {
	cmd := exec.Command("sh", "-c", line)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = cwd, env, log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return -1, StateError, err
	}
	killGroup := func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case err := <-done:
		killGroup()
		if err == nil {
			return 0, StatePassed, nil
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode(), StateFailed, nil
		}
		return -1, StateError, err
	case <-timer.C:
		killGroup()
		<-done
		return -1, StateTimeout, nil
	case name := <-lost:
		killGroup()
		<-done
		return -1, StateError, fmt.Errorf("lease lost: %s", name)
	}
}

// Wait polls status.json until the job is terminal or timeout passes, then
// returns the last status read. A zero timeout reads it once. A queued or
// running job whose runner pid is dead becomes error (Contract 0 rule 12).
func Wait(d store.Dir, id string, timeout time.Duration) (JobStatus, error) {
	deadline := time.Now().Add(timeout)
	for {
		st, err := readStatus(d, id)
		if err != nil {
			return JobStatus{}, err
		}
		if !terminal(st.State) && st.PID > 0 && errors.Is(syscall.Kill(st.PID, 0), syscall.ESRCH) {
			return markDead(d, id)
		}
		left := time.Until(deadline)
		if terminal(st.State) || left <= 0 {
			return st, nil
		}
		time.Sleep(min(pollEvery, left))
	}
}

// markDead records error for a job whose runner died. It reads the status
// again first, because a runner writes its terminal status just before it
// exits.
func markDead(d store.Dir, id string) (JobStatus, error) {
	st, err := readStatus(d, id)
	if err != nil || terminal(st.State) {
		return st, err
	}
	st.State, st.ExitCode, st.Ended = StateError, -1, stamp()
	st.Failed = []string{fmt.Sprintf("runner pid %d died", st.PID)}
	return st, writeStatus(d, st)
}

// terminal reports whether a job in state s has ended.
func terminal(s string) bool {
	return s == StatePassed || s == StateFailed || s == StateTimeout || s == StateError
}

// PRStatus returns the commit status that PR gate job id gives to head, the
// open PR head of task: success for a passed job, failure for a failed or
// timed-out one.
func PRStatus(d store.Dir, id, task, head string) (string, error) {
	unknown := exit.New(exit.Refused, "unknown job %s", id)
	if !validID(id) {
		return "", unknown
	}
	var j Job
	if b, err := os.ReadFile(d.Run("jobs", id, "job.json")); err != nil || json.Unmarshal(b, &j) != nil {
		return "", unknown
	}
	if head == "" || j.Kind != KindPR || j.Task != task || j.Head != head {
		return "", exit.New(exit.Refused, "job %s did not gate head %s of %s", id, head[:min(len(head), 7)], task)
	}
	st, err := Wait(d, id, 0)
	// A job with no status.json never ran: Start stopped after it wrote job.json.
	if errors.Is(err, os.ErrNotExist) {
		st, err = JobStatus{State: StateError}, nil
	}
	if err != nil {
		return "", err
	}
	switch st.State {
	case StatePassed:
		return "success", nil
	case StateFailed, StateTimeout:
		return "failure", nil
	case StateError:
		return "", exit.New(exit.Refused, "gate job %s could not run: run the gate again", id)
	}
	return "", exit.New(exit.Pending, "gate job %s is still %s: post after it ends", id, st.State)
}
