package main

import (
	"fmt"
	"io"
	"time"

	"gang/internal/exit"
	"gang/internal/watch"
)

func init() { commands["watch"] = cmdWatch }

// cmdWatch is gang watch [--interval 60s] [--once]. It reloads the plan each
// pass, prints each event line as it is found, and backs off after errors.
func cmdWatch(c *Ctx, args []string) error {
	fs := newFlagSet("watch")
	interval := fs.Duration("interval", 60*time.Second, "time between passes")
	once := fs.Bool("once", false, "run one pass and exit")
	if _, err := owArgs(fs, args, 0); err != nil {
		return err
	}
	if *interval <= 0 {
		return exit.New(exit.Usage, "watch: --interval must be positive")
	}
	fails := 0
	for {
		err := watchPass(c)
		if *once {
			return err
		}
		if err != nil {
			fails++
			fmt.Fprintf(c.Stderr, "gang watch: %v\n", err)
		} else {
			fails = 0
		}
		time.Sleep(watchDelay(*interval, fails))
	}
}

func watchPass(c *Ctx) error {
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	lines, err := watch.Poll(c.Repo, d, p, c.Now())
	for _, l := range lines {
		fmt.Fprintln(c.Stdout, l)
		watchFlush(c.Stdout)
	}
	return err
}

// watchDelay doubles the interval per failed pass in a row, up to 8 times.
func watchDelay(interval time.Duration, fails int) time.Duration {
	return interval << min(fails, 3)
}

func watchFlush(w io.Writer) {
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}
}
