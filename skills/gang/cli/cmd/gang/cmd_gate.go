package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gang/internal/exit"
	"gang/internal/gate"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/prq"
	"gang/internal/repair"
	"gang/internal/store"
)

func init() {
	commands["gate"] = subcommands("gate", map[string]func(c *Ctx, args []string) error{
		"run":   gateRun,
		"wait":  gateWait,
		"_exec": gateExec,
	})
}

const gateUsage = "usage: gang gate run <id> | run --health | run --wave | run --pr <id> | wait <job> [--timeout 540s]"

// gateExec is the detached runner that gate.Start spawns as
// `gang --dir <dir> gate _exec <id>`.
func gateExec(c *Ctx, args []string) error {
	if len(args) != 1 {
		return exit.New(exit.Usage, gateUsage)
	}
	return gate.Exec(store.Dir{Root: c.Dir}, args[0])
}

func gateRun(c *Ctx, args []string) error {
	fs := newFlagSet("gate run")
	health := fs.Bool("health", false, "run the health commands on the integration tip in this worktree")
	wave := fs.Bool("wave", false, "run the wave gate and health commands in run/wt/wave")
	pr := fs.String("pr", "", "run the gate commands at the head of this task's open PR, in run/wt/pr")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	modes := len(pos)
	for _, on := range []bool{*health, *wave, *pr != ""} {
		if on {
			modes++
		}
	}
	if len(pos) > 1 || modes != 1 {
		return exit.New(exit.Usage, gateUsage)
	}
	// A task gate needs the task worktree. A health gate uses this worktree
	// when it is one, else the gang-owned run/wt/health. A wave gate always
	// uses run/wt/wave.
	var top string
	if !*wave && *pr == "" { // a PR gate uses run/wt/pr
		if top, err = worktreeTop(c); err != nil && !(*health && isNoWorktree(err)) {
			return err
		}
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	j := gate.Job{Env: p.Env, TimeoutMin: p.Limits.GateTimeoutMin, Slots: p.Limits.GateSlots, Locks: p.Locks}
	switch {
	case *wave:
		tip, err := integrationTip(c, p)
		if err != nil {
			return err
		}
		if j.Cwd, err = gangWorktree(c, d, "wave", tip); err != nil {
			return err
		}
		j.Kind = gate.KindWave
		j.Commands = append(append([]string{}, p.Commands.WaveGate...), p.Commands.Health...)
	case *health:
		tip, err := integrationTip(c, p)
		if err != nil {
			return err
		}
		if top == "" {
			if top, err = gangWorktree(c, d, "health", tip); err != nil {
				return err
			}
		} else if _, err := gitx.Run(top, "switch", "--detach", tip); err != nil {
			return err
		}
		j.Kind, j.Cwd, j.Commands = gate.KindHealth, top, p.Commands.Health
	case *pr != "":
		rows, err := prq.Read(d)
		if err != nil {
			return err
		}
		row, ok := prq.Current(rows)
		if !ok || row.Task != *pr {
			return exit.New(exit.Refused, "%s has no open PR", *pr)
		}
		// All PR jobs share run/wt/pr, so a start must not move it under a live job.
		jobs, err := os.ReadDir(d.Run("jobs"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		for _, e := range jobs {
			if strings.HasPrefix(e.Name(), "pr-") {
				st, err := gate.Wait(d, e.Name(), 0)
				if err != nil {
					return err
				}
				if st.State == gate.StateQueued || st.State == gate.StateRunning {
					return exit.New(exit.Refused, "gate job %s is still %s: wait for it first", e.Name(), st.State)
				}
			}
		}
		if j.Cwd, err = gangWorktree(c, d, "pr", row.HeadSHA); err != nil {
			return err
		}
		j.Kind, j.Task, j.Head, j.Commands = gate.KindPR, row.Task, row.HeadSHA, p.Commands.Gate
	default:
		t, ok := repair.TaskFor(p, pos[0])
		if !ok {
			return exit.New(exit.Refused, "unknown task %s", pos[0])
		}
		j.Kind, j.Task, j.Cwd, j.Locks = gate.KindTask, t.ID, top, t.Locks
		j.Commands = append(append([]string{}, repair.GateFor(p, t)...), t.Verify.Commands...)
	}
	if len(j.Commands) == 0 {
		return exit.New(exit.User, "the plan has no commands for this gate")
	}
	exe, err := executable() // the running gang binary spawns the runner
	if err != nil {
		return err
	}
	id, err := gate.Start(d, exe, j)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, id)
	return nil
}

func integrationTip(c *Ctx, p *plan.Plan) (string, error) {
	tip, err := gitx.RevParse(c.Repo, "refs/heads/"+p.Repo.Integration)
	return strings.TrimSpace(tip), err
}

// isNoWorktree reports whether err is worktreeTop's NO-WORKTREE refusal.
func isNoWorktree(err error) bool {
	var e *exit.Error
	return errors.As(err, &e) && e.Code == exit.Refused && strings.HasPrefix(e.Msg, "NO-WORKTREE")
}

// gangWorktree puts the gang-owned worktree run/wt/<name> at tip, creating it
// when absent. It discards local edits and untracked files but keeps ignored
// ones, such as build caches. An existing path must be the top of its own
// worktree: a plain directory would let git find the main checkout and reset it.
func gangWorktree(c *Ctx, d store.Dir, name, tip string) (string, error) {
	wt := d.Run("wt", name)
	if _, err := os.Stat(wt); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(wt), 0o755); err != nil {
			return "", err
		}
		_, err := gitx.Run(c.Repo, "worktree", "add", "--detach", wt, tip)
		return wt, err
	} else if err != nil {
		return "", err
	}
	top, err := gitx.Run(wt, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	realTop, err1 := filepath.EvalSymlinks(strings.TrimSpace(top))
	realWT, err2 := filepath.EvalSymlinks(wt)
	if err := errors.Join(err1, err2); err != nil {
		return "", err
	}
	if realTop != realWT {
		return "", exit.New(exit.User, "%s exists but is not a git worktree. Remove it and run again.", wt)
	}
	if _, err := gitx.Run(wt, "switch", "--detach", "--discard-changes", tip); err != nil {
		return "", err
	}
	_, err = gitx.Run(wt, "clean", "-fd")
	return wt, err
}

// gateWait prints the last 40 log lines. Exit 0 passed, 1 failed or timeout,
// 75 still queued or running, 2 unknown job or state error (the gate could
// not run, for example because its runner died).
func gateWait(c *Ctx, args []string) error {
	fs := newFlagSet("gate wait")
	timeout := fs.Duration("timeout", 540*time.Second, "how long to wait")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return exit.New(exit.Usage, gateUsage)
	}
	d := store.Dir{Root: c.Dir}
	st, err := gate.Wait(d, pos[0], *timeout)
	if errors.Is(err, os.ErrNotExist) {
		return exit.New(exit.Refused, "unknown job %s", pos[0])
	}
	if err != nil {
		return err
	}
	tail, err := logTail(d.Run("jobs", pos[0], "out.log"), 40)
	if err != nil {
		return err
	}
	fmt.Fprint(c.Stdout, tail)
	switch st.State {
	case gate.StatePassed:
		return nil
	case gate.StateQueued, gate.StateRunning:
		return exit.New(exit.Pending, "gate %s is still %s", st.ID, st.State)
	case gate.StateError:
		return exit.New(exit.Refused, "gate %s could not run: %s", st.ID, strings.Join(st.Failed, "; "))
	}
	return exit.New(exit.User, "gate %s %s: %s", st.ID, st.State, strings.Join(st.Failed, "; "))
}

func logTail(path string, n int) (string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	if len(lines) == 1 && lines[0] == "" {
		return "", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}
