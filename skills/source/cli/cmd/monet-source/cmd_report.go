package main

import (
	"encoding/json"
	"time"

	"source/internal/exit"
	"source/internal/state"
	"source/internal/store"
)

func init() { commands["report"] = cmdReport }

const reportUsage = "usage: monet-source report [done --at <time> --row <n> --holder <session>]"

// reportDoneWord is the inline subcommand of report that records a page write.
const reportDoneWord = "done"

// cmdReport prints the packet of the run as JSON with a two-space indent and takes no lease (R1).
// A buildPacket error is exit 2, "report done" records a page write, and any other argument is exit 64.
func cmdReport(c *Ctx, args []string) error {
	if len(args) > 0 && args[0] == reportDoneWord {
		return reportDone(c, args[1:])
	} else if len(args) > 0 {
		return exit.New(exit.Usage, "%s", reportUsage)
	}
	p, err := buildPacket(store.Dir{Root: c.Run})
	if err != nil {
		return exit.New(exit.Refused, "report: %v", err)
	}
	enc := json.NewEncoder(c.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(p)
}

// reportDone renews brain for --holder and stores --at in UTC as last_report and --row as report_row (R2, R30).
// A time that is not RFC 3339 or no --row is exit 64. A time after now or before last_report, or a row above the
// log.tsv row count, is exit 2. It writes no log row and emits no event.
func reportDone(c *Ctx, args []string) error {
	fs := newFlagSet("report done")
	atText := fs.String("at", "", "when the page was written, in RFC 3339")
	row := fs.Int("row", -1, "the log.tsv row where the next packet starts")
	holder := fs.String("holder", "", "the brain lease holder")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	at, err := time.Parse(time.RFC3339, *atText)
	if err != nil || *row < 0 || len(pos) != 0 {
		return exit.New(exit.Usage, "%s", reportUsage)
	}
	if err := requireLease(c, "brain", *holder); err != nil {
		return err
	}
	d, at := store.Dir{Root: c.Run}, at.UTC()
	return state.Update(d, func(s *state.State) error {
		rows, err := store.ReadTSV(d.Log(), logHeader)
		if err != nil {
			return err
		}
		now := c.Now().UTC()
		switch {
		case at.After(now):
			return exit.New(exit.Refused, "report done: --at %s is after now %s", *atText, now.Format(time.RFC3339Nano))
		case s.LastReport != nil && at.Before(*s.LastReport):
			return exit.New(exit.Refused, "report done: --at %s is before last_report %s", *atText, s.LastReport.UTC().Format(time.RFC3339Nano))
		case *row > len(rows):
			return exit.New(exit.Refused, "report done: --row %d is above the %d rows of log.tsv", *row, len(rows))
		}
		s.LastReport, s.ReportRow = &at, *row
		return nil
	})
}
