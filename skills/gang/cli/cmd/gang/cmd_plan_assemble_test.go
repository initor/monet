package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
)

// assembleInputs writes, under root, skeleton.json (testPlan without tasks)
// and drafts/ with B1 in b.json and A1 in a.json, plus a file that is not a
// draft.
func assembleInputs(t *testing.T, root string) (skeleton, drafts string) {
	t.Helper()
	skel := testPlan()
	skel.Tasks = nil
	skeleton = filepath.Join(root, "skeleton.json")
	if err := plan.Save(skeleton, skel); err != nil {
		t.Fatal(err)
	}
	drafts = filepath.Join(root, "drafts")
	full := testPlan()
	writeDraft(t, filepath.Join(drafts, "b.json"), full.Tasks[1])
	writeDraft(t, filepath.Join(drafts, "a.json"), full.Tasks[0])
	if err := os.WriteFile(filepath.Join(drafts, "notes.txt"), []byte("not a draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return skeleton, drafts
}

func writeDraft(t *testing.T, path string, tasks ...plan.Task) {
	t.Helper()
	data, err := json.Marshal(map[string][]plan.Task{"tasks": tasks})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The default paths come from Contract 0 item 5: plan/skeleton.json and
// plan/drafts/.
func TestPlanAssembleDefaults(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	if err := os.Remove(store.Dir{Root: dir}.Plan()); err != nil {
		t.Fatal(err)
	}
	assembleInputs(t, filepath.Join(dir, "plan"))
	out, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "assemble")
	if code != 0 || out != "2\n" {
		t.Fatalf("code %d out %q stderr %q", code, out, stderr)
	}
	p, err := plan.Load(store.Dir{Root: dir}.Plan())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, testPlan()) {
		t.Error("the assembled plan differs from testPlan(), which the skeleton and drafts were cut from")
	}
}

func TestPlanAssembleFlags(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	skeleton, drafts := assembleInputs(t, t.TempDir())
	out, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "assemble", "--skeleton", skeleton, "--drafts", drafts)
	if code != 0 || out != "2\n" {
		t.Fatalf("code %d out %q stderr %q", code, out, stderr)
	}
	p, err := plan.Load(store.Dir{Root: dir}.Plan())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, testPlan()) {
		t.Error("the assembled plan differs from testPlan()")
	}
}

// A draft names only scope.write. plan assemble writes the test_only glob of
// each Go package in it into plan.json, and lint passes the result.
func TestPlanAssembleAddsTestOnlyScope(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	skeleton, drafts := assembleInputs(t, t.TempDir())
	a1 := testPlan().Tasks[0]
	a1.Scope = plan.Scope{Write: []string{"alpha/a.go", "alpha/data/**"}}
	writeDraft(t, filepath.Join(drafts, "a.json"), a1)
	if _, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "assemble", "--skeleton", skeleton, "--drafts", drafts); code != 0 {
		t.Fatalf("assemble: code %d stderr %q", code, stderr)
	}
	b, err := os.ReadFile(store.Dir{Root: dir}.Plan())
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Tasks []struct {
			ID    string `json:"id"`
			Scope struct {
				TestOnly []string `json:"test_only"`
			} `json:"scope"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"A1": {"alpha/*_test.go"}, "B1": {}}
	for _, tk := range saved.Tasks {
		if !reflect.DeepEqual(tk.Scope.TestOnly, want[tk.ID]) {
			t.Errorf("%s scope.test_only in plan.json = %#v, want %#v", tk.ID, tk.Scope.TestOnly, want[tk.ID])
		}
	}
	if out, _, code := runGang(t, &Ctx{Dir: dir}, "lint"); code != 0 {
		t.Errorf("lint: code %d out %q", code, out)
	}
}

func TestPlanAssembleRefusals(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	t.Run("duplicate id", func(t *testing.T) {
		skeleton, drafts := assembleInputs(t, t.TempDir())
		writeDraft(t, filepath.Join(drafts, "c.json"), testPlan().Tasks[0])
		_, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "assemble", "--skeleton", skeleton, "--drafts", drafts)
		if code != exit.User || !strings.Contains(stderr, "task A1 is in ") || !strings.Contains(stderr, "a.json and ") {
			t.Errorf("code %d stderr %q", code, stderr)
		}
	})
	t.Run("unknown field in a draft", func(t *testing.T) {
		skeleton, drafts := assembleInputs(t, t.TempDir())
		if err := os.WriteFile(filepath.Join(drafts, "c.json"), []byte(`{"tasks":[{"id":"C1","owner":"x"}]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := runGang(t, &Ctx{Dir: dir}, "plan", "assemble", "--skeleton", skeleton, "--drafts", drafts); code != exit.User || !strings.Contains(stderr, `unknown field "owner"`) {
			t.Errorf("code %d stderr %q", code, stderr)
		}
	})
	t.Run("skeleton with tasks", func(t *testing.T) {
		_, drafts := assembleInputs(t, t.TempDir())
		full := filepath.Join(t.TempDir(), "full.json")
		if err := plan.Save(full, testPlan()); err != nil {
			t.Fatal(err)
		}
		if _, _, code := runGang(t, &Ctx{Dir: dir}, "plan", "assemble", "--skeleton", full, "--drafts", drafts); code != exit.User {
			t.Errorf("code %d, want 1", code)
		}
	})
	t.Run("no drafts", func(t *testing.T) {
		skeleton, _ := assembleInputs(t, t.TempDir())
		if _, _, code := runGang(t, &Ctx{Dir: dir}, "plan", "assemble", "--skeleton", skeleton, "--drafts", t.TempDir()); code != exit.User {
			t.Errorf("code %d, want 1", code)
		}
	})
	t.Run("no skeleton at the default path", func(t *testing.T) {
		_, drafts := assembleInputs(t, t.TempDir())
		if _, _, code := runGang(t, &Ctx{Dir: dir}, "plan", "assemble", "--drafts", drafts); code != exit.User {
			t.Errorf("code %d, want 1", code)
		}
	})
	t.Run("positional argument", func(t *testing.T) {
		if _, _, code := runGang(t, &Ctx{Dir: dir}, "plan", "assemble", "extra"); code != exit.Usage {
			t.Errorf("code %d, want 64", code)
		}
	})
}
