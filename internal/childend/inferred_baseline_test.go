// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// A usage record priced against a decision whose baseline was inferred
// from settings must not carry baseline_source "requested".
func TestBuildUsageEvent_InferredBaselineSource(t *testing.T) {
	target := &telemetry.Event{CorrelationID: "d1", Harness: "claude-code", SessionID: "s1",
		Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
		FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5",
		ReasonCodes: []string{telemetry.ReasonBaselineFromSettings}}
	ev := buildUsageEvent("claude-code", telemetry.TokenUsage{InputTokens: 1000, OutputTokens: 500},
		"claude-haiku-5-5", "s1", "test", target, catalog.Load(), time.Now())
	if ev.BaselineCostUSD == nil {
		t.Fatal("no baseline priced")
	}
	if ev.BaselineSource == telemetry.BaselineRequested {
		t.Fatalf("inferred baseline recorded as baseline_source=%q (baseline $%.6f, actual $%.6f)",
			ev.BaselineSource, *ev.BaselineCostUSD, *ev.ActualCostUSD)
	}
}
