package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gang/internal/exit"
	"gang/internal/gen"
	"gang/internal/plan"
	"gang/internal/repair"
)

func init() { commands["prompts"] = cmdPrompts }

var promptVarRe = regexp.MustCompile(`\{\{([A-Za-z0-9_]+)\}\}`)

// cmdPrompts is gang prompts [--task <id>]. For each key it fills the
// variables known before a run and lists the ones the engine fills
// (Contract 0.19). A block that uses a name outside its VARS entry is an error.
func cmdPrompts(c *Ctx, args []string) error {
	fs := newFlagSet("prompts")
	taskID := fs.String("task", "", "task id (default: the first task)")
	if _, err := owArgs(fs, args, 0); err != nil {
		return err
	}
	skill, err := needSkill(c)
	if err != nil {
		return err
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	var t *plan.Task
	switch {
	case *taskID != "":
		var ok bool
		if t, ok = repair.TaskFor(p, *taskID); !ok {
			return exit.New(exit.User, "prompts: unknown task %s", *taskID)
		}
	case len(p.Tasks) > 0:
		t = &p.Tasks[0]
	default:
		return exit.New(exit.User, "prompts: the plan has no tasks")
	}
	md, err := os.ReadFile(filepath.Join(skill, "reference", "roles.md"))
	if err != nil {
		return err
	}
	prompts, err := gen.ParsePrompts(md)
	if err != nil {
		return exit.New(exit.User, "%v", err)
	}
	tpl, err := os.ReadFile(filepath.Join(skill, "engine", "engine.template.js"))
	if err != nil {
		return err
	}
	vars, err := gen.ParseVars(tpl)
	if err != nil {
		return exit.New(exit.User, "%v", err)
	}
	gangPath, err := filepath.Abs(d.Run("bin", "gang"))
	if err != nil {
		return err
	}
	known := map[string]string{
		"gang": gangPath, "repo": c.Repo, "integration": p.Repo.Integration,
		"runTag": "preview", "id": t.ID, "branch": plan.BranchFor(t),
	}
	for _, k := range gen.PromptKeys {
		allowed := map[string]bool{}
		var rest []string
		for _, v := range vars[k] {
			allowed[v] = true
			if _, ok := known[v]; !ok {
				rest = append(rest, v)
			}
		}
		for _, m := range promptVarRe.FindAllStringSubmatch(prompts[k], -1) {
			if !allowed[m[1]] {
				return exit.New(exit.User, "prompts: %s uses {{%s}}, which VARS does not list", k, m[1])
			}
		}
		text := promptVarRe.ReplaceAllStringFunc(prompts[k], func(m string) string {
			if v, ok := known[m[2:len(m)-2]]; ok {
				return v
			}
			return m
		})
		if len(rest) == 0 {
			rest = []string{"none"}
		}
		fmt.Fprintf(c.Stdout, "===== %s =====\n%s\n----- engine fills: %s\n", k, text, strings.Join(rest, ", "))
	}
	return nil
}
