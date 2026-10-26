// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package accounting provides a unified view of what Downshift saved in a
// session or globally. It aggregates telemetry (routing decisions, token
// savings) and execwrap (compactor bytes) into a single struct that CLI
// commands and external consumers (mods, tools) can depend on.
//
// This package is the ONLY interface external consumers should use. The JSON
// schema of SessionSummary is the contract with mod authors.
package accounting

// SessionSummary is the complete picture of what Downshift did in one session.
// CLI commands and mods consume this struct; they should not import telemetry
// or execwrap directly.
type SessionSummary struct {
	// SessionID is the first 8 chars of the hashed session id.
	SessionID string `json:"session"`

	// Routing aggregates model routing decisions and token savings.
	Routing RoutingMetrics `json:"routing"`

	// Compactor aggregates shell output compression. Omitted when the session
	// has no wrapped commands.
	Compactor *CompactorMetrics `json:"compactor,omitempty"`

	// Spawns is the per-subagent breakdown for the agents panel.
	Spawns []SpawnSummary `json:"spawns"`

	// ByAgentType is the per-agent-type breakdown. Omitted when all spawns
	// belong to the same (or no) type.
	ByAgentType []AgentTypeSummary `json:"by_agent_type,omitempty"`
}

// RoutingMetrics aggregates model routing decisions.
type RoutingMetrics struct {
	// Switched is the count of spawns where a rewrite was applied.
	Switched int `json:"switched"`
	// Held is the count of spawns held at the original model by a guardrail.
	Held int `json:"held"`
	// Observed is the count of spawns whose outcome was observed but unchanged.
	Observed int `json:"observed"`

	// Token counts, work (input+output) and cache counted apart.
	WorkTokens  int64 `json:"work_tokens"`
	WorkSaved   int64 `json:"work_saved"`
	CacheTokens int64 `json:"cache_tokens"`
	CacheSaved  int64 `json:"cache_saved"`

	// WorkPricier and CachePricier ran on a pricier model than requested.
	WorkPricier  int64 `json:"work_pricier,omitempty"`
	CachePricier int64 `json:"cache_pricier,omitempty"`

	// SavedChildren is the count of children whose tokens ran on a cheaper model.
	SavedChildren int `json:"saved_children,omitempty"`
	// AtFloor is the count of children already on the cheapest model.
	AtFloor int `json:"at_floor,omitempty"`

	// Line is the human-readable summary, e.g. "saved 62% of work · 58% of cache".
	// Empty when nothing was observed.
	Line string `json:"line,omitempty"`
}

// CompactorMetrics aggregates shell output compression from `downshift compactor`.
type CompactorMetrics struct {
	// Commands is the total number of wrapped commands.
	Commands int64 `json:"commands"`
	// Compacted is the count of commands whose output was compressed.
	Compacted int64 `json:"compacted"`
	// RawBytes is the total bytes before compression.
	RawBytes int64 `json:"raw_bytes"`
	// SavedBytes is RawBytes minus emitted bytes.
	SavedBytes int64 `json:"saved_bytes"`
	// EstimatedTokensSaved is SavedBytes / 4, labelled as an estimate.
	EstimatedTokensSaved int64 `json:"estimated_tokens_saved"`
	// TokenEstimate explains the estimate.
	TokenEstimate string `json:"token_estimate,omitempty"`
}

// SpawnSummary is one subagent's metrics.
type SpawnSummary struct {
	// ToolUseKey is the sha256 hash of the harness tool call id, for panel lookup.
	ToolUseKey string `json:"tool_use,omitempty"`
	// State is the routing state: switched, held, paused, control, blocked, kept.
	State string `json:"state"`
	// FromModel is the model the harness requested.
	FromModel string `json:"from_model"`
	// ToModel is the model Downshift picked.
	ToModel string `json:"to_model"`
	// ObservedModel is set only when the child ran on a different model than ToModel.
	ObservedModel string `json:"observed_model,omitempty"`
	// Label is the human-readable summary, e.g. "opus → sonnet (mid)".
	Label string `json:"label"`

	// Tokens is this spawn's token savings.
	Tokens RoutingMetrics `json:"tokens"`
	// TokensLabel is the human-readable summary, e.g. "48k tokens · saved from opus".
	TokensLabel string `json:"tokens_label,omitempty"`
}

// AgentTypeSummary is one agent type's aggregated metrics.
type AgentTypeSummary struct {
	// AgentType is the type label, or "untyped" when the spawn named none.
	AgentType string `json:"agent_type"`
	// Tokens is this type's token savings.
	Tokens RoutingMetrics `json:"tokens"`
	// Line is the human-readable summary, e.g. "general-purpose: 70% work · 64% cache".
	Line string `json:"line"`
}

// GlobalSummary aggregates across all sessions.
type GlobalSummary struct {
	// Days is the time window; 0 means all time.
	Days int `json:"days"`

	// Routing aggregates all routing decisions in the window.
	Routing RoutingMetrics `json:"routing"`

	// Compactor aggregates all compactor metrics in the window.
	Compactor CompactorMetrics `json:"compactor"`

	// Stats is the detailed breakdown (complexity, child status, etc.).
	Stats DetailedStats `json:"stats,omitempty"`
}

// DetailedStats carries the breakdown for `downshift stats --detail`.
type DetailedStats struct {
	// Total is the count of routing decisions.
	Total int `json:"total"`
	// Downshifted is the count where a cheaper model was applied.
	Downshifted int `json:"downshifted"`
	// Upshifted is the count where a more expensive model was applied.
	Upshifted int `json:"upshifted"`
	// NotApplied is the count where a shift was classified but not applied.
	NotApplied int `json:"not_applied"`
	// OK is the count left unchanged because no shift was warranted.
	OK int `json:"ok"`
	// Unknown is the count with unknown outcome.
	Unknown int `json:"unknown"`

	// ByComplexity counts decisions by complexity class.
	ByComplexity map[string]int `json:"by_complexity,omitempty"`
	// ByAgentType counts decisions by agent type.
	ByAgentType map[string]int `json:"by_agent_type,omitempty"`
	// ChildStatus counts how children ended.
	ChildStatus map[string]int `json:"child_status,omitempty"`
}

// ComparisonReport is the routed-vs-control analysis.
type ComparisonReport struct {
	// Days is the time window; 0 means all time.
	Days int `json:"days"`

	// Routed is the group where routing was applied.
	Routed GroupSummary `json:"routed"`
	// Control is the group left unrouted for baseline comparison.
	Control GroupSummary `json:"control"`

	// Excluded is the count of spawns excluded from both groups.
	Excluded int `json:"excluded"`

	// Enough is true when both groups have enough data for the comparison.
	Enough bool `json:"enough"`
	// Message explains why data is insufficient, if Enough is false.
	Message string `json:"message,omitempty"`

	// Savings is the token-per-task comparison, only set when Enough is true.
	Savings *SavingsEstimate `json:"savings,omitempty"`
}

// GroupSummary is one group's side of the comparison.
type GroupSummary struct {
	// Spawns is the count of routing decisions.
	Spawns int `json:"spawns"`
	// Ended is the count with an observed end.
	Ended int `json:"ended"`
	// Completed is the count of completed tasks with priced usage.
	Completed int `json:"completed"`

	// TokensPerTask is the average tokens per completed task.
	TokensPerTask float64 `json:"tokens_per_task"`
	// ReworkRate is the share of ended spawns that were reworked.
	ReworkRate float64 `json:"rework_rate"`
}

// SavingsEstimate is the token-per-task comparison with confidence interval.
type SavingsEstimate struct {
	// PerTaskTokens is control minus routed tokens per task.
	PerTaskTokens float64 `json:"per_task_tokens"`
	// PerTaskTokensLow is the 2.5th percentile.
	PerTaskTokensLow float64 `json:"per_task_tokens_low"`
	// PerTaskTokensHigh is the 97.5th percentile.
	PerTaskTokensHigh float64 `json:"per_task_tokens_high"`
}
