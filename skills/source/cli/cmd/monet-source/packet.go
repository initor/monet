package main

import (
	"time"

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
	Verdicts    []any                     `json:"verdicts,omitempty"`
}

// buildPacket reads the charter, the state, the decisions, the new log rows, the newest round, and the waiting items
// and walls of the run in d. The log starts at report_row, log_next is the log row count, and changed is the at of
// the last log row (R28). The round is nil before the first round, and the diagram is none.
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
		Counts: s.Counts, Diagram: "none", LastWrite: s.LastReport}
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
