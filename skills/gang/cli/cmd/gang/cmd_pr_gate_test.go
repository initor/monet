package main

import (
	"os"
	"strings"
	"testing"

	"gang/internal/plan"
	"gang/internal/prq"
	"gang/internal/store"
	"gang/internal/testkit"
)

func prGateRun(t *testing.T, openHead, checks string) (*Ctx, string) {
	c, _, ghDir := prRun(t, func(p *plan.Plan) { p.Repo.Checks = checks })
	lease(t, c, "me")
	_ = prq.Put(store.Dir{Root: c.Dir}, prq.Row{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: "h1", State: prq.StateOpen}) // a failed put fails every gate below
	testkit.Answer(t, ghDir, "pr-view.json", `{"state":"OPEN","headRefOid":"`+openHead+`","statusCheckRollup":[]}`)
	testkit.Answer(t, ghDir, "status", "{}")
	return c, ghDir
}

func gateJob(t *testing.T, c *Ctx, id, head, state string) {
	dir := store.Dir{Root: c.Dir}.Run("jobs", id)
	if os.MkdirAll(dir, 0o755) != nil || os.WriteFile(dir+"/job.json", []byte(`{"id":"`+id+`","task":"T1","kind":"pr","head":"`+head+`"}`), 0o644) != nil || os.WriteFile(dir+"/status.json", []byte(`{"id":"`+id+`","state":"`+state+`"}`), 0o644) != nil {
		t.Fatal("write job", id)
	}
}

func TestPRGatePostsTheJobResultOnItsHead(t *testing.T) {
	c, ghDir := prGateRun(t, "h1", plan.ChecksGate)
	for _, tc := range [][]string{{"pr-T1-1", "h1", "failed", "gang/gate failure on h1\n"}, {"pr-T1-2", "h1", "passed", "gang/gate success on h1\n"}, {"pr-T1-3", "h0", "passed", ""}} {
		gateJob(t, c, tc[0], tc[1], tc[2])
		if out, errOut, code := runGang(t, c, "pr", "gate", "T1", tc[0], "--holder", "me"); out != tc[3] || code != map[bool]int{true: 2}[tc[3] == ""] {
			t.Fatalf("%s: exit %d, stdout %q, stderr %q, want %q", tc[0], code, out, errOut, tc[3])
		}
	}
	calls := strings.Join(testkit.Calls(t, ghDir), "\n")
	if f, s := strings.Index(calls, "repos/acme/widget/statuses/h1 -f state=failure"), strings.Index(calls, "repos/acme/widget/statuses/h1 -f state=success"); f < 0 || s < f || strings.Count(calls, "/statuses/") != 2 {
		t.Fatalf("gh calls %q, want a failure then a success post on h1 and no third", calls)
	}
}

func TestPRGateRefusals(t *testing.T) {
	step := func(c *Ctx, ghDir, holder, want string, code int) {
		if out, errOut, got := runGang(t, c, "pr", "gate", "T1", "pr-T1-1", "--holder", holder); got != code || !strings.Contains(out+errOut, want) || strings.Contains(strings.Join(testkit.Calls(t, ghDir), " "), "/statuses/") {
			t.Fatalf("holder %s: exit %d, %q, %q, want exit %d, %q, no post", holder, got, out, errOut, code, want)
		}
	}
	c, ghDir := prGateRun(t, "h2", plan.ChecksGate) // GitHub shows h2: the head moved
	gateJob(t, c, "pr-T1-1", "h1", "passed")
	step(c, ghDir, "other", "not held by other", 4)
	step(c, ghDir, "me", "head moved", 2)
	c, ghDir = prGateRun(t, "h1", plan.ChecksGitHub)
	step(c, ghDir, "me", "only for repo.checks gate", 2)
}
