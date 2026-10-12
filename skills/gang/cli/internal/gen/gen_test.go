package gen

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/schedule"
	"gang/internal/store"
	"gang/internal/testkit"
)

type genFixture struct {
	repo, skill, exe string
	d                store.Dir
	p                *plan.Plan
	defaults         map[string]plan.Role
}

// opts returns Options for tag with the fixture's role defaults.
func (f genFixture) opts(tag string) Options { return Options{Tag: tag, Defaults: f.defaults} }

// newGenFixture builds a repo whose integration branch holds Task lines for
// T1, for a merged repair task F03, and for X9 (neither in the plan nor a
// repair id). An unmerged repair branch gang/F07 also exists.
func newGenFixture(t *testing.T) genFixture {
	t.Helper()
	repo := testkit.Repo(t)
	p := testkit.Plan()
	p.Tasks[1].Title = "Caf\U000000e9 \U0001F600 <ok> & done"
	p.Tasks[1].Risk = "high"
	p.Tasks[1].Model = map[string]plan.Role{"review": {Model: "sonnet"}}
	p.Tasks[1].Locks = []string{"gpu"}
	p.Locks = []string{"gpu"}
	d := testkit.GangDir(t, repo, p)
	testkit.CommitOn(t, repo, p.Repo.Integration, "T1: build\n\nTask: T1\n\nTesting Done: unit", map[string]string{"a/T1.go": "package a\n"})
	testkit.CommitOn(t, repo, p.Repo.Integration, "F03: repair\n\nTask: F03", map[string]string{"a/fix.go": "package a\n"})
	testkit.CommitOn(t, repo, p.Repo.Integration, "old run\n\nTask: X9", map[string]string{"x.txt": "x\n"})
	testkit.CommitOn(t, repo, "gang/F07", "F07: unmerged repair\n\nTask: F07", map[string]string{"a/f7.go": "package a\n"})
	if err := os.WriteFile(d.Run("cap"), []byte("6\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := memGB
	memGB = func() int { return 32 }
	t.Cleanup(func() { memGB = old })
	var defaults map[string]plan.Role
	if err := json.Unmarshal([]byte(testkit.RolesJSON(t)), &defaults); err != nil {
		t.Fatal(err)
	}
	return genFixture{repo: repo, skill: testkit.SkillDir(t), exe: filepath.Join(d.Root, "run", "bin", "gang"), d: d, p: p, defaults: defaults}
}

func jsConst(t *testing.T, js, name string) string {
	t.Helper()
	for _, line := range strings.Split(js, "\n") {
		if v, ok := strings.CutPrefix(line, "const "+name+" = "); ok {
			return v
		}
	}
	t.Fatalf("no const %s in output", name)
	return ""
}

func TestGenerateFillsEveryPlaceholder(t *testing.T) {
	f := newGenFixture(t)
	path, summary, err := Generate(f.repo, f.d, f.p, f.skill, f.exe, f.opts("r1"))
	if err != nil {
		t.Fatal(err)
	}
	if path != f.d.Run("engine.js") {
		t.Fatalf("path = %s, want %s", path, f.d.Run("engine.js"))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if strings.Contains(js, "/*@@") {
		t.Fatal("a placeholder survived")
	}

	// TASKS_JSON is a string literal holding JSON: decode twice.
	var text string
	if err := json.Unmarshal([]byte(jsConst(t, js, "TASKS_JSON")), &text); err != nil {
		t.Fatal(err)
	}
	var got []taskEntry
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatal(err)
	}
	sched, err := schedule.Run(f.p, schedule.Options{Cap: 6})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(f.p.Tasks) {
		t.Fatalf("got %d tasks, want %d", len(got), len(f.p.Tasks))
	}
	for i := range f.p.Tasks {
		pt, g := &f.p.Tasks[i], got[i]
		if g.ID != pt.ID || g.Title != pt.Title || g.Branch != plan.BranchFor(pt) || g.Wave != pt.Wave || g.Risk != pt.Risk {
			t.Errorf("task %d = %+v, plan %+v", i, g, pt)
		}
		if !reflect.DeepEqual(g.Deps, pt.DependsOn) || !reflect.DeepEqual(g.MergeAfter, pt.MergeAfter) || !reflect.DeepEqual(g.Locks, pt.Locks) {
			t.Errorf("task %s edges or locks differ: %+v", pt.ID, g)
		}
		if g.Chain != sched.ChainScore[pt.ID] {
			t.Errorf("task %s chain = %v, schedule says %v", pt.ID, g.Chain, sched.ChainScore[pt.ID])
		}
		if len(g.Roles) != len(plan.Roles) {
			t.Errorf("task %s has %d roles, want %d", pt.ID, len(g.Roles), len(plan.Roles))
		}
		for _, r := range plan.Roles {
			if want := plan.ResolveRole(f.p, pt, r, f.defaults); g.Roles[r] != want {
				t.Errorf("task %s role %s = %+v, want %+v", pt.ID, r, g.Roles[r], want)
			}
		}
	}
	if got[1].Roles["review"].Model != "sonnet" {
		t.Errorf("task override lost: %+v", got[1].Roles["review"])
	}

	var sum uint32
	if err := json.Unmarshal([]byte(jsConst(t, js, "TASKSUM")), &sum); err != nil {
		t.Fatal(err)
	}
	if sum != FNV(text) {
		t.Errorf("TASKSUM = %d, FNV(text) = %d", sum, FNV(text))
	}

	var done []string
	if err := json.Unmarshal([]byte(jsConst(t, js, "DONE")), &done); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(done, []string{"F03", "T1"}) {
		t.Errorf("DONE = %v, want [F03 T1]: plan ids and merged repair ids only", done)
	}

	var cfg config
	if err := json.Unmarshal([]byte(jsConst(t, js, "CFG")), &cfg); err != nil {
		t.Fatal(err)
	}
	tip := testkit.Git(t, f.repo, "rev-parse", "refs/heads/"+f.p.Repo.Integration)
	want := config{
		RunTag: "r1", Slug: "demo", Gang: f.exe, Dir: f.d.Root, Repo: f.repo,
		Integration: "gang-demo", TrunkStart: tip, Count: 2,
		MaxImplementers: 4, // max(2, min(8, 6*2/3, 32/4))
		MaxFixRounds:    f.p.Limits.FixRounds, MaxRepairAttempts: f.p.Limits.RepairTries,
		MaxConflictRounds: 3, MaxTrunkRed: 2, HaltAfterNulls: f.p.Limits.NullStop,
		BlockedStop: f.p.Limits.BlockedStop, OnlyTasks: nil, MaxWave: 999,
		HealthAfterMerge: true, NextFixNumber: 8, // one past the unmerged gang/F07 branch
		Roles: map[string]plan.Role{
			"merge":  plan.ResolveRole(f.p, nil, "merge", f.defaults),
			"health": plan.ResolveRole(f.p, nil, "health", f.defaults),
			"repair": plan.ResolveRole(f.p, nil, "repair", f.defaults),
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("CFG =\n%+v\nwant\n%+v", cfg, want)
	}
	if !strings.Contains(jsConst(t, js, "CFG"), `"onlyTasks":null`) {
		t.Error("onlyTasks must be null without --only")
	}

	var prompts map[string]string
	if err := json.Unmarshal([]byte(jsConst(t, js, "PROMPTS")), &prompts); err != nil {
		t.Fatal(err)
	}
	wantPrompts, err := ParsePrompts([]byte(testkit.RolesMD()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prompts, wantPrompts) {
		t.Error("PROMPTS differ from roles.md")
	}
	if !strings.HasPrefix(summary, "tag r1 trunk "+tip[:7]+" tasks 2 done 2 ") || !strings.HasSuffix(summary, "next-fix F08 dropped X9") {
		t.Errorf("summary = %q", summary)
	}
}

// The fixture template runs the engine's own checksum and count guards.
func TestGeneratedEngineGuardsPassInNode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not on PATH")
	}
	f := newGenFixture(t)
	path, _, err := Generate(f.repo, f.d, f.p, f.skill, f.exe, f.opts("r1"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", path).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "ok 2" {
		t.Fatalf("node: %v: %s", err, out)
	}
}

func TestGenerateOnlyAndMaxWave(t *testing.T) {
	f := newGenFixture(t)
	o := f.opts("r2")
	o.Only, o.MaxWave = []string{"T2"}, 1
	path, _, err := Generate(f.repo, f.d, f.p, f.skill, f.exe, o)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	var cfg config
	if err := json.Unmarshal([]byte(jsConst(t, string(b), "CFG")), &cfg); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.OnlyTasks, []string{"T2"}) || cfg.MaxWave != 1 {
		t.Errorf("onlyTasks %v maxWave %d", cfg.OnlyTasks, cfg.MaxWave)
	}
	o.Only = []string{"T9"}
	_, _, err = Generate(f.repo, f.d, f.p, f.skill, f.exe, o)
	var e *exit.Error
	if !errors.As(err, &e) || e.Code != exit.User {
		t.Fatalf("unknown --only id: err = %v, want exit 1", err)
	}
}

func TestGenerateNeedsEveryRole(t *testing.T) {
	f := newGenFixture(t)
	delete(f.defaults, "sync")
	_, _, err := Generate(f.repo, f.d, f.p, f.skill, f.exe, f.opts("r1"))
	if err == nil || !strings.Contains(err.Error(), "role sync has no model or effort") {
		t.Fatalf("err = %v", err)
	}
}

func TestGeneratePlaceholderCountMustBeOne(t *testing.T) {
	cases := map[string]string{
		"/*@@DONE@@*/null appears 0 times": strings.Replace(testkit.Template, "/*@@DONE@@*/null", "[]", 1),
		"/*@@CFG@@*/null appears 2 times":  testkit.Template + "const CFG2 = /*@@CFG@@*/null\n",
	}
	for want, tpl := range cases {
		t.Run(want, func(t *testing.T) {
			f := newGenFixture(t)
			testkit.WriteSkill(t, f.skill, tpl, testkit.RolesMD(), testkit.RolesJSON(t))
			_, _, err := Generate(f.repo, f.d, f.p, f.skill, f.exe, f.opts("r1"))
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want %q", err, want)
			}
			if _, err := os.Stat(f.d.Run("engine.js")); !os.IsNotExist(err) {
				t.Fatal("engine.js written despite a bad template")
			}
		})
	}
}

// A marker inside plan text must not be replaced: markers are counted in the
// template only, then replaced in one pass.
func TestGenerateIgnoresMarkersInPlanText(t *testing.T) {
	f := newGenFixture(t)
	f.p.Tasks[0].Title = "mentions /*@@DONE@@*/null"
	if _, _, err := Generate(f.repo, f.d, f.p, f.skill, f.exe, f.opts("r1")); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateNeedsIntegrationBranch(t *testing.T) {
	f := newGenFixture(t)
	testkit.Git(t, f.repo, "update-ref", "-d", "refs/heads/"+f.p.Repo.Integration)
	_, _, err := Generate(f.repo, f.d, f.p, f.skill, f.exe, f.opts("r1"))
	if err == nil || !strings.Contains(err.Error(), "integration branch gang-demo not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestImplementersFormula(t *testing.T) {
	cases := []struct{ agentCap, ram, want int }{
		{4, 64, 2},  // 4*2/3 = 2
		{12, 64, 8}, // 8 is the ceiling
		{12, 8, 2},  // 8/4 = 2
		{9, 64, 6},  // 9*2/3 = 6
		{1, 1, 2},   // 2 is the floor
		{14, 20, 5}, // 20/4 = 5
	}
	for _, c := range cases {
		if got := implementers(c.agentCap, c.ram); got != c.want {
			t.Errorf("implementers(%d, %d) = %d, want %d", c.agentCap, c.ram, got, c.want)
		}
	}
}
