// Package watch computes the gang watch events (Contract 7).
package watch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gang/internal/gh"
	"gang/internal/gitx"
	"gang/internal/plan"
	"gang/internal/store"
	"gang/internal/wave"
)

// state is run/watch.json. ActiveAt starts a STALL episode: STALL de-dupe
// only counts events.tsv rows from the current episode.
type state struct {
	Tip      string `json:"tip"`
	Newest   string `json:"newest"`
	ActiveAt string `json:"active_at"`
}

type event struct {
	key string
	at  time.Time
}

var eventsHeader = []string{"key", "at"}

// Poll runs one watch pass and returns the new event lines. A gh error is
// returned beside any events found. Only one on a wave PR blocks WAVE_READY.
// Without repo.gh_account it never runs gh (Contract 0.9). The events.tsv key
// of a PR event is its line plus " @<head sha>", so a new push can raise the
// same event again (Contract 0.8).
func Poll(repo string, d store.Dir, p *plan.Plan, now time.Time) ([]string, error) {
	st, err := loadState(d)
	if err != nil {
		return nil, err
	}
	seen, err := readEvents(d)
	if err != nil {
		return nil, err
	}
	var lines []string
	var keys []string
	emit := func(line, key string, since time.Time) {
		k := store.SanitizeCell(key)
		for _, e := range seen {
			if e.key == k && !e.at.Before(since) {
				return
			}
		}
		lines, keys = append(lines, line), append(keys, key)
	}

	var ghErr error
	rows, err := wave.Read(d)
	if err != nil {
		return nil, err
	}
	// Poll every open or closed row: a closed PR can be reopened. Never a merged one.
	prs := map[int]gh.State{}
	if p.Repo.GHAccount != "" {
		for _, r := range rows {
			if r.State == wave.StateMerged {
				continue
			}
			ps, err := gh.PRState(p.Repo.Remote, r.PR)
			if err != nil {
				ghErr = errors.Join(ghErr, err)
				continue
			}
			if _, err := wave.Settle(d, r, ps); err != nil {
				return nil, err
			}
			prs[r.PR] = ps
		}
		if rows, err = wave.Read(d); err != nil {
			return nil, err
		}
	}
	var open *wave.Row
	for i := range rows {
		if rows[i].State == wave.StateOpen {
			open = &rows[i]
		}
	}
	if open != nil {
		if ps, ok := prs[open.PR]; ok {
			green, red, failed := gh.Classify(ps)
			if green {
				l := fmt.Sprintf("PR_GREEN %d %d", open.Wave, open.PR)
				emit(l, l+" @"+ps.HeadSHA, time.Time{})
			}
			if red {
				l := fmt.Sprintf("PR_RED %d %d %s", open.Wave, open.PR, strings.Join(failed, ","))
				emit(l, l+" @"+ps.HeadSHA, time.Time{})
			}
		}
	}
	if open == nil && ghErr == nil {
		done, err := wave.Done(repo, p)
		if err != nil {
			return nil, err
		}
		if k, ok := readyWave(p, rows, done); ok {
			l := fmt.Sprintf("WAVE_READY %d", k)
			emit(l, l, time.Time{})
		}
	}

	soft, err := queue(repo, d, p, now, emit)
	if err != nil {
		return nil, err
	}
	ghErr = errors.Join(ghErr, soft)

	tip := ""
	if s, err := gitx.RevParse(repo, "refs/heads/"+p.Repo.Integration); err == nil {
		tip = strings.TrimSpace(s)
	}
	newest, err := newestMtime(d.Run("tasks"), d.Run("jobs"))
	if err != nil {
		return nil, err
	}
	if st.ActiveAt == "" || tip != st.Tip || newest != st.Newest {
		st.Tip, st.Newest, st.ActiveAt = tip, newest, stamp(now)
	} else if limit := time.Duration(p.Limits.StallMin) * time.Minute; limit > 0 {
		active := parse(st.ActiveAt)
		if idle := now.Sub(active); idle >= limit {
			l := fmt.Sprintf("STALL %d", int(idle/limit)*p.Limits.StallMin)
			emit(l, l, active)
		}
	}

	if err := saveState(d, st); err != nil {
		return nil, err
	}
	for _, k := range keys {
		if err := store.AppendTSV(d.Run("events.tsv"), eventsHeader, []string{k, stamp(now)}); err != nil {
			return nil, err
		}
	}
	return lines, ghErr
}

// readyWave returns the highest wave k above every wave in waves.tsv such that
// every task with wave <= k is done (Contract 0.7).
func readyWave(p *plan.Plan, rows []wave.Row, done map[string]bool) (int, bool) {
	last := 0
	for _, r := range rows {
		last = max(last, r.Wave)
	}
	waves := map[int]bool{}
	for _, t := range p.Tasks {
		if t.Wave > last {
			waves[t.Wave] = true
		}
	}
	ks := make([]int, 0, len(waves))
	for k := range waves {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	best, ok := 0, false
	for _, k := range ks {
		for _, t := range p.Tasks {
			if t.Wave <= k && !done[t.ID] {
				return best, ok
			}
		}
		best, ok = k, true
	}
	return best, ok
}

// newestMtime is the newest modification time under the roots, as RFC 3339
// with nanoseconds, or "" when none exist.
func newestMtime(roots ...string) (string, error) {
	var newest time.Time
	for _, root := range roots {
		err := filepath.WalkDir(root, func(_ string, e fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			info, err := e.Info()
			if err != nil {
				return err
			}
			if info.ModTime().After(newest) {
				newest = info.ModTime()
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	if newest.IsZero() {
		return "", nil
	}
	return stamp(newest), nil
}

func readEvents(d store.Dir) ([]event, error) {
	recs, err := store.ReadTSV(d.Run("events.tsv"), eventsHeader)
	if err != nil {
		return nil, err
	}
	out := make([]event, len(recs))
	for i, r := range recs {
		out[i] = event{key: r[0], at: parse(r[1])}
	}
	return out, nil
}

func loadState(d store.Dir) (state, error) {
	var st state
	b, err := os.ReadFile(d.Run("watch.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	return st, json.Unmarshal(b, &st)
}

func saveState(d store.Dir, st state) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return store.WriteFileAtomic(d.Run("watch.json"), append(b, '\n'), 0o644)
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parse(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}
