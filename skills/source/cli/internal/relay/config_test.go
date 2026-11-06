package relay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// putConfig writes body at ConfigPath(dir).
func putConfig(t *testing.T, dir, body string) {
	t.Helper()
	p := ConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A number field counts only when it decodes as a JSON number above 0, else the hook default applies.
func TestReadConfigNumbers(t *testing.T) {
	values := []string{"", "0", "-5", "0.5", "50", "2e6", `"70"`, "true", "null", "[]", "{}"}
	for _, key := range []string{"threshold_pct", "window_tokens"} {
		for _, v := range values {
			dir, body := t.TempDir(), "{}"
			if v != "" {
				body = `{"` + key + `": ` + v + `}`
			}
			putConfig(t, dir, body)
			want := map[string]float64{"threshold_pct": 80, "window_tokens": 1000000}
			var x any
			if json.Unmarshal([]byte(v), &x) == nil {
				if f, ok := x.(float64); ok && f > 0 {
					want[key] = f
				}
			}
			cfg, found, err := ReadConfig(dir)
			if err != nil || !found || cfg.Threshold != want["threshold_pct"] || cfg.Window != want["window_tokens"] {
				t.Errorf("%s=%s: %+v, found %v, err %v", key, v, cfg, found, err)
			}
		}
	}
}

// The procedure resolves against the folder, with the default for a value that is absent, empty, or not a string.
func TestReadConfigProcedure(t *testing.T) {
	dir := t.TempDir()
	if got, want := ConfigPath(dir), dir+"/.claude/handoff-relay.json"; got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
	def := dir + "/.planning/handoff-relay.md"
	for _, c := range []struct{ body, want string }{
		{`{}`, def}, {`{"procedure": 5}`, def}, {`{"procedure": ""}`, def},
		{`{"procedure": "x/r.md"}`, dir + "/x/r.md"}, {`{"procedure": "/abs/r.md"}`, "/abs/r.md"},
	} {
		putConfig(t, dir, c.body)
		cfg, found, err := ReadConfig(dir)
		if err != nil || !found || cfg.Procedure != c.want {
			t.Errorf("%s: %+v, found %v, err %v, want %q", c.body, cfg, found, err, c.want)
		}
	}
	empty := t.TempDir()
	if cfg, found, err := ReadConfig(empty); err != nil || found || cfg != (Config{80, 1000000, empty + "/.planning/handoff-relay.md"}) {
		t.Errorf("no file: %+v, found %v, err %v", cfg, found, err)
	}
	for _, body := range []string{"{", "[]", "5"} {
		putConfig(t, dir, body)
		if _, _, err := ReadConfig(dir); err == nil || !strings.Contains(err.Error(), ConfigPath(dir)) {
			t.Errorf("%s: err %v, want one that names %s", body, err, ConfigPath(dir))
		}
	}
}

// WriteConfig writes only the procedure, so a read gives the hook defaults and that procedure.
func TestWriteConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := WriteConfig(dir, "/r/relay/brain.md"); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(ConfigPath(dir)); err != nil || string(b) != "{\n  \"procedure\": \"/r/relay/brain.md\"\n}\n" {
		t.Errorf("file = %q, err %v", b, err)
	}
	if cfg, found, err := ReadConfig(dir); err != nil || !found || cfg != (Config{80, 1000000, "/r/relay/brain.md"}) {
		t.Errorf("read back: %+v, found %v, err %v", cfg, found, err)
	}
}
