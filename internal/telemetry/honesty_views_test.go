// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Views that must honour refuted rewrites (what ran is the resolved model, no
// saving) and say when a saving rests on an inferred baseline.
package telemetry

import (
	"strings"
	"testing"
)

func hvBool(b bool) *bool  { return &b }
func hvTok(v int64) *int64 { return &v }

const hvSess = "abcdef0123456789"

// A frontier-free downshift opus -> haiku that the harness refuted: the
// resolved record says it ran opus.
func hvRefuted() []Event {
	return []Event{
		{CorrelationID: "d1", Timestamp: "2026-10-15T10:00:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", Complexity: "SIMPLE",
			FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5", QuotaStatus: QuotaAvailable},
		{CorrelationID: "r1", Timestamp: "2026-10-15T10:01:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeResolved, LinkedDecision: "d1", FromModel: "claude-haiku-5-5", ToModel: "claude-opus-5-5",
			RewriteHonored: hvBool(false)},
	}
}

// ModelMix ("Models:" headline) must not count the model of a refuted rewrite.
func TestHonesty_ModelMixCountsRefutedRewriteTarget(t *testing.T) {
	s := Aggregate(hvRefuted())
	if s.Downshifted != 0 {
		t.Fatalf("precondition: refuted rewrite counted downshifted=%d", s.Downshifted)
	}
	if s.ModelMix["haiku"] != 0 || s.ModelMix["opus"] != 1 {
		t.Fatalf("ModelMix = %v; refuted rewrite should count opus (what ran), not haiku", s.ModelMix)
	}
}

// FrontierOnDoubt is "money really spent at frontier on doubt"; a refuted
// frontier rewrite spent nothing at frontier.
func TestHonesty_FrontierOnDoubtCountsRefutedRewrite(t *testing.T) {
	evs := []Event{
		{CorrelationID: "d2", Timestamp: "2026-10-15T10:00:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeRewriteEmitted, Verdict: "UPSHIFT", Tier: "frontier",
			FromModel: "claude-sonnet-5-5", ToModel: "claude-opus-5-5", ReasonCodes: []string{ReasonClassifierUnconfident}},
		{CorrelationID: "r2", Timestamp: "2026-10-15T10:01:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeResolved, LinkedDecision: "d2", ToModel: "claude-sonnet-5-5", RewriteHonored: hvBool(false)},
	}
	s := Aggregate(evs)
	if s.FrontierOnDoubt != 0 {
		t.Fatalf("FrontierOnDoubt = %d for a frontier rewrite the harness refuted; want 0", s.FrontierOnDoubt)
	}
}

// Statusline: "N on <model>" must not name a model the refuted child never ran.
func TestHonesty_StatuslineTopModelUsesRefutedTarget(t *testing.T) {
	s := SessionSummary(hvRefuted(), hvSess)
	if s.TopModel != "opus" {
		t.Fatalf("statusline TopModel = %q; harness resolved opus for the only child", s.TopModel)
	}
}

// Session tree: the usage record (written without a payload model) carries the
// refuted rewrite target; rowModel prefers it over the resolved observation.
func TestHonesty_SessionTreeRowModelPrefersRefutedUsageModel(t *testing.T) {
	evs := append(hvRefuted(), Event{CorrelationID: "u1", Timestamp: "2026-10-15T10:02:00Z",
		Harness: "claude-code", SessionID: hvSess, Outcome: OutcomeUsage, LinkedDecision: "d1",
		FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5", InputTokens: hvTok(1000), BaselineSource: BaselineRequested})
	v := BuildSessionTree(evs, hvSess, TreeOptions{})
	if v.Rows[0].Model != "opus" {
		t.Fatalf("tree row model = %q, saved %d work tokens; resolved record says opus ran", v.Rows[0].Model, v.Tokens.WorkSaved)
	}
}

// Session tree header "tokens saved: …" rests entirely on an inferred baseline
// and must say so.
func TestHonesty_SessionTreeHidesInferredBaseline(t *testing.T) {
	evs := []Event{
		{CorrelationID: "d3", Timestamp: "2026-10-15T10:00:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
			ReasonCodes: []string{ReasonBaselineFromSettings}},
		{CorrelationID: "u3", Timestamp: "2026-10-15T10:02:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeUsage, LinkedDecision: "d3", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
			InputTokens: hvTok(1000), BaselineSource: BaselineInferred},
	}
	out := SessionTree(evs, hvSess, TreeOptions{})
	if !strings.Contains(out, "saved") {
		t.Fatalf("precondition: no saving shown:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "inferred") {
		t.Fatalf("session tree claims a saving on an inferred baseline without saying so:\n%s", out)
	}
}
