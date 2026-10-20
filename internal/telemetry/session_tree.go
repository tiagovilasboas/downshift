// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// RedoWindow is how soon after a failed child a new spawn in the same session
// counts as its redo (see RespawnAfterFailure).
const RedoWindow = 10 * time.Minute

// This file builds the session view of `downshift session` (U3): the subagents
// of one session as a tree, and the plain-words reason for the latest decision.
// It is the one place that text is made, for every harness. It reads only
// routing metadata already in the event log: a row is labelled "subagent N",
// never with task text, and nothing here is written back.

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
	Switched int       `json:"switched"`
	Held     int       `json:"held"`
	Rows     []TreeRow `json:"rows"`
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

// childFacts gathers the observations linked to one decision.
type childFacts struct {
	resolved *Event
	usage    *Event
	failed   bool
	done     bool
}

// BuildSessionTree folds the session's events into rows.
func BuildSessionTree(events []Event, sessionHash string, opt TreeOptions) SessionTreeView {
	view := SessionTreeView{Paused: opt.Paused}
	if len(sessionHash) >= 8 {
		view.Session = sessionHash[:8]
	}
	evs := sessionEvents(events, sessionHash)

	var decisions []Event
	facts := map[string]*childFacts{}
	get := func(k string) *childFacts {
		if facts[k] == nil {
			facts[k] = &childFacts{}
		}
		return facts[k]
	}
	for i := range evs {
		ev := evs[i]
		switch {
		case IsDecisionRow(ev):
			decisions = append(decisions, ev)
		case ev.Outcome == OutcomeResolved && ev.LinkedDecision != "":
			get(ev.LinkedDecision).resolved = &evs[i]
		case ev.Outcome == OutcomeUsage && ev.LinkedDecision != "":
			f := get(ev.LinkedDecision)
			f.usage = foldUsage(f.usage, &evs[i])
		case ev.Outcome == OutcomeChildEnded && ev.LinkedDecision != "":
			f := get(ev.LinkedDecision)
			switch {
			case failedEnd(ev.ChildStatus):
				f.failed = true
			case ev.ChildStatus == ChildCompleted || ev.ChildStatus == ChildStopped:
				f.done = true
			}
		}
	}

	saved := TokenSavings(evs)
	view.Tokens, view.ByAgentType = saved.Total, AgentTypeLines(saved)
	rows := make([]TreeRow, len(decisions))
	index := map[string]int{}
	for i, d := range decisions {
		key := DecisionKey(d)
		index[key] = i
		rows[i] = buildRow(i+1, d, facts[key], opt)
		rows[i].Tokens = saved.Child(d)
		rows[i].TokensLabel = ChildTokensLabel(rows[i].Tokens, d)
	}

	// A redo nests under the child it redid; the child it redid is marked ↻.
	parent := make([]int, len(rows))
	for i := range parent {
		parent[i] = -1
	}
	for _, r := range RespawnAfterFailure(evs, RedoWindow) {
		f, okF := index[r.FailedKey]
		n, okN := index[r.NextKey]
		if !okF || !okN || f == n {
			continue
		}
		rows[f].Status, rows[f].mark = "redone", markRedone
		if parent[n] == -1 && n > f {
			parent[n] = f
			rows[n].RedoOf = rows[f].Index
		}
	}

	view.Rows = orderRows(rows, parent)
	view.Subagents = len(rows)
	for _, r := range view.Rows {
		switch r.State {
		case StateSwitched:
			view.Switched++
		case StateHeld:
			view.Held++
		}
	}
	return view
}

func buildRow(n int, d Event, f *childFacts, opt TreeOptions) TreeRow {
	row := TreeRow{
		Index: n, Label: fmt.Sprintf("subagent %d", n), Status: "running", mark: markRunning,
		Reason: ShortReason(d), Time: d.Timestamp,
		Paused: hasCode(d.ReasonCodes, ReasonPaused),
	}
	if f == nil {
		f = &childFacts{}
	}
	row.Model = rowModel(d, f)
	row.State, row.FromModel, row.ReasonCodes = SpawnStateOf(d), shortOrQ(d.FromModel), d.ReasonCodes
	if row.State == StateSwitched && d.FromModel != d.ToModel {
		row.Reason = "switched from " + row.FromModel + ", " + row.Reason
	}
	switch {
	case d.Outcome == OutcomeBlocked:
		row.Status, row.mark = "blocked", markError
	case f.failed:
		row.Status, row.mark = "error", markError
	case f.usage != nil || f.done:
		row.Status, row.mark = "done", markDone
	default:
		now := opt.Now
		if now.IsZero() {
			now = time.Now()
		}
		if t := evTime(d); !t.IsZero() && now.Sub(t) > RunningTimeout {
			row.Status, row.mark = "unknown", markUnknown
		}
	}
	return row
}

// rowModel is the model the child ran on: the usage record's, else the
// resolved record's, else the decision's own final model. A resolved record
// that refutes the rewrite comes first: it is what the harness ran.
func rowModel(d Event, f *childFacts) string {
	order := []*Event{f.usage, f.resolved, &d}
	if f.resolved != nil && f.resolved.RewriteHonored != nil && !*f.resolved.RewriteHonored {
		order = []*Event{f.resolved, f.usage, &d}
	}
	for _, ev := range order {
		if ev != nil && ev.ToModel != "" && ev.ToModel != "unknown" {
			return ShortModel(ev.ToModel)
		}
	}
	return "?"
}

func hasCode(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// shortCode is the few-word form of a reason code for the tree's last column.
var shortCode = map[string]string{
	ReasonSemanticBoost:    "boosted",
	ReasonTaskFocus:        "task focus",
	ReasonRiskFloor:        "risk floor",
	ReasonNBUpshiftApplied: "second opinion up",
	ReasonTierFloor:        "tier floor",
	ReasonCreditHeld:       "credit held",
	ReasonGuessHeld:        "guessed baseline",
	ReasonScoreRouted:      "by score",
	ReasonPaused:           "paused",
}

// ShortReason is the tree's reason column for a decision: the complexity, how
// sure the classifier was, then the notable rules that applied. Events written
// before reason codes carry only the complexity.
func ShortReason(d Event) string {
	var parts []string
	if d.Complexity != "" {
		parts = append(parts, d.Complexity)
	}
	if hasCode(d.ReasonCodes, ReasonClassifierConfident) {
		parts = append(parts, "confident")
	} else if hasCode(d.ReasonCodes, ReasonClassifierUnconfident) {
		parts = append(parts, "unsure")
	}
	held := false
	for _, c := range d.ReasonCodes {
		if strings.HasPrefix(c, "HELD_") {
			held = true
		}
	}
	if len(d.Corrections) > 0 {
		held = true
	}
	for _, c := range sortedCodes(d.ReasonCodes) {
		if s, ok := shortCode[c]; ok {
			parts = append(parts, s)
		}
	}
	if held {
		parts = append(parts, "held")
	}
	if d.Outcome == OutcomeBlocked {
		parts = append(parts, "blocked")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

func sortedCodes(codes []string) []string {
	out := append([]string(nil), codes...)
	sort.Strings(out)
	return out
}

// orderRows lists parents in time order, each followed by its redo chain.
func orderRows(rows []TreeRow, parent []int) []TreeRow {
	children := map[int][]int{}
	var roots []int
	for i := range rows {
		if parent[i] >= 0 {
			children[parent[i]] = append(children[parent[i]], i)
		} else {
			roots = append(roots, i)
		}
	}
	out := make([]TreeRow, 0, len(rows))
	var walk func(i, depth int)
	walk = func(i, depth int) {
		r := rows[i]
		r.Depth = depth
		out = append(out, r)
		for _, c := range children[i] {
			walk(c, depth+1)
		}
	}
	for _, i := range roots {
		walk(i, 0)
	}
	return out
}

// Render lays the view out as text. Columns are aligned: tree and label, model,
// status, tokens, reason; a last line gives tokens saved per agent type.
func (v SessionTreeView) Render() string {
	noun := "subagents"
	if v.Subagents == 1 {
		noun = "subagent"
	}
	head := []string{"session"}
	if v.Subagents == 0 {
		head = append(head, "no subagents yet")
	} else {
		head = append(head, fmt.Sprintf("%d %s", v.Subagents, noun))
		head = append(head, fmt.Sprintf("%d switched, %d held", v.Switched, v.Held))
		if line := v.Tokens.Line(); line != "" {
			head = append(head, line)
		} else {
			head = append(head, "tokens not observed")
		}
	}
	if v.Paused {
		head = append(head, "PAUSED: routing off for this session")
	}
	var b strings.Builder
	b.WriteString(strings.Join(head, " · "))
	b.WriteByte('\n')

	names := make([]string, len(v.Rows))
	costs := make([]string, len(v.Rows))
	nameW, modelW, costW := 0, 0, 0
	var lastAt []bool // per ancestor depth: that ancestor was the last of its siblings
	for i, r := range v.Rows {
		last := true
		for _, n := range v.Rows[i+1:] {
			if n.Depth < r.Depth {
				break
			}
			if n.Depth == r.Depth {
				last = false
				break
			}
		}
		lastAt = append(lastAt[:r.Depth], last)
		var p strings.Builder
		for d := 0; d < r.Depth; d++ {
			if lastAt[d] {
				p.WriteString("   ")
			} else {
				p.WriteString("│  ")
			}
		}
		if last {
			p.WriteString("└─ ")
		} else {
			p.WriteString("├─ ")
		}
		names[i] = p.String() + r.Label
		nameW = max(nameW, utf8.RuneCountInString(names[i]))
		modelW = max(modelW, utf8.RuneCountInString(r.Model))
		costs[i] = "-"
		if r.TokensLabel != "" {
			costs[i] = r.TokensLabel
		}
		costW = max(costW, utf8.RuneCountInString(costs[i]))
	}
	for i, r := range v.Rows {
		line := fmt.Sprintf("%s  %s  %s  %s  %s",
			padRight(names[i], nameW), padRight(r.Model, modelW), r.mark,
			padRight(costs[i], costW), r.Reason)
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteByte('\n')
	}
	if lines := SavedTypeLines(v.ByAgentType); len(lines) > 0 {
		b.WriteString("by type: " + strings.Join(lines, " · ") + "\n")
	}
	return b.String()
}

func padRight(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func padLeft(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return strings.Repeat(" ", w-n) + s
	}
	return s
}

// foldUsage merges a child's usage records into one view. A child that stopped,
// was sent another message and stopped again has one record per stop, each
// holding that stretch's tokens and cost, so the child's cost is their sum. The
// later record's model wins, as the one the child ended on.
func foldUsage(prev, next *Event) *Event {
	if prev == nil {
		return next
	}
	sumI := func(a, b *int64) *int64 {
		if a == nil && b == nil {
			return nil
		}
		var n int64
		if a != nil {
			n += *a
		}
		if b != nil {
			n += *b
		}
		return &n
	}
	merged := *next
	merged.InputTokens = sumI(prev.InputTokens, next.InputTokens)
	merged.OutputTokens = sumI(prev.OutputTokens, next.OutputTokens)
	merged.CachedTokens = sumI(prev.CachedTokens, next.CachedTokens)
	merged.CacheWriteTokens = sumI(prev.CacheWriteTokens, next.CacheWriteTokens)
	return &merged
}
