// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"strings"
	"testing"
)

func refBool(b bool) *bool    { return &b }
func refF(f float64) *float64 { return &f }

func refShift(id, session string) Event {
	return Event{CorrelationID: id, Timestamp: "2026-10-15T10:00:00Z", Harness: "claude-code", SessionID: session,
		Verdict: "DOWNSHIFT", Outcome: OutcomeRewriteEmitted, FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
		Tier: "SMALL", Complexity: "TRIVIAL", EstimatedSavings: 0.8, QuotaStatus: QuotaAvailable}
}

// a rewrite the harness refused (resolved model != rewrite) must not be a saving.
func TestAggregate_RefutedRewriteSavesNothing(t *testing.T) {
	events := []Event{
		refShift("d1", "s1"),
		{Outcome: OutcomeResolved, Harness: "claude-code", SessionID: "s1", LinkedDecision: "d1",
			Timestamp: "2026-10-15T10:00:01Z", ToModel: "claude-opus-5-5", RewriteHonored: refBool(false)},
	}
	s := Aggregate(events)
	saved, _ := s.NormSaved()
	if s.Downshifted != 0 || saved != 0 {
		t.Fatalf("refuted rewrite counted: Downshifted=%d normalised saved=%.2f units", s.Downshifted, saved)
	}
}

// a resolved record that refutes the rewrite must beat the later-spawn heuristic.
func TestCountInferredHonored_RefutedBeatsLaterSpawn(t *testing.T) {
	later := refShift("d2", "s1")
	later.FromModel, later.Verdict, later.Outcome, later.Timestamp = "claude-haiku-5-5", "OK", OutcomeAllow, "2026-10-15T10:05:00Z"
	events := []Event{
		refShift("d1", "s1"),
		{Outcome: OutcomeResolved, Harness: "claude-code", SessionID: "s1", LinkedDecision: "d1",
			Timestamp: "2026-10-15T10:00:01Z", ToModel: "claude-opus-5-5", RewriteHonored: refBool(false)},
		later,
	}
	shifted, honored := CountInferredHonored(events)
	if honored != 0 {
		t.Fatalf("rewrite refuted by resolved record still counted honored: shifted=%d honored=%d", shifted, honored)
	}
}

// real saving whose baseline was inferred from settings must not be presented as measured.
func TestPrintStats_MarksInferredBaseline(t *testing.T) {
	d := refShift("d1", "s1")
	d.ReasonCodes = []string{ReasonBaselineFromSettings, ReasonClassifierConfident}
	u := Event{Outcome: OutcomeUsage, Harness: "claude-code", SessionID: "s1", LinkedDecision: "d1",
		Timestamp: "2026-10-15T10:10:00Z", FromModel: d.FromModel, ToModel: d.ToModel,
		ActualCostUSD: refF(0.10), BaselineCostUSD: refF(0.50), BaselineSource: BaselineRequested}
	var buf bytes.Buffer
	PrintStats([]Event{d, u}, StatsOptions{Detail: true}, &buf)
	out := buf.String()
	if !strings.Contains(strings.ToLower(out), "inferred") {
		t.Fatalf("saving from an inferred baseline printed without any inferred marker:\n%s", out)
	}
}
