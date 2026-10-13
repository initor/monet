package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gang/internal/exit"
	"gang/internal/gitx"
	"gang/internal/lint"
	"gang/internal/plan"
	"gang/internal/store"
)

func init() { planCommands["commit"] = cmdPlanCommit }

// beforePlanCAS runs after the plan commit is built and before the plan ref
// moves. Tests replace it to move the ref in between.
var beforePlanCAS = func() {}

// cmdPlanCommit lints the plan, re-renders plan.md, and commits a tree that
// holds only plan.json and plan.md to refs/gang/<slug>/plan, with the
// previous ref commit as parent and a compare-and-swap update. It never
// touches a branch, the index, or the work tree of the main checkout. An
// unchanged plan makes no commit. It prints the ref's SHA.
func cmdPlanCommit(c *Ctx, args []string) error {
	fs := newFlagSet("plan commit")
	msg := fs.String("message", "update plan", "subject after plan(<slug>): ")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return exit.New(exit.Usage, "plan commit takes no arguments")
	}
	p, err := loadPlan(c)
	if err != nil {
		return err
	}
	if filepath.Base(c.Dir) != p.Slug {
		return exit.New(exit.User, "gang dir %s does not match plan slug %q", c.Dir, p.Slug)
	}
	if probs := lint.Lint(p, lint.Options{}); len(probs) > 0 {
		for _, pr := range probs {
			fmt.Fprintln(c.Stderr, pr.String())
		}
		return exit.New(exit.User, "plan commit: %d lint problems", len(probs))
	}
	if _, err := writePlanMD(c, p); err != nil {
		return err
	}

	ref := plan.Ref(p.Slug)
	old, err := gitx.RevParse(c.Repo, ref)
	if err != nil && !errors.Is(err, gitx.ErrNotFound) {
		return err
	}
	tree, err := planTree(c)
	if err != nil {
		return err
	}
	var parents []string
	if old != "" {
		oldTree, err := gitx.Run(c.Repo, "rev-parse", old+"^{tree}")
		if err != nil {
			return err
		}
		if oldTree == tree {
			fmt.Fprintln(c.Stdout, old)
			return nil
		}
		parents = []string{old}
	}
	sha, err := gitx.CommitTree(c.Repo, tree, parents, fmt.Sprintf("plan(%s): %s", p.Slug, *msg))
	if err != nil {
		return err
	}
	beforePlanCAS()
	err = gitx.UpdateRefCAS(c.Repo, ref, sha, old)
	if errors.Is(err, gitx.ErrCASLost) {
		return exit.New(exit.Conflict, "%s moved during plan commit: run it again", ref)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, sha)
	return nil
}

// planTree returns a tree that holds only plan.json and plan.md from the
// gang dir, at its root. It builds the tree in a temporary index under run/.
func planTree(c *Ctx) (string, error) {
	d := store.Dir{Root: c.Dir}
	if err := os.MkdirAll(d.Run(), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(d.Run(), "plan-commit-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(tmp, "index")}
	for _, f := range []struct{ src, name string }{{d.Plan(), "plan.json"}, {d.PlanMD(), "plan.md"}} {
		blob, err := gitx.RunEnv(c.Repo, env, "hash-object", "-w", "--no-filters", "--", f.src)
		if err != nil {
			return "", err
		}
		if _, err := gitx.RunEnv(c.Repo, env, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+f.name); err != nil {
			return "", err
		}
	}
	return gitx.RunEnv(c.Repo, env, "write-tree")
}
