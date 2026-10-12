// Package status renders run/status.md and the 5-line summary.
package status

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"gang/internal/ledger"
	"gang/internal/notes"
	"gang/internal/plan"
	"gang/internal/store"
	"gang/internal/wave"
)

// logHeading starts the coordinator's own section of status.md. Write keeps
// everything from this line to the end.
const logHeading = "## Log"

// Write renders run/status.md and returns the summary lines: merged tasks,
// ledger counts, open gates, the wave PR, and the driver lease.
func Write(repo string, d store.Dir, p *plan.Plan, now time.Time) ([]string, error) {
	done, err := wave.Done(repo, p)
	if err != nil {
		return nil, err
	}
	sum, err := ledger.Summary(d)
	if err != nil {
		return nil, err
	}
	gates, err := notes.List(d)
	if err != nil {
		return nil, err
	}
	open := 0 // notes.List returns open gates, the check keeps the count honest either way
	for _, g := range gates {
		if g.State == "open" {
			open++
		}
	}
	rows, err := wave.Read(d)
	if err != nil {
		return nil, err
	}
	driver, err := driverLine(d, now)
	if err != nil {
		return nil, err
	}
	lines := []string{
		fmt.Sprintf("merged %d/%d tasks", len(done), len(p.Tasks)),
		fmt.Sprintf("ledger CLEAN %d CHANGES %d INCOMPLETE %d", sum[ledger.Clean], sum[ledger.Changes], sum[ledger.Incomplete]),
		fmt.Sprintf("gates open %d", open),
		waveLine(rows),
		driver,
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# gang status: %s\n\nUpdated %s.\n\n", p.Slug, now.UTC().Format(time.RFC3339))
	for _, l := range lines {
		b.WriteString("- " + l + "\n")
	}
	b.WriteString("\n## Tasks\n\n| Task | Wave | Merged |\n|---|---|---|\n")
	for _, t := range p.Tasks {
		m := "no"
		if done[t.ID] {
			m = "yes"
		}
		fmt.Fprintf(&b, "| %s | %d | %s |\n", t.ID, t.Wave, m)
	}
	b.WriteString("\n" + keptLog(d.Run("status.md")))
	if err := store.WriteFileAtomic(d.Run("status.md"), []byte(b.String()), 0o644); err != nil {
		return nil, err
	}
	return lines, nil
}

// waveLine names the open wave PR, else the latest one, else none.
func waveLine(rows []wave.Row) string {
	var pick *wave.Row
	for i := range rows {
		r := &rows[i]
		if r.State == wave.StateOpen {
			pick = r
			break
		}
		if pick == nil || r.Wave > pick.Wave {
			pick = r
		}
	}
	if pick == nil {
		return "wave PR none"
	}
	return fmt.Sprintf("wave PR %d #%d %s", pick.Wave, pick.PR, pick.State)
}

// driverLine reads run/locks/driver.json (Contract 4). An expired lease is free.
func driverLine(d store.Dir, now time.Time) (string, error) {
	b, err := os.ReadFile(d.Run("locks", "driver.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return "driver free", nil
	}
	if err != nil {
		return "", err
	}
	var lease struct {
		Holder  string    `json:"holder"`
		Expires time.Time `json:"expires"`
	}
	if err := json.Unmarshal(b, &lease); err != nil {
		return "", fmt.Errorf("run/locks/driver.json: %w", err)
	}
	if !now.Before(lease.Expires) {
		return "driver free", nil
	}
	return fmt.Sprintf("driver %s until %s", lease.Holder, lease.Expires.UTC().Format(time.RFC3339)), nil
}

// keptLog returns the "## Log" section of the old status.md, or a new one.
func keptLog(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return logHeading + "\n"
	}
	s := string(b)
	if strings.HasPrefix(s, logHeading+"\n") {
		return s
	}
	if i := strings.Index(s, "\n"+logHeading+"\n"); i >= 0 {
		return s[i+1:]
	}
	return logHeading + "\n"
}
