package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"source/internal/events"
	"source/internal/exit"
	"source/internal/store"
)

func init() { commands["watch"] = cmdWatch }

const watchUsage = "usage: monet-source watch --role brain|dev [--interval 30s] [--once]"

// watchSleep waits between passes. Tests replace it.
var watchSleep = time.Sleep

// watchCursor is <run>/watch/<role>.json: the events row of the next pass.
type watchCursor struct {
	Row int `json:"row"`
}

// cmdWatch is watch --role brain|dev [--interval 30s] [--once]. It makes one
// pass with --once, else a pass after each interval. A failed pass prints its
// error, and each failed pass in a row doubles the delay, up to 8 times.
func cmdWatch(c *Ctx, args []string) error {
	fs := newFlagSet("watch")
	role := fs.String("role", "", "brain or dev")
	interval := fs.Duration("interval", 30*time.Second, "time between passes")
	once := fs.Bool("once", false, "run one pass and exit")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 0 || (*role != "brain" && *role != "dev") {
		return exit.New(exit.Usage, "%s", watchUsage)
	}
	if *interval <= 0 {
		return exit.New(exit.Usage, "watch: --interval must be positive")
	}
	fails := 0
	for {
		err := watchPass(c, *role)
		if *once {
			return err
		}
		if err != nil {
			fails++
			fmt.Fprintf(c.Stderr, "monet-source watch: %v\n", err)
		} else {
			fails = 0
		}
		watchSleep(watchDelay(*interval, fails))
	}
}

// watchPass records the due time events (recordDue), prints each event line of role
// after its cursor and flushes it, then stores the new cursor. A failed write or flush
// keeps the old cursor, so the next pass prints that line again instead of skipping it.
func watchPass(c *Ctx, role string) error {
	d := store.Dir{Root: c.Run}
	if err := recordDue(c, d); err != nil {
		return err
	}
	path := filepath.Join(d.Watch(), role+".json")
	var cur watchCursor
	if err := store.DecodeStrict(path, &cur); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines, next, err := events.For(d, role, cur.Row)
	if err != nil {
		return err
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(c.Stdout, l); err != nil {
			return err
		}
		if f, ok := c.Stdout.(interface{ Flush() error }); ok {
			if err := f.Flush(); err != nil {
				return err
			}
		}
	}
	return store.WriteJSON(path, watchCursor{Row: next})
}

// watchDelay doubles interval for each failed pass in a row, up to 8 times.
func watchDelay(interval time.Duration, fails int) time.Duration {
	return interval << min(fails, 3)
}
