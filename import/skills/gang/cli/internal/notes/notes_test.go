package notes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"

	"gang/internal/store"
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestReportAppendsSections(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	path, err := Report(d, "A1", "", []byte("first\n\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := d.Run("tasks", "A1", "report.md"); path != want {
		t.Fatalf("path %s, want %s", path, want)
	}
	if _, err := Report(d, "A1", "round\n2", []byte("second line")); err != nil {
		t.Fatal(err)
	}
	want := "## report\n\nfirst\n\n## round 2\n\nsecond line\n"
	if got := read(t, path); got != want {
		t.Fatalf("report.md = %q, want %q", got, want)
	}
	if _, err := Report(d, "A1", "x", []byte(" \n")); err == nil {
		t.Fatal("an empty report was accepted")
	}
	for _, bad := range []string{"", ".", "..", "a/b"} {
		if _, err := Report(d, bad, "x", []byte("y")); err == nil {
			t.Errorf("task id %q accepted", bad)
		}
	}
}

func TestWriteNamed(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	path, err := WriteNamed(d, "A1", "review-r1-1.md", []byte("findings\n"))
	if err != nil {
		t.Fatal(err)
	}
	if path != d.Run("tasks", "A1", "review-r1-1.md") || read(t, path) != "findings\n" {
		t.Fatalf("path %s content %q", path, read(t, path))
	}
	for _, bad := range []string{"Review.md", "../x.md", "a/b.md", "x.txt", ".md", "owner.md", "report.md"} {
		if _, err := WriteNamed(d, "A1", bad, []byte("x")); err == nil {
			t.Errorf("name %q accepted", bad)
		}
	}
	entries, _ := os.ReadDir(d.Run("tasks", "A1"))
	if len(entries) != 1 {
		t.Fatalf("task dir holds %d entries, want only review-r1-1.md", len(entries))
	}
}

func TestParkWritesContractBlocks(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	id1, err := Park(d, "A1", "Which port?\nAny?", "8080")
	if err != nil || id1 != "G1" {
		t.Fatalf("first park: %s %v", id1, err)
	}
	id2, err := Park(d, "B2", "Keep the flag?", "yes")
	if err != nil || id2 != "G2" {
		t.Fatalf("second park: %s %v", id2, err)
	}
	want := "## G1 A1 open\nQuestion: Which port? Any?\nDefault: 8080\n\n## G2 B2 open\nQuestion: Keep the flag?\nDefault: yes\n"
	if got := read(t, d.Run("gates.md")); got != want {
		t.Fatalf("gates.md = %q, want %q", got, want)
	}
	if _, err := Park(d, "A 1", "q", "d"); err == nil {
		t.Fatal("a task id with a space was accepted")
	}
}

func TestParkConcurrentIDsAreUnique(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var ids []string
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := Park(d, "A1", fmt.Sprintf("q%d", i), "d")
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			ids = append(ids, id)
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(ids, func(i, j int) bool {
		return len(ids[i]) < len(ids[j]) || (len(ids[i]) == len(ids[j]) && ids[i] < ids[j])
	})
	want := []string{"G1", "G2", "G3", "G4", "G5", "G6", "G7", "G8", "G9", "G10"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids %v, want %v", ids, want)
	}
}

func TestAnswerAndList(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	if gs, err := List(d); err != nil || len(gs) != 0 {
		t.Fatalf("no gates.md: %v %v", gs, err)
	}
	for _, q := range []string{"q1", "q2"} {
		if _, err := Park(d, "A1", q, "d"); err != nil {
			t.Fatal(err)
		}
	}
	if err := Answer(d, "G1", "use\n9090"); err != nil {
		t.Fatal(err)
	}
	want := "## G1 A1 answered\nQuestion: q1\nDefault: d\nAnswer: use 9090\n\n## G2 A1 open\nQuestion: q2\nDefault: d\n"
	if got := read(t, d.Run("gates.md")); got != want {
		t.Fatalf("gates.md = %q, want %q", got, want)
	}
	gs, err := List(d)
	if err != nil || !reflect.DeepEqual(gs, []Gate{{ID: "G2", Task: "A1", State: "open", Question: "q2", Default: "d"}}) {
		t.Fatalf("open gates %+v %v", gs, err)
	}
	if err := Answer(d, "G9", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown gate: %v, want ErrNotFound", err)
	}
	if err := Answer(d, "G2", " "); err == nil {
		t.Fatal("an empty answer was accepted")
	}
}

func TestStandingNumberingAndStop(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	if s, err := Show(d); err != nil || s != "" {
		t.Fatalf("no standing.md: %q %v", s, err)
	}
	steps := []struct {
		do   func() error
		want string
	}{
		{func() error { return Add(d, "Run make check before commit.") }, "1. Run make check before commit.\n"},
		{func() error { return Add(d, "No new deps.") }, "1. Run make check before commit.\n2. No new deps.\n"},
		{func() error { return Stop(d, "usage reset at 14:00") }, "STOP: usage reset at 14:00\n1. Run make check before commit.\n2. No new deps.\n"},
		{func() error { return Stop(d, "wave PR red") }, "STOP: wave PR red\n1. Run make check before commit.\n2. No new deps.\n"},
		{func() error { return Add(d, "Keep logs.") }, "STOP: wave PR red\n1. Run make check before commit.\n2. No new deps.\n3. Keep logs.\n"},
		{func() error { return ClearStop(d) }, "1. Run make check before commit.\n2. No new deps.\n3. Keep logs.\n"},
		{func() error { return ClearStop(d) }, "1. Run make check before commit.\n2. No new deps.\n3. Keep logs.\n"},
	}
	for i, s := range steps {
		if err := s.do(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if got, _ := Show(d); got != s.want {
			t.Fatalf("step %d: standing.md = %q, want %q", i, got, s.want)
		}
	}
	if err := Add(d, "  "); err == nil {
		t.Fatal("an empty order was accepted")
	}
	if err := Stop(d, ""); err == nil {
		t.Fatal("a stop without a reason was accepted")
	}
}

func TestStandingRenumbersHandEdits(t *testing.T) {
	d := store.Dir{Root: t.TempDir()}
	path := d.Run("standing.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("1. a\n\n5. b\nloose line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Add(d, "c"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != "1. a\n2. b\n3. loose line\n4. c\n" {
		t.Fatalf("standing.md = %q", got)
	}
}
