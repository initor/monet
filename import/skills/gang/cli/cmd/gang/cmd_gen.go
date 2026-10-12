package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gang/internal/exit"
	"gang/internal/gen"
)

func init() { commands["gen"] = cmdGen }

// genTagRe is Contract 0.18: review files are named review-<tag>-<n>.md.
var genTagRe = regexp.MustCompile(`^[a-z0-9._-]+$`)

// cmdGen is gang gen --tag <tag> [--only <ids>] [--max-wave N].
func cmdGen(c *Ctx, args []string) error {
	fs := newFlagSet("gen")
	tag := fs.String("tag", "", "run tag (required)")
	only := fs.String("only", "", "comma-separated task ids")
	maxWave := fs.Int("max-wave", 0, "highest wave to start (default: all)")
	if _, err := owArgs(fs, args, 0); err != nil {
		return err
	}
	if !genTagRe.MatchString(*tag) {
		return exit.New(exit.Usage, "gen: --tag is required and must match %s", genTagRe)
	}
	if *maxWave < 0 {
		return exit.New(exit.Usage, "gen: --max-wave must be 0 or more")
	}
	skill, err := needSkill(c)
	if err != nil {
		return err
	}
	defaults, err := loadRoles(skill)
	if err != nil {
		return err
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	var ids []string
	for _, id := range strings.Split(*only, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	exe, err := filepath.Abs(d.Run("bin", "gang"))
	if err != nil {
		return err
	}
	o := gen.Options{Tag: *tag, Only: ids, MaxWave: *maxWave, Defaults: defaults}
	path, summary, err := gen.Generate(c.Repo, d, p, skill, exe, o)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, path)
	fmt.Fprintln(c.Stdout, summary)
	return nil
}
