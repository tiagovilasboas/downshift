// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "strings"

// SpawnSummary is the shape of one session's subagent spawns: counts only.
// It is built from routing metadata already in the event log (tier, agent type
// label, reason codes) and never holds task text.
type SpawnSummary struct {
	Spawns int `json:"spawns"`
	// ByTier counts spawns by the tier they ran on (small, mid, frontier,
	// unknown); ByAgentType counts them by squad agent name, else by AgentTypeLabel.
	ByTier      map[string]int `json:"by_tier"`
	ByAgentType map[string]int `json:"by_agent_type"`
	// AboveAgentDefault counts spawns that pinned a model above the tier their
	// agent declares (reason code ABOVE_AGENT_DEFAULT).
	AboveAgentDefault int `json:"above_agent_default"`
}

// SummarizeSpawns counts the routing decisions of one session. An empty
// sessionHash summarizes nothing: a view never mixes sessions.
func SummarizeSpawns(events []Event, sessionHash string) SpawnSummary {
	out := SpawnSummary{ByTier: map[string]int{}, ByAgentType: map[string]int{}}
	if sessionHash == "" {
		return out
	}
	for _, ev := range events {
		if ev.SessionID != sessionHash || !IsDecisionRow(ev) {
			continue
		}
		out.Spawns++
		tier := strings.ToLower(ev.Tier)
		if tier == "" {
			tier = "unknown"
		}
		out.ByTier[tier]++
		// A squad agent counts under its own name; any other spawn under the
		// stored label (custom agents stay "custom" for privacy).
		agent := ev.AgentName
		if agent == "" {
			agent = ev.AgentType
		}
		if agent == "" {
			agent = "unspecified"
		}
		out.ByAgentType[agent]++
		for _, code := range ev.ReasonCodes {
			if code == ReasonAboveAgentDefault {
				out.AboveAgentDefault++
				break
			}
		}
	}
	return out
}
