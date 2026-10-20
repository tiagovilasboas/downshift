// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// exportNow is a fixed clock so window tests never depend on wall time.
var exportNow time.Time = time.Date(2026, time.September, 30, 12, 0, 0, 0, time.UTC)

func exportInt64Ptr(value int64) *int64 {
	return &value
}

func exportFloat64Ptr(value float64) *float64 {
	return &value
}

func exportUSDPtr(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// exportEvent builds a minimal Event with an explicit timestamp.
func exportEvent(stamp time.Time, complexity string, verdict string, savings float64) telemetry.Event {
	return telemetry.Event{
		Timestamp:        stamp.Format(time.RFC3339Nano),
		Harness:          "claude-code",
		Complexity:       complexity,
		FromModel:        "model-frontier",
		ToModel:          "claude-haiku-4-5",
		Verdict:          verdict,
		EstimatedSavings: savings,
	}
}

func TestExportSummary_CountsRatesAndCosts(t *testing.T) {
	real := exportEvent(exportNow, "TRIVIAL", "DOWNSHIFT", 0.8)
	real.ActualCostUSD = exportFloat64Ptr(0.02)
	real.BaselineCostUSD = exportFloat64Ptr(0.10)
	real.InputTokens = exportInt64Ptr(1000)
	real.OutputTokens = exportInt64Ptr(500)
	held := exportEvent(exportNow, "MEDIUM", "OK", 0)
	held.Corrections = []string{"guardrail-demo"}

	events := []telemetry.Event{
		real,
		exportEvent(exportNow, "COMPLEX", "UPSHIFT", 0),
		held,
		exportEvent(exportNow, "SIMPLE", "UNKNOWN", 0),
	}

	got := telemetry.ExportSummary(events, 30, exportNow)

	wantCounts := map[string]int{
		"total": 4, "downshifted": 1, "upshifted": 1, "ok": 1, "unknown": 1, "corrected": 1,
	}
	gotCounts := map[string]int{
		"total": got.Total, "downshifted": got.Downshifted, "upshifted": got.Upshifted,
		"ok": got.OK, "unknown": got.Unknown, "corrected": got.Corrected,
	}
	for key, want := range wantCounts {
		if gotCounts[key] != want {
			t.Errorf("%s = %d, want %d", key, gotCounts[key], want)
		}
	}
	if math.Abs(got.DownshiftRate-0.25) > 1e-9 {
		t.Errorf("DownshiftRate = %f, want 0.25", got.DownshiftRate)
	}
	if got.WindowDays != 30 {
		t.Errorf("WindowDays = %d, want 30", got.WindowDays)
	}
	if got.RealCostEvents != 1 {
		t.Errorf("RealCostEvents = %d, want 1", got.RealCostEvents)
	}
	if math.Abs(exportUSDPtr(got.RealSavedUSD)-0.08) > 1e-9 {
		t.Errorf("RealSavedUSD = %f, want 0.08", exportUSDPtr(got.RealSavedUSD))
	}
	if math.Abs(got.NormBaseline-4.0) > 1e-9 {
		t.Errorf("NormBaseline = %f, want 4.0", got.NormBaseline)
	}
	if got.ByComplexity["TRIVIAL"] != 1 || got.ByComplexity["COMPLEX"] != 1 {
		t.Errorf("ByComplexity = %v, want TRIVIAL=1 COMPLEX=1", got.ByComplexity)
	}
	if got.GeneratedAt != exportNow.UTC().Format(time.RFC3339Nano) {
		t.Errorf("GeneratedAt = %q, want fixed clock", got.GeneratedAt)
	}
}

func TestExportSummary_UnknownCaution(t *testing.T) {
	tests := []struct {
		name        string
		verdicts    []string
		wantCaution bool
		wantRate    string
	}{
		{name: "clean majority has no caution", verdicts: []string{"DOWNSHIFT", "DOWNSHIFT", "DOWNSHIFT", "OK"}, wantCaution: false},
		{name: "exactly 25 percent has no caution", verdicts: []string{"DOWNSHIFT", "DOWNSHIFT", "DOWNSHIFT", "UNKNOWN"}, wantCaution: false},
		{name: "above 25 percent warns with rate", verdicts: []string{"DOWNSHIFT", "UNKNOWN", "UNKNOWN"}, wantCaution: true, wantRate: "unknown_rate=66.7%"},
		{name: "all unknown warns with rate", verdicts: []string{"UNKNOWN", "UNKNOWN"}, wantCaution: true, wantRate: "unknown_rate=100.0%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := make([]telemetry.Event, 0, len(tt.verdicts))
			for _, verdict := range tt.verdicts {
				events = append(events, exportEvent(exportNow, "TRIVIAL", verdict, 0))
			}
			got := telemetry.ExportSummary(events, 30, exportNow)
			hasCaution := strings.Contains(got.Note, "Caution: unknown_rate=")
			if hasCaution != tt.wantCaution {
				t.Fatalf("caution present = %v, want %v (note=%q)", hasCaution, tt.wantCaution, got.Note)
			}
			if !strings.Contains(got.Note, "Single-user dogfood") {
				t.Errorf("note lost honesty base: %q", got.Note)
			}
			if tt.wantCaution && !strings.Contains(got.Note, tt.wantRate) {
				t.Errorf("note = %q, want rate %q", got.Note, tt.wantRate)
			}
		})
	}
}

func TestExportSummary_EmptyInputZeros(t *testing.T) {
	got := telemetry.ExportSummary(nil, 30, exportNow)

	if got.Total != 0 || got.Downshifted != 0 || got.Upshifted != 0 || got.OK != 0 || got.Unknown != 0 {
		t.Errorf("counts not zero: %+v", got)
	}
	if got.DownshiftRate != 0 || got.RealSavedUSD != nil || got.ActualCostUSD != nil || got.NormBaseline != 0 || got.NormRouted != 0 {
		t.Errorf("rates/costs wrong for empty input: %+v", got)
	}
	if got.ByComplexity == nil || len(got.ByComplexity) != 0 {
		t.Errorf("ByComplexity should be empty non-nil map, got %v", got.ByComplexity)
	}
	if strings.Contains(got.Note, "Caution:") {
		t.Errorf("empty input must not warn: %q", got.Note)
	}
}

func TestExportSummary_WindowFiltering(t *testing.T) {
	recent := exportEvent(exportNow.Add(-1*time.Hour), "TRIVIAL", "DOWNSHIFT", 0.8)
	old := exportEvent(exportNow.Add(-48*time.Hour), "TRIVIAL", "DOWNSHIFT", 0.8)
	bad := exportEvent(exportNow, "TRIVIAL", "DOWNSHIFT", 0.8)
	bad.Timestamp = "not-a-timestamp"

	gotWindow := telemetry.ExportSummary([]telemetry.Event{recent, old, bad}, 1, exportNow)
	if gotWindow.Total != 1 {
		t.Errorf("windowed Total = %d, want 1 (recent only)", gotWindow.Total)
	}

	gotAll := telemetry.ExportSummary([]telemetry.Event{recent, old, bad}, 0, exportNow)
	if gotAll.Total != 3 {
		t.Errorf("all-time Total = %d, want 3", gotAll.Total)
	}
}

func TestExportJSON_RoundTrip(t *testing.T) {
	events := []telemetry.Event{
		exportEvent(exportNow, "TRIVIAL", "DOWNSHIFT", 0.8),
		exportEvent(exportNow, "COMPLEX", "OK", 0),
	}
	summary := telemetry.ExportSummary(events, 7, exportNow)

	raw, err := telemetry.ExportJSON(summary)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	if !strings.Contains(raw, "\n") || !strings.Contains(raw, `"window_days"`) {
		t.Fatalf("expected indented JSON with window_days, got: %s", raw)
	}
	for _, field := range []string{`"total"`, `"downshifted"`, `"downshift_rate"`, `"note"`, `"by_complexity"`, `"generated_at"`, `"rewrite_honored_inferred"`, `"schema_version": 2`} {
		if !strings.Contains(raw, field) {
			t.Errorf("JSON missing field %s: %s", field, raw)
		}
	}

	var back telemetry.Summary
	if err := json.Unmarshal([]byte(raw), &back); err != nil {
		t.Fatalf("Unmarshal ExportJSON: %v", err)
	}
	if back.SchemaVersion != 2 || back.Total != summary.Total || back.Downshifted != summary.Downshifted || back.Note != summary.Note {
		t.Errorf("round-trip mismatch: got %+v, want %+v", back, summary)
	}
	if math.Abs(back.DownshiftRate-summary.DownshiftRate) > 1e-9 {
		t.Errorf("DownshiftRate round-trip = %f, want %f", back.DownshiftRate, summary.DownshiftRate)
	}
}

func TestExportJSON_OmitsAllCostUSD_SchemaV2(t *testing.T) {
	dollarKeys := []string{
		"actual_cost_usd",
		"real_saved_usd",
		"real_cost_delta_usd",
		"real_overspend_usd",
		"max_session_cost_usd",
		"max_subagent_cost_usd",
	}

	tests := []struct {
		name   string
		events []telemetry.Event
	}{
		{
			name:   "no events",
			events: nil,
		},
		{
			name: "routing only",
			events: []telemetry.Event{
				exportEvent(exportNow, "TRIVIAL", "DOWNSHIFT", 0.8),
			},
		},
		{
			name: "observed spend without baseline comparison",
			events: func() []telemetry.Event {
				u := exportEvent(exportNow, "TRIVIAL", "UNKNOWN", 0)
				u.Outcome = telemetry.OutcomeUsage
				u.ActualCostUSD = exportFloat64Ptr(0.25)
				u.InputTokens = exportInt64Ptr(1000)
				return []telemetry.Event{u}
			}(),
		},
		{
			name: "measured zero savings",
			events: func() []telemetry.Event {
				ev := exportEvent(exportNow, "COMPLEX", "UPSHIFT", 0)
				ev.ActualCostUSD = exportFloat64Ptr(6.0)
				ev.BaselineCostUSD = exportFloat64Ptr(2.0)
				return []telemetry.Event{ev}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum := telemetry.ExportSummary(tt.events, 30, exportNow)
			raw, err := telemetry.ExportJSON(sum)
			if err != nil {
				t.Fatal(err)
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal([]byte(raw), &m); err != nil {
				t.Fatal(err)
			}
			rawSchema, ok := m["schema_version"]
			if !ok {
				t.Fatalf("schema_version missing from export:\n%s", raw)
			}
			var schemaVer int
			if err := json.Unmarshal(rawSchema, &schemaVer); err != nil || schemaVer != 2 {
				t.Fatalf("expected schema_version 2, got %d (err: %v)", schemaVer, err)
			}
			for _, key := range dollarKeys {
				if _, ok := m[key]; ok {
					t.Errorf("dollar key %q must be omitted from schema v2 export, got %s", key, raw)
				}
			}
		})
	}
}
