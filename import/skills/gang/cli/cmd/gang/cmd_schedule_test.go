package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/plan"
	"gang/internal/store"
)

// testPlan estimates: A1 20+200/15 = 33.3 min, then B1 20+300/15 = 40 min.
// Makespan 73.3, so the range is 58.7-110, printed as 59-110.
func TestScheduleOutput(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	cases := []struct {
		args []string
		want string
	}{
		{nil, "wave 1: A1 B1\nchain: A1 > B1\nrange 59-110 min\n"},
		{[]string{"--waves", "2", "--cap", "1"}, "wave 1: A1\nwave 2: B1\nchain: A1 > B1\nrange 59-110 min\n"},
	}
	for _, c := range cases {
		out, stderr, code := runGang(t, &Ctx{Dir: dir}, append([]string{"schedule"}, c.args...)...)
		if code != 0 || out != c.want {
			t.Errorf("%v: code %d out %q stderr %q, want %q", c.args, code, out, stderr, c.want)
		}
	}
	p, err := plan.Load(store.Dir{Root: dir}.Plan())
	if err != nil {
		t.Fatal(err)
	}
	if p.Tasks[1].Wave != 2 {
		t.Error("schedule without --write changed plan.json")
	}
}

// A Task: A1 commit on the integration branch (Contract 0 rule 1) takes A1
// out of the chain and the range: B1 alone is 40 min, so 32-60. The waves
// still cover both tasks. With both merged no work is left.
func TestScheduleLeavesOutMergedTasks(t *testing.T) {
	repo := newTestRepo(t)
	dir := newTestGangDir(t, repo, testPlan())
	ref := "refs/heads/" + testPlan().Repo.Integration
	gitT(t, repo, "update-ref", ref, gitT(t, repo, "rev-parse", "refs/heads/main"))
	rsEmptyCommit(t, repo, ref, "a1 work\n\nTask: A1")
	out, stderr, code := runGang(t, &Ctx{Dir: dir}, "schedule")
	if want := "wave 1: A1 B1\nchain: B1\nrange 32-60 min\n"; code != 0 || out != want {
		t.Errorf("A1 merged: code %d out %q stderr %q, want %q", code, out, stderr, want)
	}
	rsEmptyCommit(t, repo, ref, "b1 work\n\nTask: B1")
	out, stderr, code = runGang(t, &Ctx{Dir: dir}, "schedule")
	if want := "wave 1: A1 B1\nchain: none\nrange 0-0 min\n"; code != 0 || out != want {
		t.Errorf("all merged: code %d out %q stderr %q, want %q", code, out, stderr, want)
	}
}

// Spec 5.4: schedule uses the measured agent cap, run/cap from preflight,
// else CPUs minus 2, and --cap wins. A1 (33.3 min) and B1 (40 min) run in
// parallel: one agent takes 73.3 min (59-110), two or more take 40 (32-60).
func TestScheduleUsesMeasuredCap(t *testing.T) {
	p := testPlan()
	p.Tasks[1].DependsOn = []string{}
	dir := newTestGangDir(t, newTestRepo(t), p)
	capFile := store.Dir{Root: dir}.Run("cap")
	serial, parallel := "range 59-110 min\n", "range 32-60 min\n"
	want := parallel
	if max(1, runtime.NumCPU()-2) == 1 {
		want = serial
	}
	if out, stderr, code := runGang(t, &Ctx{Dir: dir}, "schedule"); code != 0 || !strings.HasSuffix(out, want) {
		t.Errorf("no run/cap: code %d out %q stderr %q, want %q", code, out, stderr, want)
	}
	cases := []struct {
		capFile string
		args    []string
		want    string
	}{
		{"1\n", nil, serial},
		{"1\n", []string{"--cap", "2"}, parallel},
		{"2\n", nil, parallel},
		{"2\n", []string{"--cap", "1"}, serial},
	}
	for _, c := range cases {
		if err := os.WriteFile(capFile, []byte(c.capFile), 0o644); err != nil {
			t.Fatal(err)
		}
		out, stderr, code := runGang(t, &Ctx{Dir: dir}, append([]string{"schedule"}, c.args...)...)
		if code != 0 || !strings.HasSuffix(out, c.want) {
			t.Errorf("run/cap %q %v: code %d out %q stderr %q, want %q", c.capFile, c.args, code, out, stderr, c.want)
		}
	}
	for _, bad := range []string{"0\n", "-2\n", "four\n", ""} {
		if err := os.WriteFile(capFile, []byte(bad), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, stderr, code := runGang(t, &Ctx{Dir: dir}, "schedule"); code != exit.User || !strings.Contains(stderr, "run/cap") {
			t.Errorf("run/cap %q: code %d stderr %q, want 1 naming run/cap", bad, code, stderr)
		}
	}
}

func TestScheduleWriteSavesWaves(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	_, stderr, code := runGang(t, &Ctx{Dir: dir}, "schedule", "--write")
	if code != 0 || !strings.Contains(stderr, "wrote waves to ") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	p, err := plan.Load(store.Dir{Root: dir}.Plan())
	if err != nil {
		t.Fatal(err)
	}
	if p.Tasks[0].Wave != 1 || p.Tasks[1].Wave != 1 {
		t.Errorf("waves %d %d, want 1 1 (one wave for two tasks)", p.Tasks[0].Wave, p.Tasks[1].Wave)
	}
}

// A1 took 50 actual minutes against 33.3 estimated: scale 1.5, so 88-165.
// F01 is an engine repair task, which run/durations.tsv can hold but the
// plan does not (Contract 0 items 11 and 13).
func TestScheduleCalibrate(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	if err := os.WriteFile(store.Dir{Root: dir}.Run("durations.tsv"), []byte("# pilot\nA1\t50\nF01\t12\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runGang(t, &Ctx{Dir: dir}, "schedule", "--calibrate", "run/durations.tsv")
	if code != 0 || !strings.HasSuffix(out, "range 88-165 min\n") {
		t.Errorf("code %d out %q stderr %q", code, out, stderr)
	}
	if stderr != "calibrate: skipped F01 (not in the plan)\n" {
		t.Errorf("stderr %q", stderr)
	}
	cal := filepath.Join(t.TempDir(), "durations.tsv")
	if err := os.WriteFile(cal, []byte("A1\t50\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, _, code := runGang(t, &Ctx{Dir: dir}, "schedule", "--calibrate", cal); code != 0 || !strings.HasSuffix(out, "range 88-165 min\n") {
		t.Errorf("absolute path: code %d out %q", code, out)
	}
	if _, stderr, code := runGang(t, &Ctx{Dir: dir}, "schedule", "--calibrate", cal, "--write"); code != exit.Usage || !strings.Contains(stderr, "prints only") {
		t.Errorf("--calibrate with --write: code %d stderr %q, want 64", code, stderr)
	}
	for name, content := range map[string]string{"bad line": "A1 50\n", "not a number": "A1\tlong\n", "zero minutes": "A1\t0\n"} {
		if err := os.WriteFile(cal, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, code := runGang(t, &Ctx{Dir: dir}, "schedule", "--calibrate", cal); code != exit.User {
			t.Errorf("%s: code %d, want 1", name, code)
		}
	}
}

func TestScheduleErrors(t *testing.T) {
	p := testPlan()
	p.Tasks[0].MergeAfter = []string{"B1"}
	dir := newTestGangDir(t, newTestRepo(t), p)
	if _, stderr, code := runGang(t, &Ctx{Dir: dir}, "schedule"); code != exit.User || !strings.Contains(stderr, "cycle") {
		t.Errorf("cycle: code %d stderr %q, want 1", code, stderr)
	}
	if _, _, code := runGang(t, &Ctx{Dir: dir}, "schedule", "--cap", "-1"); code != exit.Usage {
		t.Errorf("negative cap: code %d, want 64", code)
	}
}
