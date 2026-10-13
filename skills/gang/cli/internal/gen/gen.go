package gen

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// Options are the gang gen flags plus the role defaults. MaxWave 0 means no
// limit (999). Defaults is roles.json, loaded and checked by the caller.
type Options struct {
	Tag      string
	Only     []string
	MaxWave  int
	Defaults map[string]plan.Role
}

// taskEntry is one TASKS element (Contract 8).
type taskEntry struct {
	ID         string               `json:"id"`
	Title      string               `json:"title"`
	Branch     string               `json:"branch"`
	Deps       []string             `json:"deps"`
	MergeAfter []string             `json:"mergeAfter"`
	Wave       int                  `json:"wave"`
	Chain      float64              `json:"chain"`
	Risk       string               `json:"risk"`
	Locks      []string             `json:"locks"`
	Roles      map[string]plan.Role `json:"roles"`
}

// config is CFG (Contract 8), fields in contract order.
type config struct {
	RunTag            string               `json:"runTag"`
	Slug              string               `json:"slug"`
	Gang              string               `json:"gang"`
	Dir               string               `json:"dir"`
	Repo              string               `json:"repo"`
	Integration       string               `json:"integration"`
	TrunkStart        string               `json:"trunkStart"`
	Count             int                  `json:"count"`
	MaxImplementers   int                  `json:"maxImplementers"`
	MaxFixRounds      int                  `json:"maxFixRounds"`
	MaxRepairAttempts int                  `json:"maxRepairAttempts"`
	MaxConflictRounds int                  `json:"maxConflictRounds"`
	MaxTrunkRed       int                  `json:"maxTrunkRed"`
	HaltAfterNulls    int                  `json:"haltAfterNulls"`
	BlockedStop       int                  `json:"blockedStop"`
	OnlyTasks         []string             `json:"onlyTasks"`
	MaxWave           int                  `json:"maxWave"`
	HealthAfterMerge  bool                 `json:"healthAfterMerge"`
	NextFixNumber     int                  `json:"nextFixNumber"`
	Roles             map[string]plan.Role `json:"roles"`
}

// markers are the Contract 8 placeholder names, in template order.
var markers = []string{"TASKS_JSON", "TASKSUM", "DONE", "CFG", "PROMPTS"}

// fixRe is a repair task id (Contract 0.16).
var fixRe = regexp.MustCompile(`^F([0-9]{2,})$`)

// memGB reports physical memory in GB. Tests replace it.
var memGB = systemMemGB

// Generate writes run/engine.js from the skill's template and prompts, the
// plan, git, and o.Defaults. It returns the path and a one-line summary.
func Generate(repo string, d store.Dir, p *plan.Plan, skillDir, exe string, o Options) (string, string, error) {
	tpl, err := os.ReadFile(filepath.Join(skillDir, "engine", "engine.template.js"))
	if err != nil {
		return "", "", err
	}
	md, err := os.ReadFile(filepath.Join(skillDir, "reference", "roles.md"))
	if err != nil {
		return "", "", err
	}
	prompts, err := ParsePrompts(md)
	if err != nil {
		return "", "", err
	}
	agentCap, err := readCap(d)
	if err != nil {
		return "", "", err
	}
	sched, err := schedule.Run(p, schedule.Options{Cap: agentCap})
	if err != nil {
		return "", "", err
	}
	integ := "refs/heads/" + p.Repo.Integration
	trunk, err := gitx.RevParse(repo, integ)
	if errors.Is(err, gitx.ErrNotFound) {
		return "", "", exit.New(exit.User, "integration branch %s not found", p.Repo.Integration)
	}
	if err != nil {
		return "", "", err
	}
	merged, err := gitx.TaskIDs(repo, integ)
	if err != nil {
		return "", "", err
	}
	branches, err := gitx.Run(repo, "for-each-ref", "--format=%(refname:short)", "refs/heads/gang/")
	if err != nil {
		return "", "", err
	}

	// DONE holds plan ids and merged repair ids (the engine skips the latter).
	// Any other id would stop the engine, so it is dropped and named in the summary.
	done, dropped, maxFix := []string{}, []string{}, 0
	for _, id := range merged {
		_, inPlan := p.Task(id)
		switch {
		case inPlan || fixRe.MatchString(id):
			done = append(done, id)
		default:
			dropped = append(dropped, id)
		}
	}
	for _, id := range append(append([]string{}, done...), strings.Split(branches, "\n")...) {
		if m := fixRe.FindStringSubmatch(strings.TrimPrefix(id, "gang/")); m != nil {
			n, _ := strconv.Atoi(m[1])
			maxFix = max(maxFix, n)
		}
	}
	sort.Strings(done)

	var only []string
	for _, id := range o.Only {
		if _, ok := p.Task(id); !ok {
			return "", "", exit.New(exit.User, "--only names unknown task %s", id)
		}
		only = append(only, id)
	}
	maxWave := 999
	if o.MaxWave > 0 {
		maxWave = o.MaxWave
	}

	entries := make([]taskEntry, 0, len(p.Tasks))
	for i := range p.Tasks {
		t := &p.Tasks[i]
		roles := map[string]plan.Role{}
		for _, r := range plan.Roles {
			res := plan.ResolveRole(p, t, r, o.Defaults)
			if res.Model == "" || res.Effort == "" {
				return "", "", exit.New(exit.User, "task %s: role %s has no model or effort", t.ID, r)
			}
			roles[r] = res
		}
		entries = append(entries, taskEntry{
			ID: t.ID, Title: t.Title, Branch: plan.BranchFor(t),
			Deps: t.DependsOn, MergeAfter: t.MergeAfter, Wave: t.Wave,
			Chain: sched.ChainScore[t.ID], Risk: t.Risk, Locks: t.Locks, Roles: roles,
		})
	}
	runRoles := map[string]plan.Role{}
	for _, r := range []string{"merge", "health", "repair"} {
		runRoles[r] = plan.ResolveRole(p, nil, r, o.Defaults)
	}
	dir, err := filepath.Abs(d.Root)
	if err != nil {
		return "", "", err
	}
	cfg := config{
		RunTag: o.Tag, Slug: p.Slug, Gang: exe, Dir: dir, Repo: repo,
		Integration: p.Repo.Integration, TrunkStart: trunk, Count: len(p.Tasks),
		MaxImplementers:   implementers(agentCap, memGB()),
		MaxFixRounds:      p.Limits.FixRounds,
		MaxRepairAttempts: p.Limits.RepairTries,
		MaxConflictRounds: 3,
		MaxTrunkRed:       2,
		HaltAfterNulls:    p.Limits.NullStop,
		BlockedStop:       p.Limits.BlockedStop,
		OnlyTasks:         only,
		MaxWave:           maxWave,
		HealthAfterMerge:  len(p.Commands.Health) > 0,
		NextFixNumber:     maxFix + 1,
		Roles:             runRoles,
	}

	tasksText, err := json.Marshal(entries)
	if err != nil {
		return "", "", err
	}
	values := map[string]string{"TASKSUM": strconv.FormatUint(uint64(FNV(string(tasksText))), 10)}
	for name, v := range map[string]any{"TASKS_JSON": string(tasksText), "DONE": done, "CFG": cfg, "PROMPTS": prompts} {
		b, err := json.Marshal(v)
		if err != nil {
			return "", "", err
		}
		values[name] = string(b)
	}
	out, err := fill(string(tpl), values)
	if err != nil {
		return "", "", err
	}
	path := d.Run("engine.js")
	if err := store.WriteFileAtomic(path, []byte(out), 0o644); err != nil {
		return "", "", err
	}
	onlyText := "-"
	if len(only) > 0 {
		onlyText = strings.Join(only, ",")
	}
	summary := fmt.Sprintf("tag %s trunk %s tasks %d done %d only %s max-wave %d implementers %d next-fix F%02d",
		o.Tag, short(trunk), len(p.Tasks), len(done), onlyText, maxWave, cfg.MaxImplementers, cfg.NextFixNumber)
	if len(dropped) > 0 {
		summary += " dropped " + strings.Join(dropped, ",")
	}
	return path, summary, nil
}

// fill replaces each marker exactly once. Counting first and replacing in one
// pass keeps injected text from being read as a marker.
func fill(tpl string, values map[string]string) (string, error) {
	var pairs []string
	for _, name := range markers {
		m := "/*@@" + name + "@@*/null"
		if n := strings.Count(tpl, m); n != 1 {
			return "", exit.New(exit.User, "engine template: placeholder %s appears %d times, want 1", m, n)
		}
		pairs = append(pairs, m, values[name])
	}
	return strings.NewReplacer(pairs...).Replace(tpl), nil
}

// implementers is demo's formula: max(2, min(8, cap*2/3, RAM GB/4)).
func implementers(agentCap, ramGB int) int {
	return max(2, min(8, agentCap*2/3, ramGB/4))
}

func readCap(d store.Dir) (int, error) {
	b, err := os.ReadFile(d.Run("cap"))
	if errors.Is(err, fs.ErrNotExist) {
		return 4, nil
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

func systemMemGB() int {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("/usr/sbin/sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			if n, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil {
				return int(n >> 30)
			}
		}
	case "linux":
		b, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				f := strings.Fields(line)
				if len(f) >= 2 && f[0] == "MemTotal:" {
					if kb, err := strconv.ParseUint(f[1], 10, 64); err == nil {
						return int(kb >> 20)
					}
				}
			}
		}
	}
	return 16
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
