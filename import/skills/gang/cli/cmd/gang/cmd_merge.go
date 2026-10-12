package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"gang/internal/exit"
	"gang/internal/merge"
	"gang/internal/store"
)

func init() { commands["merge"] = cmdMerge }

// cmdMerge prints the Contract 6 JSON, then exits 0 (MERGED, ALREADY),
// 2 (REFUSED), or 3 (CONFLICT). After MERGED it appends the task's duration
// to run/durations.tsv (Contract 0 rule 11).
func cmdMerge(c *Ctx, args []string) error {
	fs := newFlagSet("merge")
	sha := fs.String("sha", "", "the reviewed tip SHA")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *sha == "" {
		return exit.New(exit.Usage, "usage: gang merge <id> --sha <sha>")
	}
	d, p, err := openRun(c)
	if err != nil {
		return err
	}
	r, err := merge.Merge(c.Repo, d, p, pos[0], *sha)
	if err != nil {
		return err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Stdout, string(b))
	switch r.Status {
	case merge.Refused:
		return exit.New(exit.Refused, "merge %s refused: %s", pos[0], r.Reason)
	case merge.Conflict:
		return exit.New(exit.Conflict, "merge %s: %s", pos[0], r.Reason)
	case merge.Merged:
		// The merge already landed, so a failure here is a warning, not an exit code.
		if err := recordDuration(d, pos[0], c.Now()); err != nil {
			fmt.Fprintf(c.Stderr, "gang: merge %s: durations.tsv not updated: %v\n", pos[0], err)
		}
	}
	return nil
}

// recordDuration appends "<id>\t<minutes>" to run/durations.tsv, counting
// whole minutes, rounded up, from run/tasks/<id>/claimed_at to now. It
// writes nothing when claimed_at is missing.
func recordDuration(d store.Dir, id string, now time.Time) error {
	b, err := os.ReadFile(d.Run("tasks", id, "claimed_at"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(string(b)))
	if err != nil {
		return err
	}
	minutes := max(0, int(math.Ceil(now.Sub(at).Minutes())))
	return store.AppendLine(d.Run("durations.tsv"), store.SanitizeCell(id)+"\t"+strconv.Itoa(minutes))
}
