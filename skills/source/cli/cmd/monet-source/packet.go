package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"source/internal/artifact"
	"source/internal/charter"
	"source/internal/item"
	"source/internal/round"
	"source/internal/state"
	"source/internal/store"
)

// packetRound is the newest round of the packet. Outcome is open while the round is open.
type packetRound struct {
	N       int    `json:"n"`
	Phase   string `json:"phase"`
	Final   bool   `json:"final"`
	Version int    `json:"version"`
	Outcome string `json:"outcome"`
}

// packetItem is a decision item that waits; a draft has an empty default.
type packetItem struct {
	ID      string `json:"id"`
	Persona string `json:"persona"`
	Phase   string `json:"phase"`
	Status  string `json:"status"`
	Default string `json:"default"`
	WayBack string `json:"way_back"`
	Affects string `json:"affects"`
}

// packetWall is a wall that is not cleared.
type packetWall struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Default  string `json:"default"`
	Blocks   string `json:"blocks"`
}

// packetWaiting holds the items that are not ruled and the walls that are not cleared, in id order.
type packetWaiting struct {
	Items []packetItem `json:"items"`
	Walls []packetWall `json:"walls"`
}

// packetDecision is one row of decisions.tsv.
type packetDecision struct {
	At       string `json:"at"`
	By       string `json:"by"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// packetLine is one row of log.tsv.
type packetLine struct {
	At   string `json:"at"`
	Line string `json:"line"`
}

// packetVerdict is the persona and result of one check of the final round.
type packetVerdict struct {
	Persona string `json:"persona"`
	Result  string `json:"result"`
}

// packet is the JSON object that report prints for the page (R27).
type packet struct {
	Slug        string                    `json:"slug"`
	Page        string                    `json:"page"`
	Goal        string                    `json:"goal"`
	Deliverable string                    `json:"deliverable"`
	Phase       string                    `json:"phase"`
	Round       *packetRound              `json:"round"`
	Counts      map[string]map[string]int `json:"counts"`
	Changed     string                    `json:"changed"`
	Waiting     packetWaiting             `json:"waiting"`
	Decisions   []packetDecision          `json:"decisions"`
	Log         []packetLine              `json:"log"`
	LogNext     int                       `json:"log_next"`
	Diagram     string                    `json:"diagram"`
	LastWrite   *time.Time                `json:"last_write"`
	Delivered   []string                  `json:"delivered,omitempty"`
	Verdicts    []packetVerdict           `json:"verdicts,omitempty"`
}

// buildPacket reads the charter, the state, the decisions, the new log rows, the newest round, and the waiting items
// and walls of the run in d. The log starts at report_row, log_next is the log row count, and changed is the at of
// the last log row (R28). The round is nil before the first round, and the diagram comes from newestDiagram (R29).
// After delivery, delivered holds the versions of the final round and verdicts the result of each of its checks.
func buildPacket(d store.Dir) (packet, error) {
	ch, err := charter.Load(d.Charter())
	if err != nil {
		return packet{}, err
	}
	s, err := state.Load(d)
	if err != nil {
		return packet{}, err
	}
	p := packet{Slug: s.Slug, Page: ch.Page, Goal: ch.Goal, Deliverable: ch.Deliverable, Phase: firstSet(s.Phase, "kickoff"),
		Counts: s.Counts, LastWrite: s.LastReport}
	if p.Diagram, err = newestDiagram(d.Root); err != nil {
		return packet{}, err
	}
	decs, err := store.ReadTSV(d.Path("decisions.tsv"), decisionHeader)
	if err != nil {
		return packet{}, err
	}
	rows, err := store.ReadTSV(d.Log(), logHeader)
	if err != nil {
		return packet{}, err
	}
	p.Decisions, p.Log, p.LogNext = make([]packetDecision, 0, len(decs)), make([]packetLine, 0, len(rows)), len(rows)
	for _, r := range decs {
		p.Decisions = append(p.Decisions, packetDecision{r[0], r[1], r[2], r[3]})
	}
	for _, r := range rows[min(s.ReportRow, len(rows)):] {
		p.Log = append(p.Log, packetLine{r[0], r[1]})
	}
	if len(rows) > 0 {
		p.Changed = rows[len(rows)-1][0]
	}
	if s.Round > 0 {
		r, err := round.Load(d.Root, s.Round)
		if err != nil {
			return packet{}, err
		}
		p.Round = &packetRound{r.N, r.Phase, r.Final, r.Version, firstSet(r.Outcome, "open")}
		if p.Phase == phaseDelivered {
			p.Delivered = r.Versions
			for _, c := range r.Checks {
				p.Verdicts = append(p.Verdicts, packetVerdict{c.Persona, c.Result})
			}
		}
	}
	items, walls, err := item.Waiting(d)
	if err != nil {
		return packet{}, err
	}
	for _, id := range items {
		var it item.Item
		if err := item.Load(d, id, &it); err != nil {
			return packet{}, err
		}
		p.Waiting.Items = append(p.Waiting.Items, packetItem{it.ID, it.Persona, it.Phase, it.Status, it.Default, it.WayBack, it.Affects})
	}
	for _, id := range walls {
		var w item.Wall
		if err := item.Load(d, id, &w); err != nil {
			return packet{}, err
		}
		p.Waiting.Walls = append(p.Waiting.Walls, packetWall{w.ID, w.Question, w.Default, w.Blocks})
	}
	return p, nil
}

// hasMermaid reports whether a line of the file at path starts with three backticks and mermaid.
func hasMermaid(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if strings.HasPrefix(line, "```mermaid") {
			return true, nil
		}
		if errors.Is(err, io.EOF) {
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
}

// newestDiagram returns the path of the first file, in manifest order, that holds a mermaid fence in the version
// with the newest manifest at among <run>/artifacts/*/v*/manifest.json, or none (R29).
func newestDiagram(run string) (string, error) {
	paths, err := filepath.Glob(filepath.Join(run, "artifacts", "*", "v*", "manifest.json"))
	if err != nil {
		return "", err
	}
	newest, seen := artifact.Manifest{}, false
	for _, p := range paths {
		var m artifact.Manifest
		if err := store.DecodeStrict(p, &m); err != nil {
			return "", err
		}
		if !seen || m.At.After(newest.At) {
			newest, seen = m, true
		}
	}
	for _, f := range newest.Files {
		if ok, err := hasMermaid(f.Path); err != nil {
			return "", err
		} else if ok {
			return f.Path, nil
		}
	}
	return "none", nil
}
