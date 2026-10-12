package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/schedule"
	"gang/internal/store"
)

func init() { commands["schedule"] = cmdSchedule }

// cmdSchedule prints "wave <k>: <ids>" per wave, then "chain: <ids joined by
// " > ">" (or "none") and "range <low>-<high> min" for the tasks not done on
// the integration branch. --write saves each task's wave into plan.json.
// --calibrate prints only, and a relative path is taken from the gang dir.
func cmdSchedule(c *Ctx, args []string) error {
	fs := newFlagSet("schedule")
	slots := fs.Int("cap", 0, "agents running at once (default run/cap, else CPUs minus 2)")
	waves := fs.Int("waves", 0, "number of waves (default ceil(tasks/15))")
	cal := fs.String("calibrate", "", "<task id>\\t<minutes> lines, such as run/durations.tsv")
	write := fs.Bool("write", false, "save the waves into plan.json")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	switch {
	case len(pos) > 0:
		return exit.New(exit.Usage, "schedule takes no arguments")
	case *slots < 0 || *waves < 0:
		return exit.New(exit.Usage, "schedule: --cap and --waves must not be negative")
	case *cal != "" && *write:
		return exit.New(exit.Usage, "schedule: --calibrate prints only, drop --write")
	}
	p, err := loadPlan(c)
	if err != nil {
		return err
	}
	if *slots == 0 {
		if *slots, err = agentCap(store.Dir{Root: c.Dir}); err != nil {
			return err
		}
	}
	var actual map[string]float64
	if *cal != "" {
		path := *cal
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.Dir, path)
		}
		if actual, err = readCalibrate(path); err != nil {
			return exit.New(exit.User, "%v", err)
		}
		var skipped []string
		for id := range actual {
			if _, ok := p.Task(id); !ok {
				skipped = append(skipped, id)
			}
		}
		if len(skipped) > 0 {
			sort.Strings(skipped)
			fmt.Fprintf(c.Stderr, "calibrate: skipped %s (not in the plan)\n", strings.Join(skipped, " "))
		}
	}
	ids, err := gitx.TaskIDs(c.Repo, "refs/heads/"+p.Repo.Integration)
	if err != nil {
		return err
	}
	done := map[string]bool{}
	for _, id := range ids {
		done[id] = true
	}
	r, err := schedule.Run(p, schedule.Options{Cap: *slots, Waves: *waves, Calibrate: actual, Done: done})
	if err != nil {
		return exit.New(exit.User, "schedule: %v", err)
	}
	byWave := map[int][]string{}
	for id, w := range r.Waves {
		byWave[w] = append(byWave[w], id)
	}
	nums := make([]int, 0, len(byWave))
	for w := range byWave {
		nums = append(nums, w)
	}
	sort.Ints(nums)
	var b strings.Builder
	for _, w := range nums {
		sort.Strings(byWave[w])
		fmt.Fprintf(&b, "wave %d: %s\n", w, strings.Join(byWave[w], " "))
	}
	chain := strings.Join(r.Chain, " > ")
	if chain == "" {
		chain = "none"
	}
	fmt.Fprintf(&b, "chain: %s\n", chain)
	fmt.Fprintf(&b, "range %d-%d min\n", int(math.Round(r.LowMin)), int(math.Round(r.HighMin)))
	fmt.Fprint(c.Stdout, b.String())
	if !*write {
		return nil
	}
	for i := range p.Tasks {
		p.Tasks[i].Wave = r.Waves[p.Tasks[i].ID]
	}
	path := store.Dir{Root: c.Dir}.Plan()
	if err := plan.Save(path, p); err != nil {
		return err
	}
	fmt.Fprintf(c.Stderr, "wrote waves to %s\n", path)
	return nil
}

// agentCap returns the measured agent cap: the positive integer in run/cap,
// or max(1, CPUs minus 2) when preflight has not written it yet.
func agentCap(d store.Dir) (int, error) {
	b, err := os.ReadFile(d.Run("cap"))
	if errors.Is(err, os.ErrNotExist) {
		return max(1, runtime.NumCPU()-2), nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n < 1 {
		return 0, exit.New(exit.User, "run/cap: want a positive integer, got %q", strings.TrimSpace(string(b)))
	}
	return n, nil
}

// readCalibrate reads lines of <task id>\t<minutes>. Blank lines and lines
// starting with # are skipped. The last line for an id wins.
func readCalibrate(path string) (map[string]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		id, mins, ok := strings.Cut(line, "\t")
		v, err := strconv.ParseFloat(strings.TrimSpace(mins), 64)
		if !ok || strings.TrimSpace(id) == "" || err != nil {
			return nil, fmt.Errorf("%s:%d: want <task id><TAB><minutes>", path, i+1)
		}
		out[strings.TrimSpace(id)] = v
	}
	return out, nil
}
