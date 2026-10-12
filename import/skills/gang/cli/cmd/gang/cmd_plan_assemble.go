package main

import (
	"fmt"
	"path/filepath"
	"sort"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
)

func init() { planCommands["assemble"] = cmdPlanAssemble }

// cmdPlanAssemble joins a skeleton plan (run config, no tasks) with task
// drafts, each a *.json file holding {"tasks":[...]}. It sorts the tasks by
// id, fails on a duplicate id, saves plan/plan.json, and prints the count.
// The defaults are plan/skeleton.json and plan/drafts/ (Contract 0 item 5).
func cmdPlanAssemble(c *Ctx, args []string) error {
	fs := newFlagSet("plan assemble")
	skeleton := fs.String("skeleton", filepath.Join(c.Dir, "plan", "skeleton.json"), "plan JSON with the run config and no tasks")
	drafts := fs.String("drafts", filepath.Join(c.Dir, "plan", "drafts"), `dir of *.json files, each {"tasks":[...]}`)
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return exit.New(exit.Usage, "usage: gang plan assemble [--skeleton <file>] [--drafts <dir>]")
	}
	p, err := plan.Load(*skeleton)
	if err != nil {
		return exit.New(exit.User, "skeleton: %v", err)
	}
	if len(p.Tasks) > 0 {
		return exit.New(exit.User, "skeleton %s already holds %d tasks", *skeleton, len(p.Tasks))
	}
	files, err := filepath.Glob(filepath.Join(*drafts, "*.json"))
	if err != nil {
		return exit.New(exit.User, "drafts: %v", err)
	}
	if len(files) == 0 {
		return exit.New(exit.User, "no *.json drafts in %s", *drafts)
	}
	sort.Strings(files)
	from := map[string]string{}
	for _, f := range files {
		var d struct {
			Tasks []plan.Task `json:"tasks"`
		}
		if err := decodeStrict(f, &d); err != nil {
			return exit.New(exit.User, "draft %v", err)
		}
		for _, t := range d.Tasks {
			if prev, dup := from[t.ID]; dup {
				return exit.New(exit.User, "task %s is in %s and %s", t.ID, prev, f)
			}
			from[t.ID] = f
			p.Tasks = append(p.Tasks, t)
		}
	}
	sort.SliceStable(p.Tasks, func(i, j int) bool { return p.Tasks[i].ID < p.Tasks[j].ID })
	if err := plan.Save(store.Dir{Root: c.Dir}.Plan(), p); err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, len(p.Tasks))
	return nil
}
