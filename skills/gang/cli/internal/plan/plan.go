// Package plan holds the plan.json model: the task graph and the run config.
package plan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gang/internal/store"
)

type Plan struct {
	Version      int               `json:"version"`
	Slug         string            `json:"slug"`
	Repo         Repo              `json:"repo"`
	Commands     Commands          `json:"commands"`
	Env          map[string]string `json:"env"`
	Locks        []string          `json:"locks"`
	Limits       Limits            `json:"limits"`
	Models       map[string]Role   `json:"models"`
	Forbidden    []string          `json:"forbidden"`
	Done         string            `json:"done"`
	Requirements []Requirement     `json:"requirements"`
	Tasks        []Task            `json:"tasks"`
}

type Repo struct {
	Remote      string `json:"remote"`
	Base        string `json:"base"`
	Integration string `json:"integration"`
	GHAccount   string `json:"gh_account"`
	WaveTitle   string `json:"wave_title"`
}

type Commands struct {
	Gate     []string `json:"gate"`
	Health   []string `json:"health"`
	WaveGate []string `json:"wave_gate"`
}

type Limits struct {
	TaskLines      int `json:"task_lines"`
	SplitLines     int `json:"split_lines"`
	FixRounds      int `json:"fix_rounds"`
	RepairTries    int `json:"repair_tries"`
	BlockedStop    int `json:"blocked_stop"`
	NullStop       int `json:"null_stop"`
	Retries        int `json:"retries"`
	GateSlots      int `json:"gate_slots"`
	GateTimeoutMin int `json:"gate_timeout_min"`
	StallMin       int `json:"stall_min"`
	MaxWorktrees   int `json:"max_worktrees"`
}

type Role struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

type Requirement struct {
	ID     string `json:"id"`
	Source string `json:"source"`
}

type Task struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Goal         string          `json:"goal"`
	Branch       string          `json:"branch,omitempty"`
	Source       string          `json:"source,omitempty"` // the adopted source task this task is a part of
	Scope        Scope           `json:"scope"`
	Peers        []string        `json:"peers"`
	DependsOn    []string        `json:"depends_on"`
	MergeAfter   []string        `json:"merge_after"`
	Requirements []string        `json:"requirements"`
	Interfaces   Interfaces      `json:"interfaces"`
	Acceptance   []string        `json:"acceptance"`
	Verify       Verify          `json:"verify"`
	Size         Size            `json:"size"`
	TimeboxMin   int             `json:"timebox_min"`
	Forbidden    []string        `json:"forbidden"`
	Locks        []string        `json:"locks"`
	Risk         string          `json:"risk"`
	Wave         int             `json:"wave"`
	Model        map[string]Role `json:"model,omitempty"`
	Notes        string          `json:"notes,omitempty"`
}

type Scope struct {
	Write  []string `json:"write"`
	Shared []string `json:"shared"`
}

type Interfaces struct {
	Consumes []string `json:"consumes"`
	Produces []string `json:"produces"`
}

type Verify struct {
	TestsFirst []string `json:"tests_first"`
	Commands   []string `json:"commands"`
}

// Size is a task's planned diff size. Reason says why a task above
// limits.task_lines cannot be split.
type Size struct {
	Lines  int    `json:"lines"`
	Reason string `json:"reason,omitempty"`
}

// Roles is the closed role set.
var Roles = []string{"implement", "implement_hard", "review", "review_adversarial", "fix", "sync", "merge", "health", "repair", "plan_draft", "plan_review"}

// Aliases are the allowed model values.
var Aliases = []string{"opus", "sonnet", "haiku", "fable"}

// Efforts are the allowed effort values.
var Efforts = []string{"low", "medium", "high", "xhigh", "max"}

// DefaultWaveTitle is the wave PR title template Normalize fills in.
const DefaultWaveTitle = "chore(gang): land wave {k} of {slug}"

// Load reads a plan strictly: unknown fields and trailing data are errors.
// The plan comes back normalized.
func Load(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p Plan
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := dec.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: trailing data after the plan object", path)
	}
	p.Normalize()
	return &p, nil
}

// Normalize fills zero limits and an empty wave title with defaults, turns
// nil slices and maps into empty ones, and sets an empty risk to "low".
func (p *Plan) Normalize() {
	l := &p.Limits
	orDefault(&l.TaskLines, 100)
	orDefault(&l.SplitLines, 300)
	orDefault(&l.FixRounds, 3)
	orDefault(&l.RepairTries, 2)
	orDefault(&l.BlockedStop, 3)
	orDefault(&l.NullStop, 6)
	orDefault(&l.Retries, 2)
	orDefault(&l.GateSlots, 3)
	orDefault(&l.GateTimeoutMin, 60)
	orDefault(&l.StallMin, 60)
	orDefault(&l.MaxWorktrees, 400)
	if p.Repo.WaveTitle == "" {
		p.Repo.WaveTitle = DefaultWaveTitle
	}
	nonNil(&p.Commands.Gate)
	nonNil(&p.Commands.Health)
	nonNil(&p.Commands.WaveGate)
	nonNil(&p.Locks)
	nonNil(&p.Forbidden)
	nonNil(&p.Requirements)
	nonNil(&p.Tasks)
	if p.Env == nil {
		p.Env = map[string]string{}
	}
	if p.Models == nil {
		p.Models = map[string]Role{}
	}
	for i := range p.Tasks {
		t := &p.Tasks[i]
		for _, s := range []*[]string{&t.Scope.Write, &t.Scope.Shared, &t.Peers, &t.DependsOn, &t.MergeAfter,
			&t.Requirements, &t.Interfaces.Consumes, &t.Interfaces.Produces, &t.Acceptance,
			&t.Verify.TestsFirst, &t.Verify.Commands, &t.Forbidden, &t.Locks} {
			nonNil(s)
		}
		if t.Risk == "" {
			t.Risk = "low"
		}
	}
}

// DefaultLimits returns the limits of a zero Plan after Normalize. Lint holds a
// plan to them: a plan may lower task_lines and split_lines, not raise them.
func DefaultLimits() Limits {
	var p Plan
	p.Normalize()
	return p.Limits
}

func orDefault(v *int, d int) {
	if *v == 0 {
		*v = d
	}
}

func nonNil[T any](s *[]T) {
	if *s == nil {
		*s = []T{}
	}
}

// Save normalizes p, then writes it as indented JSON plus a trailing newline,
// atomically.
func Save(path string, p *Plan) error {
	p.Normalize()
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return store.WriteFileAtomic(path, append(data, '\n'), 0o644)
}

// Task returns the task with id.
func (p *Plan) Task(id string) (*Task, bool) {
	for i := range p.Tasks {
		if p.Tasks[i].ID == id {
			return &p.Tasks[i], true
		}
	}
	return nil, false
}

// WaveTitle renders the wave PR title for wave k: repo.wave_title, or the
// default when it is empty, with {k} and {slug} replaced.
func WaveTitle(p *Plan, k int) string {
	tmpl := p.Repo.WaveTitle
	if tmpl == "" {
		tmpl = DefaultWaveTitle
	}
	return strings.NewReplacer("{k}", strconv.Itoa(k), "{slug}", p.Slug).Replace(tmpl)
}

// Ref is the private ref that holds the approved plan: refs/gang/<slug>/plan.
func Ref(slug string) string { return "refs/gang/" + slug + "/plan" }

// BranchFor returns the task's branch: t.Branch when set, else gang/<id>.
func BranchFor(t *Task) string {
	if t.Branch != "" {
		return t.Branch
	}
	return "gang/" + t.ID
}

// ResolveRole resolves role through the task model, then the plan models,
// then defaults. Each field falls through on its own. A nil t resolves at
// run level, and a nil p skips the plan layer.
func ResolveRole(p *Plan, t *Task, role string, defaults map[string]Role) Role {
	var layers []map[string]Role
	if t != nil {
		layers = append(layers, t.Model)
	}
	if p != nil {
		layers = append(layers, p.Models)
	}
	layers = append(layers, defaults)
	var r Role
	for _, m := range layers {
		l := m[role]
		if r.Model == "" {
			r.Model = l.Model
		}
		if r.Effort == "" {
			r.Effort = l.Effort
		}
	}
	return r
}
