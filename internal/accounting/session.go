// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// BuildSessionSummary aggregates telemetry and compactor metrics for one session.
// This is the ONLY function mods and CLI commands should use to get session data.
func BuildSessionSummary(events []telemetry.Event, sessionHash string) SessionSummary {
	// Build the telemetry panel view
	panel := telemetry.BuildPanel(events, sessionHash)

	// Convert to accounting types
	summary := SessionSummary{
		SessionID: panel.Session,
		Routing:   routingFromPanel(panel),
		Spawns:    spawnsFromPanel(panel),
	}

	// Add agent type breakdown if meaningful
	if types := agentTypesFromPanel(panel); len(types) > 0 {
		summary.ByAgentType = types
	}

	// Add compactor metrics if this session has wrapped commands
	if compactor := buildCompactorMetrics(sessionHash); compactor != nil {
		summary.Compactor = compactor
	}

	return summary
}

// routingFromPanel converts telemetry.PanelView to RoutingMetrics.
func routingFromPanel(p telemetry.PanelView) RoutingMetrics {
	return RoutingMetrics{
		Switched:      p.Switched,
		Held:          p.Held,
		Observed:      p.Observed,
		WorkTokens:    p.Tokens.WorkTokens,
		WorkSaved:     p.Tokens.WorkSaved,
		CacheTokens:   p.Tokens.CacheTokens,
		CacheSaved:    p.Tokens.CacheSaved,
		WorkPricier:   p.Tokens.WorkPricier,
		CachePricier:  p.Tokens.CachePricier,
		SavedChildren: p.Tokens.SavedChildren,
		AtFloor:       p.Tokens.AtFloor,
		Line:          p.TokensLine,
	}
}

// spawnsFromPanel converts telemetry.PanelSpawn to SpawnSummary.
func spawnsFromPanel(p telemetry.PanelView) []SpawnSummary {
	if len(p.Spawns) == 0 {
		return []SpawnSummary{}
	}

	spawns := make([]SpawnSummary, len(p.Spawns))
	for i, s := range p.Spawns {
		spawns[i] = SpawnSummary{
			ToolUseKey:    s.ToolUse,
			State:         s.State,
			FromModel:     s.FromModel,
			ToModel:       s.ToModel,
			ObservedModel: s.ObservedModel,
			Label:         s.Label,
			Tokens: RoutingMetrics{
				WorkTokens:   s.Tokens.WorkTokens,
				WorkSaved:    s.Tokens.WorkSaved,
				CacheTokens:  s.Tokens.CacheTokens,
				CacheSaved:   s.Tokens.CacheSaved,
				WorkPricier:  s.Tokens.WorkPricier,
				CachePricier: s.Tokens.CachePricier,
			},
			TokensLabel: s.TokensLabel,
		}
	}
	return spawns
}

// agentTypesFromPanel converts telemetry.PanelAgentType to AgentTypeSummary.
// Returns nil if there's only one untyped group (would repeat the total).
func agentTypesFromPanel(p telemetry.PanelView) []AgentTypeSummary {
	if len(p.ByAgentType) == 0 {
		return nil
	}
	if len(p.ByAgentType) == 1 && p.ByAgentType[0].AgentType == telemetry.UntypedAgent {
		return nil
	}

	types := make([]AgentTypeSummary, len(p.ByAgentType))
	for i, t := range p.ByAgentType {
		types[i] = AgentTypeSummary{
			AgentType: t.AgentType,
			Tokens: RoutingMetrics{
				WorkTokens:  t.Tokens.WorkTokens,
				WorkSaved:   t.Tokens.WorkSaved,
				CacheTokens: t.Tokens.CacheTokens,
				CacheSaved:  t.Tokens.CacheSaved,
			},
			Line: t.Line,
		}
	}
	return types
}

// buildCompactorMetrics reads the execwrap log for this session.
// Returns nil if no wrapped commands exist for this session.
func buildCompactorMetrics(sessionHash string) *CompactorMetrics {
	if sessionHash == "" {
		return nil
	}

	path, err := execwrap.EventsPath()
	if err != nil {
		return nil
	}

	s, err := execwrap.SummarizeSession(path, sessionHash)
	if err != nil || s.Commands == 0 {
		return nil
	}

	return &CompactorMetrics{
		Commands:             s.Commands,
		Compacted:            s.Compacted,
		RawBytes:             s.RawBytes,
		SavedBytes:           s.SavedBytes,
		EstimatedTokensSaved: s.EstimatedTokensSaved,
		TokenEstimate:        s.TokenEstimate,
	}
}
