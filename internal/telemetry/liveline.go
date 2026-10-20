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
	Spawns    int    // routing decisions made for the session
	TopModel  string // short name of the model most children ran on
	TopModelN int    // how many children ran on TopModel
	// Tokens is the session's tokens saved from the requested model (work and
	// cache apart). Live views show tokens only; dollars are `downshift report`'s.
	Tokens   TokenSaving
	Switched int // spawns whose model or effort Downshift changed
	Held     int // spawns Downshift considered and left unchanged
	OnDoubt  int // frontier decisions resting on an unconfident classification
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
	var mine []Event
	for _, ev := range events {
		if ev.SessionID != sessionHash {
			continue
		}
		mine = append(mine, ev)
	}
	s.Tokens = TokenSavings(mine).Total
	for _, ev := range mine {
		if IsCostOnlyOutcome(ev.Outcome) || IsFailureOutcome(ev.Outcome) {
			continue
		}
		s.Spawns++
		switch SpawnStateOf(ev) {
		case StateSwitched:
			s.Switched++
		case StateHeld:
			s.Held++
		}
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
	parts = append(parts, fmt.Sprintf("%d switched · %d held", s.Switched, s.Held))
	if s.TopModel != "" {
		parts = append(parts, fmt.Sprintf("%d on %s", s.TopModelN, s.TopModel))
	}
	if s.Tokens.Observed() {
		parts = append(parts, fmt.Sprintf("tokens saved work %s · cache %s", percentText(s.Tokens.WorkPercent()), percentText(s.Tokens.CachePercent())))
	}
	parts = append(parts, fmt.Sprintf("%d on doubt", s.OnDoubt))
	if s.QuotaStale {
		parts = append(parts, QuotaStaleMarker)
	}
	return strings.Join(parts, " · ")
}

// ChildEndLine renders the one-line message shown when a child ends, from the
// usage record built for it: the model it ran on (final_model), its work and
// cache tokens and, when a different requested model is known, whether those
// tokens were saved from it (ran on a cheaper model) or ran on a pricier one.
// No dollars: the live line shows tokens only. Missing parts are omitted,
// never guessed. It returns "" when nothing is known.
func ChildEndLine(ev Event) string {
	var parts []string
	ran := ""
	if ev.ToModel != "" && ev.ToModel != "unknown" {
		ran = ShortModel(ev.ToModel)
		parts = append(parts, "finished on "+ran)
	}
	if hasChildTokens(ev) {
		c := classesOf(ev.TokenUsageOrZero())
		parts = append(parts, fmt.Sprintf("%s tokens (work %s, cache %s)", formatTokens(c.Total), formatTokens(c.Input+c.Output), formatTokens(c.Cache())))
	}
	if want := ev.FromModel; want != "" && want != "unknown" && ShortModel(want) != ran && ev.HasTokenUsage() {
		guess := ""
		if ev.BaselineSource == BaselineInferred {
			guess = " (inferred baseline)"
		}
		switch tierDirection(ev.Harness, want, ev.ToModel) {
		case TokenCheaper:
			parts = append(parts, "saved from "+ShortModel(want)+guess)
		case TokenPricier:
			parts = append(parts, "pricier than "+ShortModel(want)+guess)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "↳ downshift: " + strings.Join(parts, " · ")
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
