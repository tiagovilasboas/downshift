// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestObservedChildTokens_NoEvents(t *testing.T) {
	sum, rec := telemetry.ObservedChildTokens(nil)
	if sum != 0 || rec != 0 {
		t.Fatalf("sum=%d rec=%d, want 0/0", sum, rec)
	}
}

func TestObservedChildTokens_UsageWithoutTokens(t *testing.T) {
	ev := telemetry.Event{Outcome: telemetry.OutcomeUsage}
	sum, rec := telemetry.ObservedChildTokens([]telemetry.Event{ev})
	if sum != 0 || rec != 0 {
		t.Fatalf("sum=%d rec=%d, want 0/0 for usage without tokens", sum, rec)
	}
}

func TestObservedChildTokens_CatalogCostOnlyExcluded(t *testing.T) {
	// Whole dollars so an int64 cast of USD cannot hide inside a zero sum.
	actual := 12.5
	baseline := 40.0
	ev := telemetry.Event{
		Outcome:         telemetry.OutcomeUsage,
		ActualCostUSD:   &actual,
		BaselineCostUSD: &baseline,
	}
	sum, rec := telemetry.ObservedChildTokens([]telemetry.Event{ev})
	if sum != 0 || rec != 0 {
		t.Fatalf("sum=%d rec=%d, want 0/0 for cost-only usage", sum, rec)
	}

	withTokens := ev
	withTokens.InputTokens = i64Ptr(100)
	withTokens.OutputTokens = i64Ptr(40)
	withTokens.CachedTokens = i64Ptr(10)
	sum, rec = telemetry.ObservedChildTokens([]telemetry.Event{withTokens})
	if rec != 1 || sum != 150 {
		t.Fatalf("sum=%d rec=%d, want 150/1; catalog USD must not enter the token sum", sum, rec)
	}
}

func TestObservedChildTokens_WithTokens(t *testing.T) {
	ev := telemetry.Event{
		Outcome:      telemetry.OutcomeUsage,
		InputTokens:  i64Ptr(100),
		OutputTokens: i64Ptr(40),
		CachedTokens: i64Ptr(10),
	}
	sum, rec := telemetry.ObservedChildTokens([]telemetry.Event{ev})
	if rec != 1 {
		t.Fatalf("records=%d, want 1", rec)
	}
	if sum != 150 {
		t.Fatalf("sum=%d, want 150", sum)
	}
}

func TestObservedChildTokens_MeasuredZeroStillCounts(t *testing.T) {
	zero := int64(0)
	ev := telemetry.Event{
		Outcome:     telemetry.OutcomeUsage,
		InputTokens: &zero,
	}
	sum, rec := telemetry.ObservedChildTokens([]telemetry.Event{ev})
	if rec != 1 || sum != 0 {
		t.Fatalf("sum=%d rec=%d, want measured zero with one record", sum, rec)
	}
}

func TestObservedChildTokens_MixedHarnessesFilterByCaller(t *testing.T) {
	claude := telemetry.Event{
		Harness:      "claude-code",
		Outcome:      telemetry.OutcomeUsage,
		InputTokens:  i64Ptr(50),
		OutputTokens: i64Ptr(5),
	}
	cursor := telemetry.Event{
		Harness:      "cursor",
		Outcome:      telemetry.OutcomeUsage,
		OutputTokens: i64Ptr(20),
	}
	all := []telemetry.Event{claude, cursor}
	var claudeOnly []telemetry.Event
	for _, ev := range all {
		if ev.Harness == "claude-code" {
			claudeOnly = append(claudeOnly, ev)
		}
	}
	sum, rec := telemetry.ObservedChildTokens(claudeOnly)
	if sum != 55 || rec != 1 {
		t.Fatalf("claude-only sum=%d rec=%d, want 55/1", sum, rec)
	}
}

func TestObservedChildTokens_NonUsageAndCostOnlyDoNotAdd(t *testing.T) {
	actual := 5.0
	events := []telemetry.Event{
		{Outcome: telemetry.OutcomeRewriteEmitted, InputTokens: i64Ptr(999)},
		{Outcome: telemetry.OutcomeUsage, ActualCostUSD: &actual},
		{
			Outcome:      telemetry.OutcomeUsage,
			InputTokens:  i64Ptr(10),
			OutputTokens: i64Ptr(2),
		},
	}
	sum, rec := telemetry.ObservedChildTokens(events)
	if rec != 1 || sum != 12 {
		t.Fatalf("sum=%d rec=%d, want 12/1", sum, rec)
	}
}

func TestAggregate_ObservedChildFields(t *testing.T) {
	events := []telemetry.Event{
		{Outcome: telemetry.OutcomeUsage, InputTokens: i64Ptr(100)},
		{Outcome: telemetry.OutcomeUsage, ActualCostUSD: func() *float64 { v := 0.05; return &v }()},
	}
	stats := telemetry.Aggregate(events)
	if stats.ObservedChildUsage != 1 || stats.ObservedChildTokens != 100 {
		t.Fatalf("ObservedChildTokens=%d usage=%d, want 100/1",
			stats.ObservedChildTokens, stats.ObservedChildUsage)
	}
}
