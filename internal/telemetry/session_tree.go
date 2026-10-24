// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"sort"
	"time"
)

// RedoWindow is how soon after a failed child a new spawn in the same session
// counts as its redo (see RespawnAfterFailure).
const RedoWindow = 10 * time.Minute

// This file holds the types and entry points of `downshift session` (U3): the
// subagents of one session as a tree. session_tree_build.go folds the events
// into rows, session_tree_render.go writes the plain words. Both are the one
// place text is made, for every harness. They read only routing metadata
// already in the event log: a row is labelled "subagent N", never with task
// text, and nothing here is written back.

// Row marks, one cell wide.
const (
	markDone    = "✓"
	markRedone  = "↻"
	markError   = "✗"
	markRunning = "…"
	markUnknown = "?"
)

// RunningTimeout is how long a child with no end, usage or error record is
// shown as running. After it the end is unknown ("?"): the child may have
// finished without the harness telling us, and a stale "…" would lie.
const RunningTimeout = 2 * time.Hour

// TreeOptions tunes SessionTree. The zero value is valid.
type TreeOptions struct {
	// Paused adds the pause flag to the header: routing is paused for the
	// session (see SessionPaused).
	Paused bool
	// Now is the clock for the RunningTimeout. The zero value means time.Now().
	Now time.Time
}

// TreeRow is one subagent of the session.
type TreeRow struct {
	Index  int    `json:"index"`
	Depth  int    `json:"depth"` // 0 for a first attempt, +1 for each redo level
	Label  string `json:"label"`
	Model  string `json:"model"`
	Status string `json:"status"` // done|redone|error|blocked|running|unknown
	// Tokens is this child's tokens saved from the requested model (work and
	// cache apart), known once it ended; TokensLabel its text. No dollars in
	// the live view: `downshift report` prices tokens.
	Tokens      TokenSaving `json:"tokens"`
	TokensLabel string      `json:"tokens_label"`
	Reason      string      `json:"reason"`
	// State is what Downshift did: switched|held|observed|paused|control|blocked.
	State       string   `json:"state"`
	FromModel   string   `json:"from_model,omitempty"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	RedoOf      int      `json:"redo_of,omitempty"` // index of the child this one redid
	Paused      bool     `json:"paused,omitempty"`
	Time        string   `json:"time,omitempty"`

	// switchesModel is true when the child ran on another model than requested.
	switchesModel bool
	// effortFrom and effortTo are the requested and final effort of the decision.
	effortFrom, effortTo string
	// upshiftNotApplied and heldByGuardrail classify the row (see UpshiftNotApplied).
	upshiftNotApplied bool
	heldByGuardrail   bool

	mark string
}

// SessionTreeView is the structured session view; Render turns it into text.
type SessionTreeView struct {
	Session   string `json:"session"` // first 8 hex of the hashed id
	Subagents int    `json:"subagents"`
	// Tokens is the session's tokens saved from the requested model, and
	// ByAgentType the same per agent type (TokenSavings).
	Tokens      TokenSaving      `json:"tokens"`
	ByAgentType []PanelAgentType `json:"by_agent_type"`
	Paused      bool             `json:"paused"`
	// Switched and Held count the rows Downshift changed and the rows it
	// considered and kept.
	Switched int `json:"switched"`
	// ModelSwitched and EffortChanged split Switched (see PanelView).
	ModelSwitched int `json:"model_switched"`
	EffortChanged int `json:"effort_changed"`
	Held          int `json:"held"`
	// HeldGuardrail and UpshiftNotApplied split the held rows and count the
	// observed-or-held UPSHIFT verdicts that did not change the spawn (see PanelView).
	HeldGuardrail     int       `json:"held_guardrail"`
	UpshiftNotApplied int       `json:"upshift_not_applied"`
	Rows              []TreeRow `json:"rows"`
}

// IsDecisionRow reports whether an event is a routing decision of a subagent:
// not a usage, resolved, child-ended or failure record. A control-group
// (baseline) decision is one: it is a spawn, only left unchanged.
func IsDecisionRow(ev Event) bool {
	if ev.Harness == "" {
		return false
	}
	switch ev.Outcome {
	case OutcomeUsage, OutcomeResolved, OutcomeChildEnded, "error", OutcomeFailOpen, OutcomeDCI:
		return false
	}
	return true
}

// LatestSessionHash returns the session of the most recent decision in the
// log, or "" when no decision carries a session id.
func LatestSessionHash(events []Event) string {
	for i := len(events) - 1; i >= 0; i-- {
		if IsDecisionRow(events[i]) && ValidSessionHash(events[i].SessionID) {
			return events[i].SessionID
		}
	}
	return ""
}

// SessionTree renders the tree for sessionHash. An empty or unknown session
// renders the empty tree: a view never mixes sessions.
func SessionTree(events []Event, sessionHash string, opt TreeOptions) string {
	return BuildSessionTree(events, sessionHash, opt).Render()
}

// sessionEvents returns the events of one session in time order.
func sessionEvents(events []Event, sessionHash string) []Event {
	if sessionHash == "" {
		return nil
	}
	var out []Event
	for _, ev := range events {
		if ev.SessionID == sessionHash {
			out = append(out, ev)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return evTime(out[i]).Before(evTime(out[j])) })
	return out
}

func evTime(ev Event) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, ev.Timestamp)
	return t
}
