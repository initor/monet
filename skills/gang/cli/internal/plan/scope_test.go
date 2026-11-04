package plan

import (
	"reflect"
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
		{"a glob that matches some test names only", []string{"a/a*.go"}, []string{"a/*_test.go"}},
		{"a test file in write", []string{"a/x.go", "a/x_test.go"}, []string{"a/*_test.go"}},
		{"a package below a covered one", []string{"a/*.go", "a/b/x.go"}, []string{"a/b/*_test.go"}},
		{"no Go file", []string{"docs/README.md", "Makefile", "go.mod", "go.sum"}, []string{}},
		{"every Go file of the package", []string{"a/*.go", "a/x.go"}, []string{}},
		{"every file of the package", []string{"a/*", "a/x.go"}, []string{}},
		{"the tests of the package", []string{"a/x.go", "a/*_test.go"}, []string{}},
		{"a tree glob", []string{"a/**", "a/b/c/x.go"}, []string{}},
		{"the whole repo", []string{"**", "a/x.go"}, []string{}},
		{"a one-segment glob over the package", []string{"*/*.go", "a/x.go"}, []string{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := TestGlobs(c.write); !reflect.DeepEqual(got, c.want) {
				t.Errorf("TestGlobs(%q) = %q, want %q", c.write, got, c.want)
			}
		})
	}
}

// Normalize adds the test globs of scope.write to scope.test_only, after the
// globs that the plan already holds, and a second Normalize adds nothing.
func TestNormalizeFillsTestOnlyScope(t *testing.T) {
	p := &Plan{Tasks: []Task{
		{ID: "A1", Scope: Scope{Write: []string{"cli/a/a.go", "cli/b/b.go"}, TestOnly: []string{"cli/b/*_test.go", "cli/c/*_test.go"}}},
		{ID: "B2", Scope: Scope{Write: []string{"docs/index.md"}}},
	}}
	p.Normalize()
	want := []string{"cli/b/*_test.go", "cli/c/*_test.go", "cli/a/*_test.go"}
	if got := p.Tasks[0].Scope.TestOnly; !reflect.DeepEqual(got, want) {
		t.Errorf("A1 test_only = %q, want %q", got, want)
	}
	if got := p.Tasks[1].Scope.TestOnly; got == nil || len(got) != 0 {
		t.Errorf("B2 test_only = %#v, want an empty list", got)
	}
	p.Normalize()
	if got := p.Tasks[0].Scope.TestOnly; !reflect.DeepEqual(got, want) {
		t.Errorf("A1 test_only after a second Normalize = %q, want %q", got, want)
	}
}
