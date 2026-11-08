// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"sort"
	"strings"
)

// Tokens routing placed below the session model: the one routing figure.
//
// Owner decision: savings are reported only as tokens, per harness, never in
// dollars and never summed across harnesses. A lower tier is not fewer tokens;
// these tokens ran, and this view only says where.
//
// Population: the priced usage records (outcome usage, Total > 0), folded per
// spawn, so a resumed child counts once. Baseline per spawn: the session model
// (Event.ParentModel) when the catalog ranks it, else the requested model
// (Event.FromModel) as a labelled fallback, kept in separate fields and never
// added to the session ones. A spawn neither baseline can place is unobserved.
// The one comparator is directionVs (rankTiers). Billable is input, output and
// cache write; cache read has its own field and is never inside billable or
// below. Counts only: no prompt, path or command text.

// RoutingRow is one population of spawns on one harness (or one agent type).
type RoutingRow struct {
	// Spawns and Billable cover the spawns placed against the session model.
	// BelowSpawns and BelowBillable are the part that ran below it.
	Spawns        int   `json:"spawns"`
	BelowSpawns   int   `json:"below_spawns"`
	Billable      int64 `json:"billable"`
	BelowBillable int64 `json:"below_billable"`
	// RewrittenSpawns and RewrittenBillable are the part of the below-session
	// figures whose decision was an applied Downshift rewrite (the rest ran
	// below because the caller or the session already chose it).
	RewrittenSpawns   int   `json:"rewritten_spawns"`
	RewrittenBillable int64 `json:"rewritten_billable"`
	// The fallback fields cover spawns with no usable session model, placed
	// against the requested model instead. Never added to the session fields.
	FallbackSpawns        int   `json:"fallback_spawns"`
	FallbackBillable      int64 `json:"fallback_billable"`
	FallbackBelowBillable int64 `json:"fallback_below_billable"`
	// UnobservedSpawns could not be placed against either baseline.
	UnobservedSpawns int `json:"unobserved_spawns"`
	// CacheRead is the cache-read tokens of every spawn, apart from billable.
	CacheRead int64 `json:"cache_read"`
}

// Observed reports whether any spawn was placed against the session model.
// False reads "unobserved" in a report, never 0.
func (r RoutingRow) Observed() bool { return r.Spawns > 0 }

// FallbackObserved reports whether any spawn was placed against the requested
// model only.
func (r RoutingRow) FallbackObserved() bool { return r.FallbackSpawns > 0 }

// BelowShare is the share of session-placed billable tokens that ran below the
// session model, in percent; 0 with none.
func (r RoutingRow) BelowShare() float64 { return share(r.BelowBillable, r.Billable) }

// Add folds o into r, for a caller that merges rows of the same bucket (the
// operator export folds harnesses it cannot name into "unknown").
func (r *RoutingRow) Add(o RoutingRow) {
	r.Spawns += o.Spawns
	r.BelowSpawns += o.BelowSpawns
	r.Billable += o.Billable
	r.BelowBillable += o.BelowBillable
	r.RewrittenSpawns += o.RewrittenSpawns
	r.RewrittenBillable += o.RewrittenBillable
	r.FallbackSpawns += o.FallbackSpawns
	r.FallbackBillable += o.FallbackBillable
	r.FallbackBelowBillable += o.FallbackBelowBillable
	r.UnobservedSpawns += o.UnobservedSpawns
	r.CacheRead += o.CacheRead
}

// RoutingView is RoutingRow folded per harness and per agent type. It has no
// cross-harness total: harness "unknown" is its own row. ByAgentType mixes the
// harnesses of the events it was given, so callers scope the events to one
// harness (a session, a panel) before reading it.
type RoutingView struct {
	ByHarness   map[string]RoutingRow `json:"by_harness"`
	ByAgentType map[string]RoutingRow `json:"by_agent_type,omitempty"`
	// bySpawn is keyed by harness + "\n" + decision key.
	bySpawn map[string]RoutingRow
}

// Spawn returns the figures of the spawn made by decision d.
func (v RoutingView) Spawn(d Event) RoutingRow {
	return v.bySpawn[d.Harness+"\n"+DecisionKey(d)]
}

// spawnPlacement accumulates the usage records of one spawn.
type spawnPlacement struct {
	harness, agent      string
	billable, cacheRead int64
	onSession           bool // some record ranked against the session model
	belowSession        bool
	rewritten           bool // the decision was an applied model rewrite the harness did not refute
	onRequested         bool // some record ranked against the requested model
	belowRequested      bool
}

func (p *spawnPlacement) tokens() RoutingRow {
	t := RoutingRow{CacheRead: p.cacheRead}
	switch {
	case p.onSession:
		t.Spawns, t.Billable = 1, p.billable
		if p.belowSession {
			t.BelowSpawns, t.BelowBillable = 1, p.billable
			if p.rewritten {
				t.RewrittenSpawns, t.RewrittenBillable = 1, p.billable
			}
		}
	case p.onRequested:
		t.FallbackSpawns, t.FallbackBillable = 1, p.billable
		if p.belowRequested {
			t.FallbackBelowBillable = p.billable
		}
	default:
		t.UnobservedSpawns = 1
	}
	return t
}

// RoutingTokens folds the usage records of events into the routing view. A
// resumed child has several records and counts as one spawn: below when any of
// its records ran below the baseline.
func RoutingTokens(events []Event) RoutingView {
	a := newAggregator()
	a.indexDecisions(events)
	decisions := map[linkKey]Event{}
	for _, ev := range events {
		if IsDecisionRow(ev) {
			decisions[linkKey{ev.Harness, DecisionKey(ev)}] = ev
		}
	}
	spawns := map[string]*spawnPlacement{}
	for _, ev := range events {
		if ev.Outcome != OutcomeUsage || !ev.HasTokenUsage() {
			continue
		}
		c := classesOf(ev.TokenUsageOrZero())
		if c.Total == 0 {
			continue
		}
		key := linkKey{ev.Harness, ev.LinkedDecision}
		decision, found := decisions[key]
		_, isRefuted := a.refuted[key]
		id := ev.Harness + "\n" + ev.LinkedDecision
		if !found {
			id = ev.Harness + "\nusage:" + DecisionKey(ev)
		}
		sp := spawns[id]
		if sp == nil {
			sp = &spawnPlacement{harness: ev.Harness, agent: UntypedAgent}
			if found && decision.AgentType != "" {
				sp.agent = decision.AgentType
			}
			sp.rewritten = found && isShift(decision) && !isRefuted
			spawns[id] = sp
		}
		sp.billable += c.Billable()
		sp.cacheRead += c.CacheRead
		requested, ran := tokenModels(ev, decision, found, isRefuted)
		if found && decision.ParentModel != "" {
			if d := directionVs(ev.Harness, decision.ParentModel, ran); d != TokenUnattributed {
				sp.onSession = true
				sp.belowSession = sp.belowSession || d == TokenCheaper
			}
		}
		if d := directionVs(ev.Harness, requested, ran); d != TokenUnattributed {
			sp.onRequested = true
			sp.belowRequested = sp.belowRequested || d == TokenCheaper
		}
	}
	v := RoutingView{ByHarness: map[string]RoutingRow{}, ByAgentType: map[string]RoutingRow{}, bySpawn: map[string]RoutingRow{}}
	for id, sp := range spawns {
		t := sp.tokens()
		v.bySpawn[id] = t
		h := v.ByHarness[sp.harness]
		h.Add(t)
		v.ByHarness[sp.harness] = h
		g := v.ByAgentType[sp.agent]
		g.Add(t)
		v.ByAgentType[sp.agent] = g
	}
	return v
}

// Only returns the row of a view that holds exactly one harness, as a session
// does. With none or several it returns the zero value: harnesses are never
// summed.
func (v RoutingView) Only() RoutingRow {
	if len(v.ByHarness) != 1 {
		return RoutingRow{}
	}
	for _, r := range v.ByHarness {
		return r
	}
	return RoutingRow{}
}

// Line is the short text every live view uses, or "" when there is no spawn:
// "12k of 40k billable below the session model (3 of 5 spawns)". The fallback
// part is its own clause: "against the requested model". Cache reads are not
// in it. A lower tier is not fewer tokens: it says where tokens ran.
func (r RoutingRow) Line() string {
	var parts []string
	if r.Observed() {
		parts = append(parts, fmt.Sprintf("%s of %s billable below the session model (%d of %s)",
			formatTokens(r.BelowBillable), formatTokens(r.Billable), r.BelowSpawns, spawnsText(r.Spawns)))
		if r.RewrittenSpawns > 0 {
			parts = append(parts, fmt.Sprintf("by a Downshift rewrite: %s (%s tokens)", spawnsText(r.RewrittenSpawns), formatTokens(r.RewrittenBillable)))
		}
	}
	if r.FallbackObserved() {
		parts = append(parts, fmt.Sprintf("%s of %s billable below the requested model (%s, no session model)",
			formatTokens(r.FallbackBelowBillable), formatTokens(r.FallbackBillable), spawnsText(r.FallbackSpawns)))
	}
	if r.UnobservedSpawns > 0 {
		parts = append(parts, fmt.Sprintf("%d unobserved", r.UnobservedSpawns))
	}
	return strings.Join(parts, " · ")
}

// spawnsText is "1 spawn" or "3 spawns".
func spawnsText(n int) string {
	if n == 1 {
		return "1 spawn"
	}
	return fmt.Sprintf("%d spawns", n)
}

// HarnessOrder lists the harness ids of a view in print order: declared
// harnesses first, in DeclaredHarnesses order, then any other id, sorted.
func (v RoutingView) HarnessOrder() []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range DeclaredHarnesses {
		if _, ok := v.ByHarness[n.Harness]; ok {
			out = append(out, n.Harness)
			seen[n.Harness] = true
		}
	}
	var rest []string
	for h := range v.ByHarness {
		if !seen[h] {
			rest = append(rest, h)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// UntypedAgent labels spawns whose decision named no agent type.
const UntypedAgent = "untyped"
