// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestAggregate_UsageLinkageRequiresDecisionAndDeduplicatesRecords(t *testing.T) {
	decision := exportEvent(exportNow, "TRIVIAL", "DOWNSHIFT", 0.8)
	decision.CorrelationID = "decision-one"
	decision.SessionID = "hashed-session"
	usage := decision
	usage.CorrelationID = "usage-one"
	usage.Outcome = telemetry.OutcomeUsage
	usage.LinkedDecision = decision.CorrelationID
	usage.AgentHash = "hashed-agent-one"
	usage.ActualCostUSD = exportFloat64Ptr(0.02)
	usage.BaselineCostUSD = exportFloat64Ptr(0.10)

	tests := []struct {
		name   string
		change func(*telemetry.Event)
		extra  []telemetry.Event
		want   int
	}{
		{"matched decision", func(*telemetry.Event) {}, []telemetry.Event{decision}, 1},
		{"no link", func(e *telemetry.Event) { e.LinkedDecision = "" }, []telemetry.Event{decision}, 0},
		{"orphan reference", func(e *telemetry.Event) { e.LinkedDecision = "missing" }, []telemetry.Event{decision}, 0},
		{"decision outside input", func(*telemetry.Event) {}, nil, 0},
		{"different harness", func(e *telemetry.Event) { e.Harness = "cursor" }, []telemetry.Event{decision}, 0},
		{"different session", func(e *telemetry.Event) { e.SessionID = "other-session" }, []telemetry.Event{decision}, 0},
		{"legacy absent session", func(e *telemetry.Event) { e.SessionID = "" }, []telemetry.Event{decision}, 1},
		{"duplicate agent new event", func(e *telemetry.Event) { e.CorrelationID = "usage-two" }, []telemetry.Event{decision, usage}, 1},
		{"two distinct agents", func(e *telemetry.Event) { e.AgentHash = "hashed-agent-two"; e.CorrelationID = "usage-two" }, []telemetry.Event{decision, usage}, 2},
		{"resolved observation is not decision", func(*telemetry.Event) {}, []telemetry.Event{{CorrelationID: decision.CorrelationID, Harness: decision.Harness, Outcome: telemetry.OutcomeResolved}}, 0},
		{"error is not decision", func(*telemetry.Event) {}, []telemetry.Event{{CorrelationID: decision.CorrelationID, Harness: decision.Harness, Outcome: "error"}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := usage
			tt.change(&event)
			// Usage arrives first to prove the join does not depend on slice order.
			events := append([]telemetry.Event{event}, tt.extra...)
			got := telemetry.Aggregate(events)
			if got.UsageLinked != tt.want {
				t.Fatalf("UsageLinked=%d, want %d", got.UsageLinked, tt.want)
			}
			raw, _ := telemetry.ExportJSON(telemetry.ExportSummary(events, 0, exportNow))
			if !strings.Contains(raw, `"usage_records"`) || !strings.Contains(raw, `"usage_linked"`) {
				t.Fatalf("export lost raw or linked count: %s", raw)
			}
			wantRecords := 1
			for _, ev := range tt.extra {
				if ev.Outcome == telemetry.OutcomeUsage {
					wantRecords++
				}
			}
			if got.UsageRecords != wantRecords || got.RealCostEvents != wantRecords || math.Abs(got.RealSavedUSD-float64(wantRecords)*0.08) > 1e-9 {
				t.Fatalf("link filtering changed raw cost evidence: %+v", got)
			}
		})
	}
}

func TestAggregate_UsageLinkageWithoutAgentUsesEventIdentity(t *testing.T) {
	decision := exportEvent(exportNow, "TRIVIAL", "DOWNSHIFT", 0.8)
	decision.CorrelationID = "decision-one"
	usage := decision
	usage.Outcome = telemetry.OutcomeUsage
	usage.LinkedDecision = decision.CorrelationID
	usage.CorrelationID = "usage-one"
	second := usage
	second.CorrelationID = "usage-two"
	got := telemetry.Aggregate([]telemetry.Event{decision, usage, usage, second})
	if got.UsageRecords != 3 || got.UsageLinked != 2 {
		t.Fatalf("raw/linked=%d/%d, want 3/2", got.UsageRecords, got.UsageLinked)
	}
}

func TestExportSummary_UsageLinkOutsideWindowIsNotLinked(t *testing.T) {
	decision := exportEvent(exportNow.Add(-48*time.Hour), "TRIVIAL", "DOWNSHIFT", 0.8)
	decision.CorrelationID = "old-decision"
	usage := exportEvent(exportNow, "TRIVIAL", "DOWNSHIFT", 0.8)
	usage.Outcome = telemetry.OutcomeUsage
	usage.LinkedDecision = decision.CorrelationID
	got := telemetry.ExportSummary([]telemetry.Event{decision, usage}, 1, exportNow)
	if got.UsageRecords != 1 || got.UsageLinked != 0 || got.Total != 0 {
		t.Fatalf("out-of-window decision cannot satisfy linkage gate: %+v", got)
	}
}

func TestExportSummary_ObservedCostSurvivesUnknownBaseline(t *testing.T) {
	usage := exportEvent(exportNow, "TRIVIAL", "UNKNOWN", 0)
	usage.Outcome = telemetry.OutcomeUsage
	usage.ActualCostUSD = exportFloat64Ptr(0.25)
	usage.InputTokens = exportInt64Ptr(1000)
	got := telemetry.ExportSummary([]telemetry.Event{usage}, 1, exportNow)
	if got.ActualCostEvents != 1 || got.ActualCostUSD != 0.25 || got.RealCostEvents != 0 || got.RealSavedUSD != 0 {
		t.Fatalf("unknown baseline must preserve observed spend without manufacturing comparison: %+v", got)
	}
}

func TestPrintStats_UnknownBaselineDoesNotClaimUsageMissing(t *testing.T) {
	usage := exportEvent(exportNow, "TRIVIAL", "UNKNOWN", 0)
	usage.Outcome = telemetry.OutcomeUsage
	usage.ActualCostUSD = exportFloat64Ptr(0.25)
	usage.InputTokens = exportInt64Ptr(1000)
	var out strings.Builder
	telemetry.PrintStats([]telemetry.Event{usage}, telemetry.StatsOptions{}, &out)
	if !strings.Contains(out.String(), "Observed token cost") || !strings.Contains(out.String(), "known requested baseline") || strings.Contains(out.String(), "no events with token usage") {
		t.Fatalf("unknown baseline display hides observed spend: %s", out.String())
	}
}
