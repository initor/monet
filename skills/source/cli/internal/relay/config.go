// Package relay reads and writes the files that the handoff-relay hook
// shares with the engine.
package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"source/internal/store"
)

// Config is the relay config of a folder, with the hook defaults filled in.
type Config struct {
	Threshold, Window float64
	Procedure         string
}

// ConfigPath is the relay config file of folder.
func ConfigPath(folder string) string {
	return filepath.Join(folder, ".claude", "handoff-relay.json")
}

// ReadConfig reads the config of folder loosely. A number counts only when it
// is a JSON number above 0, and the procedure resolves against folder. A
// missing file gives the defaults and found false.
func ReadConfig(folder string) (cfg Config, found bool, err error) {
	cfg = Config{80, 1000000, filepath.Join(folder, ".planning", "handoff-relay.md")}
	p := ConfigPath(folder)
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, false, nil
	} else if err != nil {
		return cfg, false, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return cfg, true, fmt.Errorf("relay config %s: %w", p, err)
	}
	if f, ok := m["threshold_pct"].(float64); ok && f > 0 {
		cfg.Threshold = f
	}
	if f, ok := m["window_tokens"].(float64); ok && f > 0 {
		cfg.Window = f
	}
	if s, ok := m["procedure"].(string); ok && s != "" {
		cfg.Procedure = s
		if !filepath.IsAbs(s) {
			cfg.Procedure = filepath.Join(folder, s)
		}
	}
	return cfg, true, nil
}

// WriteConfig writes a config that holds only procedure, atomically.
func WriteConfig(folder, procedure string) error {
	return store.WriteJSON(ConfigPath(folder), map[string]string{"procedure": procedure})
}
