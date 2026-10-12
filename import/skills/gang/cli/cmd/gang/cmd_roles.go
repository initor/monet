package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gang/internal/exit"
	"gang/internal/plan"
)

func init() { commands["roles"] = cmdRoles }

// cmdRoles prints "<role> <model> <effort>" for every role in plan.Roles
// order, resolved at run level: plan models, then <skill>/roles.json.
func cmdRoles(c *Ctx, args []string) error {
	pos, err := parseFlags(newFlagSet("roles"), args)
	if err != nil {
		return err
	}
	if len(pos) > 0 {
		return exit.New(exit.Usage, "roles takes no arguments")
	}
	skill, err := needSkill(c)
	if err != nil {
		return err
	}
	p, err := loadPlan(c)
	if err != nil {
		return err
	}
	defaults, err := loadRoles(skill)
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, role := range plan.Roles {
		r := plan.ResolveRole(p, nil, role, defaults)
		if r.Model == "" || r.Effort == "" {
			return exit.New(exit.User, "role %s resolves to model %q effort %q: add it to roles.json", role, r.Model, r.Effort)
		}
		fmt.Fprintf(&b, "%s %s %s\n", role, r.Model, r.Effort)
	}
	fmt.Fprint(c.Stdout, b.String())
	return nil
}

// loadRoles reads <skill>/roles.json, a JSON object of role to
// {"model","effort"}. Unknown roles, non-alias models, and unknown efforts
// are user errors.
func loadRoles(skill string) (map[string]plan.Role, error) {
	path := filepath.Join(skill, "roles.json")
	var m map[string]plan.Role
	if err := decodeStrict(path, &m); err != nil {
		return nil, exit.New(exit.User, "%v", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r := m[k]
		switch {
		case !slices.Contains(plan.Roles, k):
			return nil, exit.New(exit.User, "%s: unknown role %q", path, k)
		case r.Model != "" && !slices.Contains(plan.Aliases, r.Model):
			return nil, exit.New(exit.User, "%s: %s model %q is not one of %s", path, k, r.Model, strings.Join(plan.Aliases, ", "))
		case r.Effort != "" && !slices.Contains(plan.Efforts, r.Effort):
			return nil, exit.New(exit.User, "%s: %s effort %q is not one of %s", path, k, r.Effort, strings.Join(plan.Efforts, ", "))
		}
	}
	return m, nil
}
