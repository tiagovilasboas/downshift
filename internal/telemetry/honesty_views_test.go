// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Views that must honour refuted rewrites (what ran is the resolved model, no
// saving) and say when a saving rests on an inferred baseline.
package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func hvBool(b bool) *bool      { return &b }
func hvUSD(v float64) *float64 { return &v }
func hvTok(v int64) *int64     { return &v }

const hvSess = "abcdef0123456789"

// A frontier-free downshift opus -> haiku that the harness refuted: the
// resolved record says it ran opus.
func hvRefuted() []Event {
	return []Event{
		{CorrelationID: "d1", Timestamp: "2026-10-15T10:00:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", Complexity: "SIMPLE",
			FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5", QuotaStatus: QuotaAvailable, EstimatedSavings: 0.8},
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

// A usage record linked to a refuted rewrite and priced on the rewrite target
// (what childend writes when the payload carries no usage model) is still
// credited as a real saving.
func TestHonesty_RealSavedCreditsUsageOfRefutedRewrite(t *testing.T) {
	evs := append(hvRefuted(), Event{CorrelationID: "u1", Timestamp: "2026-10-15T10:02:00Z",
		Harness: "claude-code", SessionID: hvSess, Outcome: OutcomeUsage, LinkedDecision: "d1",
		FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5", InputTokens: hvTok(1000),
		ActualCostUSD: hvUSD(0.01), BaselineCostUSD: hvUSD(0.05), BaselineSource: BaselineRequested})
	s := Aggregate(evs)
	if s.RealSavedUSD != 0 {
		t.Fatalf("RealSavedUSD = %.4f for a rewrite the harness refuted (ran opus); want 0", s.RealSavedUSD)
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
		FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5", InputTokens: hvTok(1000),
		ActualCostUSD: hvUSD(0.01), BaselineCostUSD: hvUSD(0.05), BaselineSource: BaselineRequested})
	v := BuildSessionTree(evs, hvSess, TreeOptions{})
	if v.Rows[0].Model != "opus" {
		t.Fatalf("tree row model = %q, saved delta $%.2f; resolved record says opus ran", v.Rows[0].Model, v.DeltaUSD)
	}
}

// Session tree header "(without downshift: $X) · saved $Y" rests entirely on
// an inferred baseline and says nothing about it.
func TestHonesty_SessionTreeHidesInferredBaseline(t *testing.T) {
	evs := []Event{
		{CorrelationID: "d3", Timestamp: "2026-10-15T10:00:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
			ReasonCodes: []string{ReasonBaselineFromSettings}},
		{CorrelationID: "u3", Timestamp: "2026-10-15T10:02:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeUsage, LinkedDecision: "d3", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
			InputTokens: hvTok(1000), ActualCostUSD: hvUSD(0.01), BaselineCostUSD: hvUSD(0.05), BaselineSource: BaselineInferred},
	}
	out := SessionTree(evs, hvSess, TreeOptions{})
	if !strings.Contains(out, "saved") {
		t.Fatalf("precondition: no saving shown:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "inferred") {
		t.Fatalf("session tree claims a saving on an inferred baseline without saying so:\n%s", out)
	}
}

// Export JSON: stats now counts RealCostInferred, but the export drops it, so
// real_saved_usd / real_cost_delta_usd carry no sign that they rest on a guess.
func TestHonesty_ExportOmitsInferredBaselineCount(t *testing.T) {
	evs := []Event{
		{CorrelationID: "d4", Timestamp: "2026-10-15T10:00:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
			ReasonCodes: []string{ReasonBaselineFromSettings}},
		{CorrelationID: "u4", Timestamp: "2026-10-15T10:02:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeUsage, LinkedDecision: "d4", FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
			InputTokens: hvTok(1000), ActualCostUSD: hvUSD(0.01), BaselineCostUSD: hvUSD(0.05), BaselineSource: BaselineInferred},
	}
	if Aggregate(evs).RealCostInferred != 1 {
		t.Fatal("precondition: Aggregate should count 1 inferred-baseline event")
	}
	sum := ExportSummary(evs, 0, time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC))
	raw, _ := json.Marshal(sum)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	found := false
	for k := range m {
		if strings.Contains(k, "inferred") && k != "rewrite_honored_inferred" {
			found = true
		}
	}
	if !found {
		saved := 0.0
		if sum.RealSavedUSD != nil {
			saved = *sum.RealSavedUSD
		}
		t.Fatalf("export has real_saved_usd=%.2f with no inferred-baseline field: %s", saved, raw)
	}
}

// Child-end line "(opus would be $X)" on an inferred baseline, unmarked.
func TestHonesty_ChildEndLineHidesInferredBaseline(t *testing.T) {
	line := ChildEndLine(Event{Outcome: OutcomeUsage, FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
		InputTokens: hvTok(1000), ActualCostUSD: hvUSD(0.01), BaselineCostUSD: hvUSD(0.05), BaselineSource: BaselineInferred})
	if !strings.Contains(strings.ToLower(line), "inferred") && !strings.Contains(line, "~") {
		t.Fatalf("child-end line states a counterfactual on an inferred baseline as fact: %q", line)
	}
}
