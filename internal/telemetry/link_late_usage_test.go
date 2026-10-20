// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

// Claude Code stops the child before the Agent tool returns, so the usage row
// is written before the resolved row that links the agent to its decision.
func TestLinkLateUsage_UsageWrittenBeforeResolved(t *testing.T) {
	tok := int64(100)
	evs := []Event{
		{CorrelationID: "d1", Timestamp: "2026-10-20T10:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: "allow",
			Verdict: "DOWNSHIFT", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5", Tier: "small", Complexity: "TRIVIAL"},
		{CorrelationID: "u1", Timestamp: "2026-10-20T10:00:05Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeUsage,
			AgentHash: "a1", FromModel: "unknown", ToModel: "claude-haiku-4-5", Verdict: "UNKNOWN", InputTokens: &tok},
		{CorrelationID: "r1", Timestamp: "2026-10-20T10:00:06Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeResolved,
			AgentHash: "a1", LinkedDecision: "d1", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5"},
		{CorrelationID: "u2", Timestamp: "2026-10-20T10:01:00Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeUsage,
			AgentHash: "other", FromModel: "unknown", ToModel: "claude-haiku-4-5", InputTokens: &tok},
	}
	linkLateUsage(evs)
	u := evs[1]
	if u.LinkedDecision != "d1" || u.FromModel != "claude-opus-4-8" || u.Verdict != "DOWNSHIFT" || u.Tier != "small" || u.Complexity != "TRIVIAL" {
		t.Errorf("usage not joined to its decision: %+v", u)
	}
	if evs[3].LinkedDecision != "" || evs[3].FromModel != "unknown" {
		t.Errorf("a usage with no resolved row must stay as written: %+v", evs[3])
	}
	// A usage that already names a decision is never rewired.
	evs[1].LinkedDecision = "keep"
	evs[1].FromModel = "x"
	linkLateUsage(evs)
	if evs[1].LinkedDecision != "keep" || evs[1].FromModel != "x" {
		t.Errorf("an existing link was overwritten: %+v", evs[1])
	}
}

// The join feeds the tokens saved view: a child that ran cheaper than requested
// counts as saved even though its usage was logged first.
func TestTokenSavings_CountsUsageLoggedBeforeResolved(t *testing.T) {
	tok := int64(1000)
	evs := []Event{
		{CorrelationID: "d1", Timestamp: "2026-10-20T10:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeRewriteEmitted,
			Verdict: "DOWNSHIFT", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5"},
		{CorrelationID: "u1", Timestamp: "2026-10-20T10:00:05Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeUsage,
			AgentHash: "a1", FromModel: "unknown", ToModel: "claude-haiku-4-5", InputTokens: &tok},
		{CorrelationID: "r1", Timestamp: "2026-10-20T10:00:06Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeResolved,
			AgentHash: "a1", LinkedDecision: "d1", FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5"},
	}
	linkLateUsage(evs)
	if got := TokenSavings(evs).Total; got.WorkSaved != 1000 {
		t.Errorf("saved work = %d, want 1000 (%+v)", got.WorkSaved, got)
	}
}

// A child that asked for the cheapest model and was held there had nothing
// cheaper to move to: the line says so instead of reading as a miss. The usage
// is logged first, as Claude Code does.
func TestTokenSavings_AlreadyCheapestIsNamed(t *testing.T) {
	tok := int64(1000)
	evs := []Event{
		{CorrelationID: "d1", Timestamp: "2026-10-20T10:00:00Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeAllow,
			Verdict: "UPSHIFT", FromModel: "claude-haiku-5-5", ToModel: "claude-haiku-5-5", Corrections: []string{"R8_UNCONFIDENT_UPSHIFT"}},
		{CorrelationID: "u1", Timestamp: "2026-10-20T10:00:05Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeUsage,
			AgentHash: "a1", FromModel: "unknown", ToModel: "claude-haiku-5-5", InputTokens: &tok},
		{CorrelationID: "r1", Timestamp: "2026-10-20T10:00:06Z", Harness: "claude-code", SessionID: "s", Outcome: OutcomeResolved,
			AgentHash: "a1", LinkedDecision: "d1", FromModel: "claude-haiku-5-5", ToModel: "claude-haiku-5-5"},
	}
	linkLateUsage(evs)
	got := TokenSavings(evs).Total
	if got.AtFloor != 1 || got.Line() != "already on the cheapest model" {
		t.Errorf("total = %+v, line %q", got, got.Line())
	}
}
