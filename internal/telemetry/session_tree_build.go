// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

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
			if r.switchesModel {
				view.ModelSwitched++
			} else {
				view.EffortChanged++
			}
		case StateHeld:
			view.Held++
		}
		if r.upshiftNotApplied {
			view.UpshiftNotApplied++
		}
		if r.heldByGuardrail {
			view.HeldGuardrail++
		}
	}
	return view
}

func buildRow(n int, d Event, f *childFacts, opt TreeOptions) TreeRow {
	row := TreeRow{
		Index: n, Label: fmt.Sprintf("subagent %d", n), Status: "running", mark: markRunning,
		Reason: ShortReason(d), Time: d.Timestamp,
		Paused:            hasCode(d.ReasonCodes, ReasonPaused),
		switchesModel:     switchesModel(d.FromModel, d.ToModel),
		upshiftNotApplied: UpshiftNotApplied(d),
		heldByGuardrail:   HeldByGuardrail(d),
		effortFrom:        d.RequestedEffort,
		effortTo:          d.FinalEffort,
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
