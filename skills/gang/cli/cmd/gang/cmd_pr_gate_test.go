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
	for _, tc := range []struct {
		job, head, state, out string
		code                  int
	}{{"pr-T1-1", "h1", "failed", "gang/gate failure on h1\n", 0}, {"pr-T1-2", "h1", "passed", "gang/gate success on h1\n", 0}, {"pr-T1-3", "h0", "passed", "", 2}} {
		gateJob(t, c, tc.job, tc.head, tc.state)
		if out, errOut, code := runGang(t, c, "pr", "gate", "T1", tc.job, "--holder", "me"); out != tc.out || code != tc.code || (code != 0 && !strings.Contains(errOut, "did not gate head h1 of T1")) {
			t.Fatalf("%s: exit %d, stdout %q, stderr %q, want exit %d and %q", tc.job, code, out, errOut, tc.code, tc.out)
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
	if calls := testkit.Calls(t, ghDir); len(calls) != 0 {
		t.Fatalf("gh calls %q before the holder check passed, want none", calls)
	}
	step(c, ghDir, "me", "head moved", 2)
	c, ghDir = prGateRun(t, "h1", plan.ChecksGitHub)
	step(c, ghDir, "me", "only for repo.checks gate", 2)
}

// The head write of gate run --pr reaches the post, and a queued task job does not block a PR start.
func TestPRGatePostsARealJobOnItsHead(t *testing.T) {
	useBuiltGang(t)
	c, repo, ghDir := prRun(t, func(p *plan.Plan) { p.Repo.Checks = plan.ChecksGate })
	c.Repo = repo
	lease(t, c, "me")
	head := testkit.Git(t, repo, "rev-parse", "gang-demo")
	d := store.Dir{Root: c.Dir}
	if err := prq.Put(d, prq.Row{Task: "T1", PR: 7, Branch: "gang/demo/T1", HeadSHA: head, State: prq.StateOpen}); err != nil {
		t.Fatal(err)
	}
	testkit.Answer(t, ghDir, "pr-view.json", `{"state":"OPEN","headRefOid":"`+head+`","statusCheckRollup":[]}`)
	testkit.Answer(t, ghDir, "status", "{}")
	if os.MkdirAll(d.Run("jobs", "T1-1"), 0o755) != nil || os.WriteFile(d.Run("jobs", "T1-1", "status.json"), []byte(`{"state":"queued"}`), 0o644) != nil {
		t.Fatal("write the queued task job")
	}
	for _, step := range [][]string{{"gate", "run", "--pr", "T1"}, {"gate", "wait", "pr-T1-1", "--timeout", "60s"}, {"pr", "gate", "T1", "pr-T1-1", "--holder", "me"}} {
		if out, errOut, code := runGang(t, c, step...); code != 0 || (step[0] == "pr" && out != "gang/gate success on "+head[:7]+"\n") {
			t.Fatalf("gang %v: exit %d, stdout %q, stderr %q", step, code, out, errOut)
		}
	}
	if posts := strings.Join(testkit.Calls(t, ghDir), "\n"); strings.Count(posts, "/statuses/") != 1 || !strings.Contains(posts, "/statuses/"+head+" -f state=success") {
		t.Fatalf("gh calls %q, want one success post on %s", posts, head)
	}
}
