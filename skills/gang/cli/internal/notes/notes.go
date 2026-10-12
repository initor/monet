// Package notes writes the run's text channels: run/tasks/<id>/report.md and
// named task files, run/gates.md, and run/standing.md. Every write is atomic
// and runs under a mutex file, so concurrent agents cannot lose an update.
package notes

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gang/internal/lock"
	"gang/internal/store"
)

// ErrNotFound means the named gate does not exist.
var ErrNotFound = errors.New("not found")

var (
	nameRE  = regexp.MustCompile(`^[a-z0-9._-]+\.md$`)
	orderRE = regexp.MustCompile(`^[0-9]+\.\s*(.*)$`)
)

// reserved names are written by the report append and by the owner.
var reserved = map[string]bool{"report.md": true, "owner.md": true}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func taskDir(d store.Dir, task string) (string, error) {
	if task == "" || task == "." || task == ".." || task != filepath.Base(task) || strings.ContainsAny(task, " \t\n") {
		return "", fmt.Errorf("bad task id %q", task)
	}
	return d.Run("tasks", task), nil
}

// update rewrites path under its mutex: fn gets the old content (nil when
// absent) and returns the new content.
func update(path string, fn func(old []byte) ([]byte, error)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return lock.WithMutex(path+".mutex", func() error {
		old, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		b, err := fn(old)
		if err != nil {
			return err
		}
		return store.WriteFileAtomic(path, b, 0o644)
	})
}

// Report appends body to run/tasks/<task>/report.md under "## <heading>"
// (default "report") and returns the file's path.
func Report(d store.Dir, task, heading string, body []byte) (string, error) {
	dir, err := taskDir(d, task)
	if err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return "", errors.New("report is empty")
	}
	if heading = oneLine(heading); heading == "" {
		heading = "report"
	}
	path := filepath.Join(dir, "report.md")
	err = update(path, func(old []byte) ([]byte, error) {
		var b bytes.Buffer
		if len(old) > 0 {
			b.Write(bytes.TrimRight(old, "\n"))
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "## %s\n\n%s\n", heading, bytes.TrimRight(body, "\n"))
		return b.Bytes(), nil
	})
	return path, err
}

// WriteNamed writes body to run/tasks/<task>/<name>. The name must match
// ^[a-z0-9._-]+\.md$ and must not be report.md or owner.md.
func WriteNamed(d store.Dir, task, name string, body []byte) (string, error) {
	dir, err := taskDir(d, task)
	if err != nil {
		return "", err
	}
	if !nameRE.MatchString(name) || name == ".md" {
		return "", fmt.Errorf("name %q must match %s", name, nameRE)
	}
	if reserved[name] {
		return "", fmt.Errorf("%s is reserved", name)
	}
	path := filepath.Join(dir, name)
	return path, update(path, func([]byte) ([]byte, error) { return body, nil })
}

// Gate is one block of run/gates.md.
type Gate struct {
	ID, Task, State, Question, Default, Answer string
}

func parseGates(b []byte) []Gate {
	var gs []Gate
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			f := strings.Fields(rest)
			for len(f) < 3 {
				f = append(f, "")
			}
			gs = append(gs, Gate{ID: f[0], Task: f[1], State: f[2]})
			continue
		}
		if len(gs) == 0 {
			continue
		}
		g := &gs[len(gs)-1]
		if v, ok := strings.CutPrefix(line, "Question: "); ok {
			g.Question = v
		} else if v, ok := strings.CutPrefix(line, "Default: "); ok {
			g.Default = v
		} else if v, ok := strings.CutPrefix(line, "Answer: "); ok {
			g.Answer = v
		}
	}
	return gs
}

func renderGates(gs []Gate) []byte {
	var b bytes.Buffer
	for i, g := range gs {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "## %s %s %s\nQuestion: %s\nDefault: %s\n", g.ID, g.Task, g.State, g.Question, g.Default)
		if g.Answer != "" {
			fmt.Fprintf(&b, "Answer: %s\n", g.Answer)
		}
	}
	return b.Bytes()
}

// Park appends an open gate for task and returns its id (G1, G2, ...).
func Park(d store.Dir, task, question, def string) (string, error) {
	if _, err := taskDir(d, task); err != nil {
		return "", err
	}
	question, def = oneLine(question), oneLine(def)
	if question == "" || def == "" {
		return "", errors.New("a gate needs a question and a default")
	}
	var id string
	err := update(d.Run("gates.md"), func(old []byte) ([]byte, error) {
		gs := parseGates(old)
		n := 0
		for _, g := range gs {
			if k, err := strconv.Atoi(strings.TrimPrefix(g.ID, "G")); err == nil && k > n {
				n = k
			}
		}
		id = "G" + strconv.Itoa(n+1)
		return renderGates(append(gs, Gate{ID: id, Task: task, State: "open", Question: question, Default: def})), nil
	})
	return id, err
}

// List returns the open gates in file order.
func List(d store.Dir) ([]Gate, error) {
	b, err := os.ReadFile(d.Run("gates.md"))
	if errors.Is(err, os.ErrNotExist) {
		return []Gate{}, nil
	}
	if err != nil {
		return nil, err
	}
	open := []Gate{}
	for _, g := range parseGates(b) {
		if g.State == "open" {
			open = append(open, g)
		}
	}
	return open, nil
}

// Answer marks gate id answered and records the answer.
func Answer(d store.Dir, id, answer string) error {
	if answer = oneLine(answer); answer == "" {
		return errors.New("the answer is empty")
	}
	return update(d.Run("gates.md"), func(old []byte) ([]byte, error) {
		gs := parseGates(old)
		for i := range gs {
			if gs[i].ID == id {
				gs[i].State, gs[i].Answer = "answered", answer
				return renderGates(gs), nil
			}
		}
		return nil, fmt.Errorf("gate %s: %w", id, ErrNotFound)
	})
}

// standing is run/standing.md: an optional STOP reason, then the orders.
type standing struct {
	stop   string
	orders []string
}

// parseStanding keeps the first STOP: line as the stop and turns every other
// non-blank line into an order, so a hand edit is renumbered, not lost.
func parseStanding(b []byte) standing {
	var s standing
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "STOP:"):
			if s.stop == "" {
				s.stop = strings.TrimSpace(strings.TrimPrefix(line, "STOP:"))
			}
		default:
			if m := orderRE.FindStringSubmatch(line); m != nil {
				line = m[1]
			}
			s.orders = append(s.orders, line)
		}
	}
	return s
}

func (s standing) render() []byte {
	var b bytes.Buffer
	if s.stop != "" {
		fmt.Fprintf(&b, "STOP: %s\n", s.stop)
	}
	for i, o := range s.orders {
		fmt.Fprintf(&b, "%d. %s\n", i+1, o)
	}
	return b.Bytes()
}

func editStanding(d store.Dir, fn func(s *standing)) error {
	return update(d.Run("standing.md"), func(old []byte) ([]byte, error) {
		s := parseStanding(old)
		fn(&s)
		return s.render(), nil
	})
}

// Show returns run/standing.md, or "" when it does not exist.
func Show(d store.Dir) (string, error) {
	b, err := os.ReadFile(d.Run("standing.md"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// Add appends one numbered order and renumbers the rest from 1.
func Add(d store.Dir, line string) error {
	if line = oneLine(line); line == "" {
		return errors.New("the order is empty")
	}
	return editStanding(d, func(s *standing) { s.orders = append(s.orders, line) })
}

// Stop sets the first line to "STOP: <reason>", replacing any earlier stop.
func Stop(d store.Dir, reason string) error {
	if reason = oneLine(reason); reason == "" {
		return errors.New("a stop needs a reason")
	}
	return editStanding(d, func(s *standing) { s.stop = reason })
}

// ClearStop removes the STOP line and keeps the orders.
func ClearStop(d store.Dir) error {
	return editStanding(d, func(s *standing) { s.stop = "" })
}
