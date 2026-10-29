// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

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

func TestPrintStats_UnknownBaselineDoesNotClaimUsageMissing(t *testing.T) {
	usage := exportEvent(exportNow, "TRIVIAL", "UNKNOWN", 0)
	usage.Outcome = telemetry.OutcomeUsage
	usage.InputTokens = exportInt64Ptr(1000)
	var out strings.Builder
	telemetry.PrintStats([]telemetry.Event{usage}, telemetry.StatsOptions{Detail: true}, &out)
	// The tokens stay visible; the spend itself is report's (priced now).
	if !strings.Contains(out.String(), "Observed child tokens") || strings.Contains(out.String(), "unobserved\n") || strings.Contains(out.String(), "$") {
		t.Fatalf("unknown baseline display hides observed tokens or prints dollars: %s", out.String())
	}
}
