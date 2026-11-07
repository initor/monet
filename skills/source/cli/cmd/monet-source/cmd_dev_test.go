package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/lock"
	"source/internal/relay"
	"source/internal/state"
	"source/internal/store"
	"source/internal/testkit"
)

// devStep is one dev call at testNow+at and the lease, Dev, and LastBeat (-1 for none) after it.
type devStep struct {
	args, stderr, dev string
	code              int
	at, beat          time.Duration
	lease             time.Time
}

// runDev runs steps in order on one run made by init, and returns the run. No step prints to stdout.
func runDev(t *testing.T, steps []devStep) string {
	run := newRun(t, "demo")
	if _, e, code := runSource(t, &Ctx{Run: run}, "init", "--slug", "demo", "--holder", "h1"); code != exit.OK {
		t.Fatalf("init: code %d, stderr %q", code, e)
	}
	for _, st := range steps {
		out, e, code := runSource(t, &Ctx{Run: run, Now: func() time.Time { return testNow.Add(st.at) }}, strings.Fields("dev "+st.args)...)
		if code != st.code || out != "" || (st.stderr == "") != (e == "") || !strings.Contains(e, st.stderr) {
			t.Errorf("%q: code %d, stdout %q, stderr %q, want %d with %q", st.args, code, out, e, st.code, st.stderr)
		}
		checkLease(t, run, "dev", st.lease)
		s, err := state.Load(store.Dir{Root: run})
		if err != nil || s.Dev != st.dev || (s.LastBeat == nil) != (st.beat < 0) || (s.LastBeat != nil && !s.LastBeat.Equal(testNow.Add(st.beat))) {
			t.Errorf("%q: dev %q, last beat %v, err %v; want %q at +%v", st.args, s.Dev, s.LastBeat, err, st.dev, st.beat)
		}
	}
	return run
}

// hello takes the dev lease. Another live holder is exit 4, a bad call exit 64.
func TestDevHello(t *testing.T) {
	runDev(t, []devStep{
		{"hello --holder d1", "", "d1", exit.OK, 0, 0, testNow.Add(12 * time.Hour)},
		{"hello --holder d2", "lock held: dev by d1", "d1", exit.Held, time.Hour, 0, testNow.Add(12 * time.Hour)},
		{"hello", "usage: monet-source dev hello|beat --holder <session>", "d1", exit.Usage, time.Hour, 0, testNow.Add(12 * time.Hour)},
		{"hello --holder d2 x", "usage: monet-source dev hello|beat --holder <session>", "d1", exit.Usage, time.Hour, 0, testNow.Add(12 * time.Hour)},
		{"hello --holder d2", "", "d2", exit.OK, 13 * time.Hour, 13 * time.Hour, testNow.Add(25 * time.Hour)},
	})
}

// beat renews the lease of its holder. No lease or another holder is exit 4.
func TestDevBeat(t *testing.T) {
	runDev(t, []devStep{
		{"beat --holder d1", "lock held: dev is not held by d1", "", exit.Held, 0, -1, time.Time{}},
		{"hello --holder d1", "", "d1", exit.OK, 0, 0, testNow.Add(12 * time.Hour)},
		{"beat --holder d1", "", "d1", exit.OK, time.Hour, time.Hour, testNow.Add(13 * time.Hour)},
		{"beat --holder d2", "lock held: dev is not held by d2", "d1", exit.Held, 2 * time.Hour, time.Hour, testNow.Add(13 * time.Hour)},
	})
}

// start needs a seal (2) and a skill dir (64), then renews brain (4 for another holder), makes dev/,
// writes the kickoff atomically at a UTC stamp, and prints its start command. No refusal writes a
// kickoff, a failed write prints no command, and no call runs the command.
func TestDevStart(t *testing.T) {
	old := skillDir
	t.Cleanup(func() { skillDir = old })
	skillDir = ""
	bin := t.TempDir() // a fake claude first on PATH marks any run of the printed command
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n: > \"$0.ran\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	run := newSealableRun(t)
	t.Setenv("SOURCE_SKILL", "") // newRun set it for init
	s1, s2 := testkit.Skill(t), testkit.Skill(t)
	d, c := store.Dir{Root: run}, &Ctx{Run: run, SkillDir: s1}
	if out, e, code := runSource(t, c, "dev", "start", "--holder", "h1"); code != exit.Refused || out != "" || e != "monet-source: dev start: seal the charter first\n" {
		t.Fatalf("before the seal: code %d, stdout %q, stderr %q", code, out, e)
	}
	early := func() time.Time { return testNow.Add(-time.Hour) }
	if _, e, code := runSource(t, &Ctx{Run: run, Now: early}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("seal: code %d, stderr %q", code, e)
	}
	usage := "monet-source: usage: monet-source dev start --holder <session>\n"
	for _, tc := range []struct {
		c      *Ctx
		args   string
		code   int
		stderr string
	}{
		{&Ctx{Run: run}, "--holder h1", exit.Usage, "monet-source: no skill dir: pass --skill or set SOURCE_SKILL\n"},
		{c, "--holder h2", exit.Held, "monet-source: lock held: brain is not held by h2\n"},
		{c, "", exit.Usage, "monet-source: --holder <session> is required: the brain lease holder\n"},
		{c, "--holder h1 x", exit.Usage, usage},
		{c, "--bogus", exit.Usage, usage},
	} {
		if out, e, code := runSource(t, tc.c, strings.Fields("dev start "+tc.args)...); code != tc.code || out != "" || e != tc.stderr {
			t.Errorf("%q: code %d, stdout %q, stderr %q, want %d with %q", tc.args, code, out, e, tc.code, tc.stderr)
		}
	}
	checkLease(t, run, "brain", testNow.Add(11*time.Hour)) // no refusal renewed brain
	if ents, err := os.ReadDir(d.Path("handoff")); err != nil || len(ents) != 0 {
		t.Fatalf("handoff/ holds %v, %v after the refusals; want nothing", ents, err)
	}
	kick0 := d.Path("handoff", "dev-20261101T120000Z.md")
	if err := os.MkdirAll(filepath.Join(kick0, "x"), 0o755); err != nil { // a folder at the kickoff path fails the write
		t.Fatal(err)
	}
	if out, e, code := runSource(t, c, "dev", "start", "--holder", "h1"); code != exit.User || out != "" || e == "" {
		t.Fatalf("failed write: code %d, stdout %q, stderr %q; want %d, no stdout", code, out, e, exit.User)
	}
	// A read-only old kickoff is replaced, which only an atomic write (rename) can do.
	if err := os.RemoveAll(kick0); err != nil || os.WriteFile(kick0, []byte("old\n"), 0o444) != nil || os.RemoveAll(d.Path("dev")) != nil {
		t.Fatalf("setup of the old kickoff and dev/ failed: %v", err)
	}
	pst := func() time.Time { return testNow.Add(time.Minute).In(time.FixedZone("PST", -8*3600)) }
	for _, tc := range []struct {
		now                func() time.Time
		stamp, slug, skill string
	}{{nil, "20261101T120000Z", "demo", s1}, {pst, "20261101T120100Z", "other", s2}} { // the first remakes dev/, the second keeps it
		if err := state.Update(d, func(s *state.State) error { s.Slug = tc.slug; return nil }); err != nil {
			t.Fatal(err)
		}
		if tc.skill == s2 { // an old config in dev/ is replaced
			if err := os.WriteFile(relay.ConfigPath(d.Path("dev")), []byte(`{"procedure": "x"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		kick := d.Path("handoff", "dev-"+tc.stamp+".md")
		out, e, code := runSource(t, &Ctx{Run: run, SkillDir: tc.skill, Now: tc.now}, "dev", "start", "--holder", "h1")
		got, err := os.ReadFile(kick)
		if fi, serr := os.Stat(d.Path("dev")); code != exit.OK || e != "" || out != devStartCommand(tc.slug, run, kick)+"\n" || err != nil || string(got) != devKickoff(tc.slug, run, tc.skill) || serr != nil || !fi.IsDir() {
			t.Fatalf("%s: code %d, stdout %q, stderr %q, kickoff %q (err %v), dev/ err %v", tc.stamp, code, out, e, got, err, serr)
		}
		steps, err := os.ReadFile(d.Path("relay", "dev.md"))
		if cfg, found, cerr := relay.ReadConfig(d.Path("dev")); err != nil || string(steps) != testkit.RelayText || cerr != nil || !found || cfg.Procedure != d.Path("relay", "dev.md") {
			t.Fatalf("%s: relay/dev.md %q (err %v), config %+v found %v (err %v)", tc.stamp, steps, err, cfg, found, cerr)
		}
	}
	checkLease(t, run, "brain", testNow.Add(time.Minute+12*time.Hour))
	if _, err := os.Stat(filepath.Join(bin, "claude.ran")); !os.IsNotExist(err) {
		t.Fatalf("dev start ran its command (mark stat: %v)", err)
	}
}

// takeover moves the live dev lease of --from to --holder and records DEV_RELAYED.
// A lease that --from does not hold live is exit 4, a bad call exit 64.
func TestDevTakeover(t *testing.T) {
	const usage = "usage: monet-source dev takeover --from <old session> --holder <session>"
	at12 := testNow.Add(12 * time.Hour)
	run := runDev(t, []devStep{
		{"hello --holder d1", "", "d1", exit.OK, 0, 0, at12},
		{"takeover --from d1 --holder d2", "", "d2", exit.OK, 0, 0, at12},
		{"takeover --from d1 --holder d3", "lock held: dev is not held by d1", "d2", exit.Held, time.Hour, 0, at12},
		{"takeover --from d2 --holder d3", "lock held: dev is not held by d2", "d2", exit.Held, 13 * time.Hour, 0, at12},
		{"takeover --holder d3", usage, "d2", exit.Usage, time.Hour, 0, at12},
		{"takeover --from d2", usage, "d2", exit.Usage, time.Hour, 0, at12},
		{"takeover --from d2 --holder d3 x", usage, "d2", exit.Usage, time.Hour, 0, at12},
		{"takeover --from d2 --holder d3 --bogus", usage, "d2", exit.Usage, time.Hour, 0, at12},
	})
	l, _, lerr := lock.Read(store.Dir{Root: run}.Locks(), "dev")
	got, _, err := events.For(store.Dir{Root: run}, "brain", 0)
	if lerr != nil || l.Holder != "d2" || err != nil || !reflect.DeepEqual(got, []string{"DEV_RELAYED d1 d2"}) {
		t.Errorf("lease %+v (%v), brain events %q (%v); want d2 and [DEV_RELAYED d1 d2]", l, lerr, got, err)
	}
	// A takeover at +1h sets the lease, Dev, and LastBeat from that clock.
	runDev(t, []devStep{
		{"hello --holder d1", "", "d1", exit.OK, 0, 0, at12},
		{"takeover --from d1 --holder d2", "", "d2", exit.OK, time.Hour, time.Hour, testNow.Add(13 * time.Hour)},
	})
}

// start with a skill dir that has no reference/relay.md is exit 2 after the seal check, and it writes no kickoff and no dev config.
func TestDevStartNeedsRelaySteps(t *testing.T) {
	run, skill := newSealableRun(t), realTempDir(t)
	if _, e, code := runSource(t, &Ctx{Run: run, SkillDir: skill}, "dev", "start", "--holder", "h1"); e != "monet-source: dev start: seal the charter first\n" {
		t.Fatalf("before the seal: code %d, stderr %q", code, e)
	}
	if _, e, code := runSource(t, &Ctx{Run: run}, "charter", "seal", "--words", "w1", "--holder", "h1"); code != exit.OK {
		t.Fatalf("seal: code %d, stderr %q", code, e)
	}
	d := store.Dir{Root: run}
	want := "monet-source: dev start: " + filepath.Join(skill, "reference", "relay.md") + ": no such file or directory\n"
	if out, e, code := runSource(t, &Ctx{Run: run, SkillDir: skill}, "dev", "start", "--holder", "h1"); code != exit.Refused || out != "" || e != want {
		t.Fatalf("code %d, stdout %q, stderr %q; want %d with %q", code, out, e, exit.Refused, want)
	}
	if got := listRun(t, d.Path("handoff")); got != "" {
		t.Errorf("handoff/ holds %q; want nothing", got)
	}
	if _, err := os.Lstat(d.Path("dev", ".claude")); !os.IsNotExist(err) {
		t.Errorf("dev/.claude lstat: %v; want it absent", err)
	}
}
