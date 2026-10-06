// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func int64Ptr(v int64) *int64 { return &v }

func float64Ptr(v float64) *float64 { return &v }

func TestCostUSD_BasicMath(t *testing.T) {
	m := core.Model{ID: "m", InputM: 1, OutputM: 5}
	u := telemetry.TokenUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000}
	if got := telemetry.CostUSD(m, u); math.Abs(got-6.0) > 1e-9 {
		t.Errorf("CostUSD = %f, want 6.0", got)
	}
}

func TestCostUSD_CachedBilledAtInputRate(t *testing.T) {
	m := core.Model{ID: "m", InputM: 2, OutputM: 10}
	u := telemetry.TokenUsage{CachedTokens: 1_000_000}
	if got := telemetry.CostUSD(m, u); math.Abs(got-2.0) > 1e-9 {
		t.Errorf("CostUSD cached-only = %f, want 2.0", got)
	}
	// Mixed: 500k input + 500k cached at $2/1M input = $2.00.
	u = telemetry.TokenUsage{InputTokens: 500_000, CachedTokens: 500_000}
	if got := telemetry.CostUSD(m, u); math.Abs(got-2.0) > 1e-9 {
		t.Errorf("CostUSD input+cached = %f, want 2.0", got)
	}
}

func TestCostUSD_NegativeClamped(t *testing.T) {
	m := core.Model{ID: "m", InputM: 1, OutputM: 5}
	u := telemetry.TokenUsage{InputTokens: -100, OutputTokens: -50, CachedTokens: -10}
	if got := telemetry.CostUSD(m, u); got != 0 {
		t.Errorf("CostUSD all-negative = %f, want 0", got)
	}
	// Negative input clamped, positive output kept.
	u = telemetry.TokenUsage{InputTokens: -100, OutputTokens: 1_000_000}
	if got := telemetry.CostUSD(m, u); math.Abs(got-5.0) > 1e-9 {
		t.Errorf("CostUSD neg-input = %f, want 5.0", got)
	}
}

func TestRealSavingsUSD_UpshiftReturnsZero(t *testing.T) {
	cheap := core.Model{ID: "cheap", InputM: 1, OutputM: 1}
	pricey := core.Model{ID: "pricey", InputM: 10, OutputM: 10}
	u := telemetry.TokenUsage{InputTokens: 1_000_000, OutputTokens: 1_000_000}
	// Routed is more expensive (upshift): must return 0, never negative.
	if got := telemetry.RealSavingsUSD(cheap, pricey, u); got != 0 {
		t.Errorf("RealSavingsUSD upshift = %f, want 0", got)
	}
	// Same model (OK): also 0.
	if got := telemetry.RealSavingsUSD(cheap, cheap, u); got != 0 {
		t.Errorf("RealSavingsUSD same-model = %f, want 0", got)
	}
	// Downshift: baseline $20, routed $2, savings $18.
	if got := telemetry.RealSavingsUSD(pricey, cheap, u); math.Abs(got-18.0) > 1e-9 {
		t.Errorf("RealSavingsUSD downshift = %f, want 18.0", got)
	}
}

func TestEvent_HasRealCost(t *testing.T) {
	withBoth := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	withBoth.ActualCostUSD = float64Ptr(1.0)
	withBoth.BaselineCostUSD = float64Ptr(2.0)
	if !withBoth.HasRealCost() {
		t.Error("HasRealCost = false, want true when both pointers set")
	}
	onlyActual := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	onlyActual.ActualCostUSD = float64Ptr(1.0)
	if onlyActual.HasRealCost() {
		t.Error("HasRealCost = true, want false when baseline missing")
	}
	onlyBaseline := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	onlyBaseline.BaselineCostUSD = float64Ptr(2.0)
	if onlyBaseline.HasRealCost() {
		t.Error("HasRealCost = true, want false when actual missing")
	}
	neither := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	if neither.HasRealCost() {
		t.Error("HasRealCost = true, want false when both missing")
	}
}

func TestEvent_RealSavedUSD(t *testing.T) {
	ev := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	ev.ActualCostUSD = float64Ptr(2.0)
	ev.BaselineCostUSD = float64Ptr(5.0)
	if got := ev.RealSavedUSD(); math.Abs(got-3.0) > 1e-9 {
		t.Errorf("RealSavedUSD = %f, want 3.0", got)
	}
	// Inverted (routed cost more): clamped to 0.
	ev.ActualCostUSD = float64Ptr(7.0)
	ev.BaselineCostUSD = float64Ptr(5.0)
	if got := ev.RealSavedUSD(); got != 0 {
		t.Errorf("RealSavedUSD inverted = %f, want 0", got)
	}
	// Missing: 0.
	ev = makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	if got := ev.RealSavedUSD(); got != 0 {
		t.Errorf("RealSavedUSD missing = %f, want 0", got)
	}
}

func TestEvent_TokenUsageOrZero(t *testing.T) {
	// Nil-safe: all missing -> zeros.
	ev := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	if u := ev.TokenUsageOrZero(); u.InputTokens != 0 || u.OutputTokens != 0 || u.CachedTokens != 0 {
		t.Errorf("TokenUsageOrZero nil = %+v, want zeros", u)
	}
	// Values pass through.
	ev.InputTokens = int64Ptr(100)
	ev.OutputTokens = int64Ptr(200)
	ev.CachedTokens = int64Ptr(50)
	if u := ev.TokenUsageOrZero(); u.InputTokens != 100 || u.OutputTokens != 200 || u.CachedTokens != 50 {
		t.Errorf("TokenUsageOrZero = %+v, want {100 200 50}", u)
	}
	// Negatives clamped to 0.
	ev.InputTokens = int64Ptr(-5)
	ev.OutputTokens = int64Ptr(-10)
	ev.CachedTokens = int64Ptr(-1)
	if u := ev.TokenUsageOrZero(); u.InputTokens != 0 || u.OutputTokens != 0 || u.CachedTokens != 0 {
		t.Errorf("TokenUsageOrZero negative = %+v, want zeros", u)
	}
}

func TestAggregate_RealCostEvents(t *testing.T) {
	withCost := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	withCost.ActualCostUSD = float64Ptr(1.0)
	withCost.BaselineCostUSD = float64Ptr(4.0) // saves 3.0
	withCost2 := makeEvent("cc", "SIMPLE", "DOWNSHIFT", 0.5)
	withCost2.ActualCostUSD = float64Ptr(2.0)
	withCost2.BaselineCostUSD = float64Ptr(3.0) // saves 1.0
	withoutCost := makeEvent("cc", "MEDIUM", "OK", 0)

	s := telemetry.Aggregate([]telemetry.Event{withCost, withCost2, withoutCost})
	if s.Total != 3 {
		t.Errorf("Total = %d, want 3", s.Total)
	}
	if s.RealCostEvents != 2 {
		t.Errorf("RealCostEvents = %d, want 2", s.RealCostEvents)
	}
	if math.Abs(s.RealSavedUSD-4.0) > 1e-9 {
		t.Errorf("RealSavedUSD = %f, want 4.0", s.RealSavedUSD)
	}
	// Norm logic untouched: 2 downshifts at 0.5 + 1 OK.
	saved, frac := s.NormSaved()
	if math.Abs(saved-1.0) > 1e-9 {
		t.Errorf("NormSaved = %f, want 1.0", saved)
	}
	if math.Abs(frac-1.0/3.0) > 1e-9 {
		t.Errorf("NormFrac = %f, want %f", frac, 1.0/3.0)
	}
}

func TestOldJSONWithoutNewFields_Parses(t *testing.T) {
	raw := `{"correlation_id":"abc","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `",` +
		`"source":"hook","agent":"subagent","harness":"cc","complexity":"TRIVIAL",` +
		`"requested_model":"m1","final_model":"m2","requested_reasoning_effort":"unknown",` +
		`"final_reasoning_effort":"low","verdict":"DOWNSHIFT","tier":"SMALL",` +
		`"policy_version":"core-route-v1","binary_version":"test","outcome":"rewrite_emitted",` +
		`"estimated_savings":0.5}`
	var ev telemetry.Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal old JSON: %v", err)
	}
	if ev.HasRealCost() {
		t.Error("HasRealCost = true for old JSON, want false")
	}
	if ev.RealSavedUSD() != 0 {
		t.Errorf("RealSavedUSD = %f, want 0 for old JSON", ev.RealSavedUSD())
	}
	if u := ev.TokenUsageOrZero(); u.InputTokens != 0 || u.OutputTokens != 0 || u.CachedTokens != 0 {
		t.Errorf("TokenUsageOrZero old JSON = %+v, want zeros", u)
	}
}

func TestMarshal_OmitsAbsentRealCostFields(t *testing.T) {
	ev := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	line, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{"input_tokens", "output_tokens", "cached_tokens", "actual_cost_usd", "baseline_cost_usd"} {
		if strings.Contains(string(line), key) {
			t.Errorf("marshalled event without real cost contains %q", key)
		}
	}
}

func TestPrintStats_RealCostSection(t *testing.T) {
	withCost := makeEvent("cc", "TRIVIAL", "DOWNSHIFT", 0.5)
	withCost.ActualCostUSD = float64Ptr(1.0)
	withCost.BaselineCostUSD = float64Ptr(4.0)

	var buf strings.Builder
	telemetry.PrintStats([]telemetry.Event{withCost}, telemetry.StatsOptions{Days: 0}, &buf)
	if !strings.Contains(buf.String(), "Real provider cost (1 events with token usage)") {
		t.Errorf("missing real-cost section, got:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "$") {
		t.Error("real-cost section should show saved USD")
	}

	var buf2 strings.Builder
	telemetry.PrintStats([]telemetry.Event{makeEvent("cc", "TRIVIAL", "OK", 0)}, telemetry.StatsOptions{Days: 0}, &buf2)
	if !strings.Contains(buf2.String(), "Real cost: no events with token usage yet (wire the PostToolUse hook to populate them).") {
		t.Errorf("missing no-real-cost line, got:\n%s", buf2.String())
	}
}
