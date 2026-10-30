package gh

import (
	"reflect"
	"strings"
	"testing"

	"gang/internal/testkit"
)

func TestPRStateMapsCheckRunsAndStatusContexts(t *testing.T) {
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "pr-view.json", `{"state":"OPEN","mergeable":"MERGEABLE","headRefOid":"abc123",
"statusCheckRollup":[
 {"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"SUCCESS"},
 {"__typename":"CheckRun","name":"e2e","status":"IN_PROGRESS","conclusion":""},
 {"__typename":"StatusContext","context":"ci/lint","state":"FAILURE"},
 {"__typename":"StatusContext","context":"ci/docs","state":"PENDING"}]}`)
	got, err := PRState("github.com/acme/widget", 7)
	if err != nil {
		t.Fatal(err)
	}
	want := State{State: "OPEN", Mergeable: "MERGEABLE", HeadSHA: "abc123", Checks: []Check{
		{"build", "COMPLETED", "SUCCESS"},
		{"e2e", "IN_PROGRESS", ""},
		{"ci/lint", "COMPLETED", "FAILURE"},
		{"ci/docs", "PENDING", "PENDING"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	calls := testkit.Calls(t, state)
	if len(calls) != 1 || calls[0] != "pr view 7 --repo github.com/acme/widget --json state,mergeable,headRefOid,statusCheckRollup" {
		t.Fatalf("calls = %q", calls)
	}
}

func TestPRStateReportsGHErrors(t *testing.T) {
	testkit.FakeGH(t) // no pr-view.json answer: the fake exits 1
	if _, err := PRState("github.com/acme/widget", 7); err == nil || !strings.Contains(err.Error(), "gh pr view 7") {
		t.Fatalf("err = %v", err)
	}
}

func TestPostStatusNamesTheHostAndTheRepo(t *testing.T) {
	state, _ := testkit.FakeGH(t)
	if PostStatus("acme/widget", "abc", "failure", "gang/gate", "red") == nil { // no status answer yet: the fake exits 1
		t.Fatal("want an error without a status answer")
	}
	testkit.Answer(t, state, "status", "{}")
	for remote, host := range map[string]string{"github.com/acme/widget": "github.com", "acme/widget": "github.com", "ghe.example.com/acme/widget": "ghe.example.com"} {
		err := PostStatus(remote, "abc", "success", "gang/gate", "gang gate pr-T1-1 passed")
		want := "api --hostname " + host + " --method POST repos/acme/widget/statuses/abc -f state=success -f context=gang/gate -f description=gang gate pr-T1-1 passed"
		if calls := testkit.Calls(t, state); err != nil || calls[len(calls)-1] != want {
			t.Errorf("%s: err = %v, calls = %q, want last %q", remote, err, calls, want)
		}
	}
}

func TestEveryRemoteFormReachesGHNormalized(t *testing.T) {
	state, _ := testkit.FakeGH(t)
	testkit.Answer(t, state, "status", "{}")
	if RepoArgs("") != nil {
		t.Fatal("an empty remote must give no --repo flag")
	}
	for _, remote := range []string{"https://github.com/acme/widget.git", "git@github.com:acme/widget.git", "github.com/acme/widget/", "ssh://git@github.com/acme/widget", "acme/widget"} {
		want := "github.com/acme/widget"
		if remote == "acme/widget" {
			want = remote
		}
		if got := RepoArgs(remote); !reflect.DeepEqual(got, []string{"--repo", want}) {
			t.Errorf("RepoArgs(%q) = %q, want --repo %s", remote, got, want)
		}
		err := PostStatus(remote, "abc", "success", "gang/gate", "d")
		wantCall := "api --hostname github.com --method POST repos/acme/widget/statuses/abc -f state=success -f context=gang/gate -f description=d"
		if calls := testkit.Calls(t, state); err != nil || calls[len(calls)-1] != wantCall {
			t.Errorf("PostStatus(%q): err = %v, last call = %q", remote, err, calls[len(calls)-1])
		}
	}
}

func TestClassify(t *testing.T) {
	ok := Check{"build", "COMPLETED", "SUCCESS"}
	cases := []struct {
		name       string
		s          State
		green, red bool
		failed     []string
	}{
		{"zero checks is never green", State{Mergeable: "MERGEABLE"}, false, false, nil},
		{"all passing and mergeable", State{Mergeable: "MERGEABLE", Checks: []Check{ok, {"lint", "COMPLETED", "NEUTRAL"}, {"docs", "COMPLETED", "SKIPPED"}}}, true, false, nil},
		{"passing but conflicting", State{Mergeable: "CONFLICTING", Checks: []Check{ok}}, false, false, nil},
		{"passing but mergeable unknown", State{Mergeable: "UNKNOWN", Checks: []Check{ok}}, false, false, nil},
		{"pending blocks green", State{Mergeable: "MERGEABLE", Checks: []Check{ok, {"e2e", "IN_PROGRESS", ""}}}, false, false, nil},
		{"pending status context", State{Mergeable: "MERGEABLE", Checks: []Check{ok, {"ci", "PENDING", "PENDING"}}}, false, false, nil},
		{"failure is red even with pending", State{Mergeable: "MERGEABLE", Checks: []Check{{"e2e", "IN_PROGRESS", ""}, {"unit", "COMPLETED", "FAILURE"}}}, false, true, []string{"unit"}},
		{"every failing conclusion, sorted and unique", State{Checks: []Check{
			{"z", "COMPLETED", "TIMED_OUT"}, {"a", "COMPLETED", "ERROR"}, {"m", "COMPLETED", "CANCELLED"},
			{"b", "COMPLETED", "ACTION_REQUIRED"}, {"a", "COMPLETED", "FAILURE"}, {"s", "COMPLETED", "STARTUP_FAILURE"}, ok,
		}}, false, true, []string{"a", "b", "m", "s", "z"}},
	}
	for _, c := range cases {
		green, red, failed := Classify(c.s)
		if green != c.green || red != c.red || !reflect.DeepEqual(failed, c.failed) {
			t.Errorf("%s: got (%v, %v, %v), want (%v, %v, %v)", c.name, green, red, failed, c.green, c.red, c.failed)
		}
	}
}
