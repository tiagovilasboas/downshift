// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// This file is the one place the text shown to the developer inside a harness
// (statusline summary, child-end line) is built. Adapters and the CLI bridge
// only attach it to their wire output. Nothing here reads prompt text: it
// renders routing metadata that is already in the event log.

// Quiet reports the operator opt-out DOWNSHIFT_QUIET=1: no routing text is
// shown inside the harness (spawn message and child-end line). Logging to the
// event log and stderr notes are unchanged.
func Quiet() bool { return os.Getenv("DOWNSHIFT_QUIET") == "1" }

// ChildLineEnabled combines a harness capability (CanShowChildLine) with the
// operator opt-out.
func ChildLineEnabled(canShow bool) bool { return canShow && !Quiet() }

// ShortModel is the family name a developer recognises: claude-haiku-4-5-2025
// becomes haiku. Ids that do not start with "claude-" are returned unchanged,
// as is an id that has no non-numeric part left.
func ShortModel(id string) string {
	if !strings.HasPrefix(id, "claude-") {
		return id
	}
	var keep []string
	for _, p := range strings.Split(strings.TrimPrefix(id, "claude-"), "-") {
		if p == "" || (p[0] >= '0' && p[0] <= '9') {
			continue
		}
		keep = append(keep, p)
	}
	if len(keep) == 0 {
		return id
	}
	return strings.Join(keep, "-")
}

// SessionRouting is the routing picture of one session, from the event log.
type SessionRouting struct {
	Spawns    int     // routing decisions made for the session
	TopModel  string  // short name of the model most children ran on
	TopModelN int     // how many children ran on TopModel
	HasCost   bool    // at least one child was priced against its baseline
	DeltaUSD  float64 // signed real delta: baseline minus actual, summed
	OnDoubt   int     // frontier decisions resting on an unconfident classification
	// QuotaStale marks that an expired quota snapshot is still holding model
	// rewrites (core.StaleQuotaHold). The caller sets it; the event log has no
	// say in it.
	QuotaStale bool
}

// QuotaStaleMarker is the segment appended while a stale snapshot holds.
const QuotaStaleMarker = "quota stale"

// SessionSummary folds the events that belong to sessionHash (see
// HashSessionID). An empty hash matches nothing: a summary must never mix
// sessions. Cost-only records contribute only their signed real delta.
func SessionSummary(events []Event, sessionHash string) SessionRouting {
	var s SessionRouting
	if sessionHash == "" {
		return s
	}
	byModel := map[string]int{}
	refuted := RefutedRewrites(events)
	decisions := map[string]Event{}
	for _, ev := range events {
		if IsDecisionRow(ev) {
			decisions[ev.Harness+"\n"+DecisionKey(ev)] = ev
		}
	}
	for _, ev := range events {
		if ev.SessionID != sessionHash {
			continue
		}
		if ev.Outcome == OutcomeUsage {
			if ev.HasRealCost() && !refuted[ev.Harness+"\n"+ev.LinkedDecision] {
				decision, found := decisions[ev.Harness+"\n"+ev.LinkedDecision]
				s.HasCost = true
				s.DeltaUSD += RoutedCost(ev, decision, found).RealDeltaUSD()
			}
			continue
		}
		if IsCostOnlyOutcome(ev.Outcome) || IsFailureOutcome(ev.Outcome) {
			continue
		}
		s.Spawns++
		isRefuted := refuted[ev.Harness+"\n"+DecisionKey(ev)]
		ran := ev.FromModel
		if AppliedRewrite(ev.Outcome, ev.Corrections) && ev.Outcome == OutcomeRewriteEmitted && !isRefuted {
			ran = ev.ToModel
		}
		if ran != "" && ran != "unknown" {
			byModel[ShortModel(ran)]++
		}
		if spentOnDoubt(ev) && !isRefuted {
			s.OnDoubt++
		}
	}
	names := make([]string, 0, len(byModel))
	for m := range byModel {
		names = append(names, m)
	}
	sort.Strings(names) // deterministic tie-break
	for _, m := range names {
		if byModel[m] > s.TopModelN {
			s.TopModel, s.TopModelN = m, byModel[m]
		}
	}
	return s
}

// Line renders the statusline summary, or "" when the session has no routing
// activity and no stale quota to flag. Segments without data are omitted.
func (s SessionRouting) Line() string {
	if s.Spawns == 0 {
		// Nothing routed yet, but a holding snapshot is the reason nothing will.
		if s.QuotaStale {
			return "◆ downshift · " + QuotaStaleMarker
		}
		return ""
	}
	noun := "subagents"
	if s.Spawns == 1 {
		noun = "subagent"
	}
	parts := []string{"◆ downshift", fmt.Sprintf("%d %s", s.Spawns, noun)}
	if s.TopModel != "" {
		parts = append(parts, fmt.Sprintf("%d on %s", s.TopModelN, s.TopModel))
	}
	if s.HasCost {
		if s.DeltaUSD < 0 {
			parts = append(parts, "over "+formatUSD(-s.DeltaUSD))
		} else {
			parts = append(parts, "saved "+formatUSD(s.DeltaUSD))
		}
	}
	parts = append(parts, fmt.Sprintf("%d on doubt", s.OnDoubt))
	if s.QuotaStale {
		parts = append(parts, QuotaStaleMarker)
	}
	return strings.Join(parts, " · ")
}

// ChildEndLine renders the one-line message shown when a child ends, from the
// usage record built for it: the model it ran on (final_model), its tokens, its
// real cost and, when a different requested model is known and priced, what
// that model would have cost. Missing parts are omitted, never guessed. It
// returns "" when nothing is known.
func ChildEndLine(ev Event) string {
	var parts []string
	ran := ""
	if ev.ToModel != "" && ev.ToModel != "unknown" {
		ran = ShortModel(ev.ToModel)
		parts = append(parts, "finished on "+ran)
	}
	if hasChildTokens(ev) {
		var n int64
		if ev.InputTokens != nil {
			n += *ev.InputTokens
		}
		if ev.OutputTokens != nil {
			n += *ev.OutputTokens
		}
		if ev.CachedTokens != nil {
			n += *ev.CachedTokens
		}
		parts = append(parts, formatTokens(n)+" tokens")
	}
	if ev.ActualCostUSD != nil {
		cost := formatUSD(*ev.ActualCostUSD)
		if ev.BaselineCostUSD != nil {
			want := ""
			if ev.FromModel != "" && ev.FromModel != "unknown" {
				want = ShortModel(ev.FromModel)
			}
			guess := ""
			if ev.BaselineSource == BaselineInferred {
				guess = ", inferred baseline"
			}
			switch {
			case want == "":
				cost += " (would be " + formatUSD(*ev.BaselineCostUSD) + guess + ")"
			case want != ran:
				cost += " (" + want + " would be " + formatUSD(*ev.BaselineCostUSD) + guess + ")"
			}
		}
		parts = append(parts, cost)
	}
	if len(parts) == 0 {
		return ""
	}
	return "↳ downshift: " + strings.Join(parts, " · ")
}

func formatUSD(v float64) string {
	if v > 0 && v < 0.005 {
		return "<$0.01"
	}
	return fmt.Sprintf("$%.2f", v)
}

func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%dk", (n+500)/1000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}
