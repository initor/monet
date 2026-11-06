package transcript

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"slices"
)

// ErrNoUsage is the error of LastUsage when no record of the transcript counts.
var ErrNoUsage = errors.New("no assistant record with usage")

// tailSteps are the sizes of the tail windows that LastUsage reads before the whole file.
var tailSteps = []int64{2 << 20, 8 << 20, 32 << 20}

// Tokens returns the sum of the input, cache creation, and cache read tokens.
func (u Usage) Tokens() int { return u.Input + u.CacheCreation + u.CacheRead }

// LastUsage returns the Tokens of the newest non-sidechain assistant record of path whose
// sum is above 0. It reads the tail windows of tailSteps and then the whole file, and drops
// the first line of each window that starts after offset 0, since the cut may split it.
func LastUsage(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	size := info.Size()
	for _, step := range append(slices.Clone(tailSteps), size) {
		n := min(step, size)
		buf := make([]byte, n)
		if _, err := f.ReadAt(buf, size-n); err != nil {
			return 0, err
		}
		lines := bytes.Split(buf, []byte("\n"))
		if n < size {
			lines = lines[1:]
		}
		for i := len(lines) - 1; i >= 0; i-- {
			var e Entry
			if json.Unmarshal(lines[i], &e) != nil || e.Type != "assistant" || e.IsSidechain || e.Message == nil || e.Message.Usage == nil {
				continue
			}
			if tokens := e.Message.Usage.Tokens(); tokens > 0 {
				return tokens, nil
			}
		}
		if n == size {
			break
		}
	}
	return 0, ErrNoUsage
}
