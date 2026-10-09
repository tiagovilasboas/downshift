// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"fmt"
	"time"
)

// baseNote is the fixed honesty disclaimer attached to every export.
// Single-user dogfood data is directional, never a billing claim.
const baseNote string = "Single-user dogfood: directional only, not billing. Compare your provider dashboard before/after deploying downshift."

// unknownCautionThreshold is the unknown verdict share above which the
// downshift rate is provisional (legacy-schema and fail-open error events
// inflate unknown without telling us about routing quality).
const unknownCautionThreshold float64 = 0.25

// Summary is a pasteable aggregate of routing events for issues and reports.
// It carries no prompt text, only counts and cost proxies. JSON field order
// follows struct order so ExportJSON output is stable across runs.
type Summary struct {
	WindowDays       int            `json:"window_days"`
	GeneratedAt      string         `json:"generated_at"`
	Total            int            `json:"total"`
	Downshifted      int            `json:"downshifted"`
	Upshifted        int            `json:"upshifted"`
	OK               int            `json:"ok"`
	Unknown          int            `json:"unknown"`
	Corrected        int            `json:"corrected"`
	NotApplied       int            `json:"not_applied"`
	ActualCostEvents int            `json:"actual_cost_events"`
	ActualCostUSD    float64        `json:"actual_cost_usd"`
	RealCostEvents   int            `json:"real_cost_events"`
	RealSavedUSD     float64        `json:"real_saved_usd"`
	UsageRecords     int            `json:"usage_records"`
	UsageLinked      int            `json:"usage_linked"`
	Baseline         int            `json:"baseline_no_route"`
	RewriteShifted   int            `json:"rewrite_shifted"`
	RewriteHonored   int            `json:"rewrite_honored_inferred"`
	NormBaseline     float64        `json:"norm_baseline"`
	NormRouted       float64        `json:"norm_routed"`
	ByComplexity     map[string]int `json:"by_complexity"`
	DownshiftRate    float64        `json:"downshift_rate"`
	Note             string         `json:"note"`
}

// ExportSummary aggregates events into a Summary without I/O or clock reads.
// The caller passes now explicitly so tests can pin time; days <= 0 selects
// all events (including ones with unparsable timestamps, mirroring
// FilterByDays). With days > 0, events with unparsable timestamps are
// excluded from every window (fail-closed for windowing).
func ExportSummary(events []Event, days int, now time.Time) Summary {
	filtered := filterByDaysAt(events, days, now)
	stats := Aggregate(filtered)

	downshiftRate := 0.0
	unknownRate := 0.0
	if stats.Total > 0 {
		downshiftRate = float64(stats.Downshifted) / float64(stats.Total)
		unknownRate = float64(stats.Unknown) / float64(stats.Total)
	}

	note := baseNote
	if unknownRate > unknownCautionThreshold {
		note = fmt.Sprintf("%s Caution: unknown_rate=%.1f%% exceeds 25%%; legacy-schema and fail-open error events inflate unknown, so treat the downshift rate as provisional.", baseNote, unknownRate*100)
	}

	byComplexity := make(map[string]int, len(stats.ByComplexity))
	for key, count := range stats.ByComplexity {
		byComplexity[key] = count
	}

	return Summary{
		WindowDays:       days,
		GeneratedAt:      now.UTC().Format(time.RFC3339Nano),
		Total:            stats.Total,
		Downshifted:      stats.Downshifted,
		Upshifted:        stats.Upshifted,
		OK:               stats.OK,
		Unknown:          stats.Unknown,
		Corrected:        stats.Corrected,
		NotApplied:       stats.NotApplied,
		ActualCostEvents: stats.ActualCostEvents,
		ActualCostUSD:    stats.ActualCostUSD,
		RealCostEvents:   stats.RealCostEvents,
		RealSavedUSD:     stats.RealSavedUSD,
		UsageRecords:     stats.UsageRecords,
		UsageLinked:      stats.UsageLinked,
		Baseline:         stats.Baseline,
		RewriteShifted:   stats.RewriteShifted,
		RewriteHonored:   stats.RewriteHonored,
		NormBaseline:     stats.NormBaseline,
		NormRouted:       stats.NormRouted,
		ByComplexity:     byComplexity,
		DownshiftRate:    downshiftRate,
		Note:             note,
	}
}

// ExportJSON renders summary as stable indented JSON that contributors can
// paste into issues. Maps serialize with sorted keys, so output is
// deterministic for the same input.
func ExportJSON(summary Summary) (string, error) {
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// filterByDaysAt is the clock-injected twin of FilterByDays. It exists so
// ExportSummary stays a pure function of (events, days, now) for tests.
func filterByDaysAt(events []Event, days int, now time.Time) []Event {
	if days <= 0 {
		return events
	}
	cutoff := now.UTC().Add(-time.Duration(days) * 24 * time.Hour)
	var out []Event
	for _, ev := range events {
		parsed, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339, ev.Timestamp)
		}
		if err != nil {
			continue
		}
		if parsed.After(cutoff) {
			out = append(out, ev)
		}
	}
	return out
}
