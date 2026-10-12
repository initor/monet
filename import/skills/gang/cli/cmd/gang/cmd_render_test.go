package main

import (
	"os"
	"strings"
	"testing"

	"gang/internal/exit"
	"gang/internal/render"
	"gang/internal/schedule"
	"gang/internal/store"
)

func TestRenderWritesPlanMD(t *testing.T) {
	p := testPlan()
	dir := newTestGangDir(t, newTestRepo(t), p)
	out, _, code := runGang(t, &Ctx{Dir: dir}, "render")
	path := store.Dir{Root: dir}.PlanMD()
	if code != 0 || out != path+"\n" {
		t.Fatalf("code %d out %q, want %q", code, out, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schedule.Run(p, schedule.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != render.Markdown(p, s) {
		t.Error("plan.md differs from render.Markdown of the plan with the default schedule")
	}
}

func TestRenderCyclicPlanSkipsSchedule(t *testing.T) {
	p := testPlan()
	p.Tasks[0].MergeAfter = []string{"B1"}
	dir := newTestGangDir(t, newTestRepo(t), p)
	_, stderr, code := runGang(t, &Ctx{Dir: dir}, "render")
	if code != 0 || !strings.Contains(stderr, "render without schedule") {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	data, err := os.ReadFile(store.Dir{Root: dir}.PlanMD())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Critical chain") {
		t.Error("a cyclic plan rendered a critical chain")
	}
}

func TestRenderNeedsAPlan(t *testing.T) {
	dir := newTestGangDir(t, newTestRepo(t), testPlan())
	if err := os.Remove(store.Dir{Root: dir}.Plan()); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runGang(t, &Ctx{Dir: dir}, "render"); code != exit.User {
		t.Errorf("code %d, want 1", code)
	}
}
