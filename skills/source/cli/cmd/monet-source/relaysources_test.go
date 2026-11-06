package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"source/internal/testkit"
	"source/internal/transcript"
)

// relayEnv points the session id s1, the job dir, the hook state dir, and the
// config dir at new temp folders, and returns the three folders.
func relayEnv(t *testing.T) (job, hook, config string) {
	job, hook, config = t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("CLAUDE_CODE_SESSION_ID", "s1")
	t.Setenv("CLAUDE_JOB_DIR", job)
	t.Setenv("CLAUDE_RELAY_STATE_DIR", hook)
	t.Setenv("CLAUDE_CONFIG_DIR", config)
	return job, hook, config
}

// writeText writes text to path, or fails the test.
func writeText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Mode is the first of --mode, the hook state, the job flags, and the
// transcript of the session that holds one, over all 16 subsets. An absent
// source holds an empty mode, or no --permission-mode flag.
func TestRelaySources(t *testing.T) {
	for mask := range 16 {
		t.Run(fmt.Sprintf("%04b", mask), func(t *testing.T) {
			job, hook, config := relayEnv(t)
			var vals [4]string
			for i := range vals {
				if mask&(1<<i) != 0 {
					vals[i] = fmt.Sprintf("m%d", i+1)
				}
			}
			flags := []string{"--effort", "high"}
			if vals[2] != "" {
				flags = append(flags, "--permission-mode", vals[2])
			}
			data, err := json.Marshal(map[string]any{"name": "j", "respawnFlags": flags})
			if err != nil {
				t.Fatal(err)
			}
			writeText(t, filepath.Join(job, "state.json"), string(data))
			writeText(t, filepath.Join(hook, "monet-relay-s1.json"), fmt.Sprintf(`{"permissionMode": %q}`, vals[1]))
			p := testkit.Transcript(t, config, "s1", testkit.Assistant(1, 0, 0), map[string]any{"type": "user", "permissionMode": vals[3]})
			want := ""
			for _, v := range vals {
				if v != "" {
					want = v
					break
				}
			}
			f, err := transcript.Newest(p)
			got := relaySources(vals[0], "")
			if got.Mode != want || !reflect.DeepEqual(got.Flags, flags) || got.Model != f.Model || f.Model == "" || err != nil {
				t.Errorf("relaySources = %+v; want mode %q, flags %q, model %q (err %v)", got, want, flags, f.Model, err)
			}
		})
	}
}

// Name is the first of the job name, the agentName, and the customTitle of
// the transcript that --transcript names, over all 8 subsets. A missing or
// broken source gives none, a missing --transcript does not fall back to the
// transcript of the session, and relaySources never fails.
func TestRelaySourcesName(t *testing.T) {
	for mask := range 8 {
		t.Run(fmt.Sprintf("%03b", mask), func(t *testing.T) {
			job, _, config := relayEnv(t)
			var vals [3]string
			for i, v := range []string{"j", "a", "c"} {
				if mask&(1<<i) != 0 {
					vals[i] = v
				}
			}
			writeText(t, filepath.Join(job, "state.json"), fmt.Sprintf(`{"name": %q}`, vals[0]))
			p := testkit.Transcript(t, filepath.Join(config, "flag"), "other", map[string]any{"agentName": vals[1], "customTitle": vals[2]})
			want := relayInputs{}
			for _, v := range vals {
				if v != "" {
					want.Name = v
					break
				}
			}
			if got := relaySources("", p); !reflect.DeepEqual(got, want) {
				t.Errorf("relaySources = %+v, want %+v", got, want)
			}
		})
	}
	job, _, config := relayEnv(t)
	testkit.Transcript(t, config, "s1", testkit.Typed("u1", "p0", "hi"), map[string]any{"agentName": "a"}, testkit.Assistant(1, 0, 0))
	gone := filepath.Join(config, "gone.jsonl")
	// The first row has no job file; each later row writes its text.
	for _, c := range []struct {
		text string
		want relayInputs
	}{
		{"", relayInputs{}},
		{`{"name": "j", "respawnFlags": ["--effort"`, relayInputs{}},
		{`[]`, relayInputs{}},
		{`null`, relayInputs{}},
		{`{"name": 5, "respawnFlags": ["--effort", "high"]}`, relayInputs{Flags: []string{"--effort", "high"}}},
		{`{"name": "j", "respawnFlags": ["--effort", 1]}`, relayInputs{Name: "j"}},
		{`{"name": "j", "respawnFlags": "--effort"}`, relayInputs{Name: "j"}},
	} {
		if c.text != "" {
			writeText(t, filepath.Join(job, "state.json"), c.text)
		}
		if got := relaySources("", gone); !reflect.DeepEqual(got, c.want) {
			t.Errorf("job %q: relaySources = %+v, want %+v", c.text, got, c.want)
		}
	}
	// With no job dir, the state.json of the working folder is not read.
	t.Setenv("CLAUDE_JOB_DIR", "")
	t.Chdir(job)
	if got := relaySources("", gone); !reflect.DeepEqual(got, relayInputs{}) {
		t.Errorf("no job dir: relaySources = %+v", got)
	}
}
