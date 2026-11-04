package plan

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestTestGlobs(t *testing.T) {
	for _, c := range []struct {
		name  string
		write []string
		want  []string
	}{
		{"a file of a package", []string{"cli/internal/ledger/ledger.go"}, []string{"cli/internal/ledger/*_test.go"}},
		{"two files of one package", []string{"a/x.go", "a/y.go"}, []string{"a/*_test.go"}},
		{"two packages in write order", []string{"b/y.go", "a/x.go"}, []string{"b/*_test.go", "a/*_test.go"}},
		{"a file at the root", []string{"main.go"}, []string{"*_test.go"}},
		{"a prefix glob does not cover every test", []string{"a/x*.go"}, []string{"a/*_test.go"}},
		{"a class glob does not cover every test", []string{"a/[az]*.go"}, []string{"a/*_test.go"}},
		{"a one-character glob does not cover every test", []string{"a/?_test.go"}, []string{"a/*_test.go"}},
		{"a file named like the tail of a test name", []string{"a/t.go"}, []string{"a/*_test.go"}},
		{"a test file in write", []string{"a/x.go", "a/x_test.go"}, []string{"a/*_test.go"}},
		{"a package below a covered one", []string{"a/*.go", "a/b/x.go"}, []string{"a/b/*_test.go"}},
		{"a tree glob over a sibling", []string{"a/**", "ab/x.go"}, []string{"ab/*_test.go"}},
		{"a deep glob in the directory", []string{"a/**/x.go"}, []string{"a/**/*_test.go"}},
		{"no Go file", []string{"docs/README.md", "Makefile", "go.mod", "go.sum", "a/x.golden"}, []string{}},
		{"a Go file under testdata", []string{"a/testdata/input.go", "testdata/b.go"}, []string{}},
		{"every Go file of the package", []string{"a/*.go", "a/x.go"}, []string{}},
		{"every file of the package", []string{"a/*", "a/x.go"}, []string{}},
		{"the tests of the package", []string{"a/x.go", "a/*_test.go"}, []string{}},
		{"a tree glob", []string{"a/**", "a/b/c/x.go"}, []string{}},
		{"a folder with a trailing slash", []string{"a/", "a/b/x.go"}, []string{}},
		{"the whole repo", []string{"**", "a/x.go"}, []string{}},
		{"a one-segment glob over the package", []string{"*/*.go", "a/x.go"}, []string{}},
		{"a deep glob over the package", []string{"a/**/*.go", "a/b/c/x.go"}, []string{}},
		{"a deep test glob", []string{"**/*_test.go", "a/x.go"}, []string{}},
		{"a leading ./", []string{"./*.go", "main.go"}, []string{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := TestGlobs(c.write); !reflect.DeepEqual(got, c.want) {
				t.Errorf("TestGlobs(%q) = %q, want %q", c.write, got, c.want)
			}
		})
	}
}

// The test-only scope is the hand-written globs, then the computed ones that
// they lack, and it follows scope.write when the write list changes.
func TestTestOnlyGlobs(t *testing.T) {
	s := Scope{Write: []string{"cli/a/a.go", "cli/b/b.go"}, TestOnly: []string{"cli/b/*_test.go", "cli/c/*_test.go"}}
	want := []string{"cli/b/*_test.go", "cli/c/*_test.go", "cli/a/*_test.go"}
	if got := s.TestOnlyGlobs(); !reflect.DeepEqual(got, want) {
		t.Errorf("TestOnlyGlobs = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(s.TestOnly, []string{"cli/b/*_test.go", "cli/c/*_test.go"}) {
		t.Errorf("TestOnlyGlobs changed s.TestOnly to %q", s.TestOnly)
	}
	s = Scope{Write: []string{"cli/d/d.go"}}
	if got := s.TestOnlyGlobs(); !reflect.DeepEqual(got, []string{"cli/d/*_test.go"}) {
		t.Errorf("TestOnlyGlobs after a split = %q, want [cli/d/*_test.go]", got)
	}
}

// gang saves no computed glob: a task with no hand-written test_only has no
// test_only key, so a gang build that predates the field still reads the plan.
func TestNormalizeSavesNoComputedTestOnly(t *testing.T) {
	p := &Plan{Tasks: []Task{
		{ID: "A1", Scope: Scope{Write: []string{"cli/a/a.go"}}},
		{ID: "B2", Scope: Scope{Write: []string{"cli/b/b.go"}, TestOnly: []string{"cli/c/*_test.go"}}},
	}}
	p.Normalize()
	b, err := json.Marshal(p.Tasks)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(b), `"test_only":`); got != 1 || !strings.Contains(string(b), `"test_only":["cli/c/*_test.go"]`) {
		t.Errorf("tasks marshal %d test_only keys, want only B2's hand-written one: %s", got, b)
	}
}

// The brief prints TestOnlyRule, and schema.md must say the same, word for word.
func TestSchemaStatesTheTestOnlyRule(t *testing.T) {
	b, err := os.ReadFile("../../../reference/schema.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), TestOnlyRule) {
		t.Errorf("reference/schema.md lacks the test_only rule %q", TestOnlyRule)
	}
}
