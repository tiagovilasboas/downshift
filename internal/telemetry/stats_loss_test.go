// Copyright (c) 2026 Tiago Vilas Boas.
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

func costEvent(actual, baseline float64) telemetry.Event {
	ev := makeEvent("cc", "COMPLEX", "UPSHIFT", 0)
	ev.ActualCostUSD = float64Ptr(actual)
	ev.BaselineCostUSD = float64Ptr(baseline)
	return ev
}

func TestEvent_RealDeltaUSD_SignedAndClampPreserved(t *testing.T) {
	over := costEvent(7.0, 5.0)
	if got := over.RealDeltaUSD(); math.Abs(got-(-2.0)) > 1e-9 {
		t.Errorf("RealDeltaUSD overspend = %f, want -2.0", got)
	}
	if got := over.RealSavedUSD(); got != 0 {
		t.Errorf("RealSavedUSD must stay clamped, got %f", got)
	}
	under := costEvent(2.0, 5.0)
	if got := under.RealDeltaUSD(); math.Abs(got-3.0) > 1e-9 {
		t.Errorf("RealDeltaUSD saving = %f, want 3.0", got)
	}
	if got := makeEvent("cc", "SIMPLE", "OK", 0).RealDeltaUSD(); got != 0 {
		t.Errorf("RealDeltaUSD without costs = %f, want 0", got)
	}
}

func TestAggregate_RealCostDeltaAndOverspend(t *testing.T) {
	saves := costEvent(1.0, 4.0)  // +3
	loses := costEvent(6.0, 2.0)  // -4
	loses2 := costEvent(3.0, 2.5) // -0.5
	s := telemetry.Aggregate([]telemetry.Event{saves, loses, loses2})
	if math.Abs(s.RealSavedUSD-3.0) > 1e-9 {
		t.Errorf("RealSavedUSD = %f, want 3.0 (clamped per event)", s.RealSavedUSD)
	}
	if math.Abs(s.RealCostDeltaUSD-(-1.5)) > 1e-9 {
		t.Errorf("RealCostDeltaUSD = %f, want -1.5", s.RealCostDeltaUSD)
	}
	if math.Abs(s.RealOverspendUSD-4.5) > 1e-9 {
		t.Errorf("RealOverspendUSD = %f, want 4.5", s.RealOverspendUSD)
	}
}

func TestAggregate_UsageRecordLossCounts(t *testing.T) {
	usage := costEvent(5.0, 2.0)
	usage.Outcome = telemetry.OutcomeUsage
	s := telemetry.Aggregate([]telemetry.Event{usage})
	if s.Total != 0 || math.Abs(s.RealOverspendUSD-3.0) > 1e-9 || math.Abs(s.RealCostDeltaUSD-(-3.0)) > 1e-9 {
		t.Errorf("Total=%d overspend=%f delta=%f, want 0/3/-3", s.Total, s.RealOverspendUSD, s.RealCostDeltaUSD)
	}
}

func TestFromDecision_UpshiftCostRatio(t *testing.T) {
	mid := core.Model{ID: "m-mid", InputM: 3, OutputM: 15}
	top := core.Model{ID: "m-top", InputM: 5, OutputM: 25}
	ev := telemetry.FromDecision(core.Decision{Verdict: core.VerdictUpshift, Model: top, Baseline: core.Baseline{CurrentModel: mid}}, telemetry.NewCorrelationID(), "t")
	if math.Abs(ev.UpshiftCostRatio-30.0/18.0) > 1e-9 {
		t.Errorf("UpshiftCostRatio = %f, want %f", ev.UpshiftCostRatio, 30.0/18.0)
	}
	unknown := telemetry.FromDecision(core.Decision{Verdict: core.VerdictUpshift, Model: top, Baseline: core.Baseline{CurrentModel: core.Model{ID: "x"}}}, telemetry.NewCorrelationID(), "t")
	if unknown.UpshiftCostRatio != 0 {
		t.Errorf("unknown requested price must leave the ratio absent, got %f", unknown.UpshiftCostRatio)
	}
	down := telemetry.FromDecision(core.Decision{Verdict: core.VerdictDownshift, Model: mid, Baseline: core.Baseline{CurrentModel: top}}, telemetry.NewCorrelationID(), "t")
	if down.UpshiftCostRatio != 0 {
		t.Errorf("downshift must not carry an upshift ratio, got %f", down.UpshiftCostRatio)
	}
}

func TestAggregate_AppliedUpshiftAddsRoutedUnits(t *testing.T) {
	priced := makeEvent("cc", "COMPLEX", "UPSHIFT", 0)
	priced.UpshiftCostRatio = 2.5
	neutral := makeEvent("cc", "COMPLEX", "UPSHIFT", 0) // ratio unknown
	notApplied := makeEvent("cc", "COMPLEX", "UPSHIFT", 0)
	notApplied.UpshiftCostRatio = 4
	notApplied.Outcome = telemetry.OutcomeAllow // spawn unchanged: no extra cost

	s := telemetry.Aggregate([]telemetry.Event{priced, neutral, notApplied})
	if s.Upshifted != 2 || s.NotApplied != 1 {
		t.Fatalf("Upshifted=%d NotApplied=%d, want 2/1", s.Upshifted, s.NotApplied)
	}
	if math.Abs(s.NormRouted-(2.5+1.0+1.0)) > 1e-9 {
		t.Errorf("NormRouted = %f, want 4.5", s.NormRouted)
	}
	if saved, _ := s.NormSaved(); saved >= 0 {
		t.Errorf("NormSaved = %f, want negative (the router overspent)", saved)
	}
}

func TestExportSummary_LossFieldsRoundTrip(t *testing.T) {
	loses := costEvent(6.0, 2.0)
	loses.Timestamp = exportNow.Format(time.RFC3339Nano)
	got := telemetry.ExportSummary([]telemetry.Event{loses}, 30, exportNow)
	raw, err := telemetry.ExportJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"real_cost_delta_usd": -4`, `"real_overspend_usd": 4`} {
		if !strings.Contains(raw, key) {
			t.Errorf("export missing %s:\n%s", key, raw)
		}
	}
	var back telemetry.Summary
	if err := json.Unmarshal([]byte(raw), &back); err != nil {
		t.Fatal(err)
	}
	if back.RealCostDeltaUSD != -4 || back.RealOverspendUSD != 4 || back.RealSavedUSD == nil || *back.RealSavedUSD != 0 {
		t.Errorf("round trip delta=%f overspend=%f saved=%v", back.RealCostDeltaUSD, back.RealOverspendUSD, back.RealSavedUSD)
	}

	// Additive: a clean window omits both keys, and older JSON still parses.
	clean, _ := telemetry.ExportJSON(telemetry.ExportSummary(nil, 30, exportNow))
	for _, key := range []string{"real_cost_delta_usd", "real_overspend_usd", "real_saved_usd", "actual_cost_usd"} {
		if strings.Contains(clean, key) {
			t.Errorf("unmeasured export must omit %q:\n%s", key, clean)
		}
	}
	var old telemetry.Summary
	if err := json.Unmarshal([]byte(`{"total":1,"real_saved_usd":0.5}`), &old); err != nil || old.RealCostDeltaUSD != 0 {
		t.Errorf("older export must parse unchanged: %v %+v", err, old)
	}
}

func TestPrintStats_ShowsLoss(t *testing.T) {
	var sb strings.Builder
	telemetry.PrintStats([]telemetry.Event{costEvent(6.0, 2.0)}, telemetry.StatsOptions{Detail: true}, &sb)
	out := sb.String()
	for _, want := range []string{"Real saved", "Real cost delta", "-4.00", "Real overspend"} {
		if !strings.Contains(out, want) {
			t.Errorf("stats output missing %q:\n%s", want, out)
		}
	}
}
