package main

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"

	"source/internal/exit"
	"source/internal/relay"
	"source/internal/transcript"
)

func init() { commands["context"] = cmdContext }

const contextUsage = "usage: monet-source context --role brain|dev [--transcript <path>]"

// cmdContext prints the context percent of this session against the relay
// config of its role (<home> for brain, <run>/dev for dev), and relay when it
// reaches the threshold, else ok (R13, R14, R15). It takes no lease.
func cmdContext(c *Ctx, args []string) error {
	fs := newFlagSet("context")
	role, tr := fs.String("role", "", ""), fs.String("transcript", "", "")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || (*role != "brain" && *role != "dev") {
		return exit.New(exit.Usage, "%s", contextUsage)
	}
	folder := filepath.Join(c.Run, "dev")
	if *role == "brain" {
		home, err := runHome(c.Run)
		if err != nil {
			return err
		}
		folder = home
	}
	cfg, _, err := relay.ReadConfig(folder)
	if err != nil {
		return exit.New(exit.Refused, "context: %v", err)
	}
	p, err := transcript.Find(*tr, relay.SessionID())
	if err != nil {
		return exit.New(exit.Refused, "context: %v", err)
	}
	tokens, err := transcript.LastUsage(p)
	if errors.Is(err, transcript.ErrNoUsage) {
		return exit.New(exit.Refused, "context: no assistant record with usage in %s", p)
	} else if err != nil {
		return err
	}
	pct, verdict := math.Round(float64(tokens)/cfg.Window*1000)/10, "ok"
	if pct >= cfg.Threshold {
		verdict = "relay"
	}
	num := func(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }
	_, err = fmt.Fprintf(c.Stdout, "context %s%% of %s tokens, threshold %s%%, %s\n", num(pct), num(cfg.Window), num(cfg.Threshold), verdict)
	return err
}
