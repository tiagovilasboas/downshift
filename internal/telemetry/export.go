// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
)

// baseNote is the fixed honesty disclaimer attached to every export.
// Single-user dogfood data is directional, never a billing claim.
const baseNote string = "Single-user dogfood: directional only, not billing. Compare your provider dashboard before/after deploying downshift."

// unknownCautionThreshold is the unknown verdict share above which the
// downshift rate is provisional (legacy-schema and fail-open error events
// inflate unknown without telling us about routing quality).
const unknownCautionThreshold float64 = 0.25

// Summary is a pasteable aggregate of routing events for issues and reports.
// It carries no prompt text, only counts and token quantities. JSON field order
// follows struct order so ExportJSON output is stable across runs.
type Summary struct {
	SchemaVersion       int                       `json:"schema_version"`
	WindowDays          int                       `json:"window_days"`
	GeneratedAt         string                    `json:"generated_at"`
	Total               int                       `json:"total"`
	Downshifted         int                       `json:"downshifted"`
	Upshifted           int                       `json:"upshifted"`
	OK                  int                       `json:"ok"`
	Unknown             int                       `json:"unknown"`
	Corrected           int                       `json:"corrected"`
	NotApplied          int                       `json:"not_applied"`
	PolicyFloor         int                       `json:"policy_floor"`
	UsageRecords        int                       `json:"usage_records"`
	UsageLinked         int                       `json:"usage_linked"`
	Baseline            int                       `json:"baseline_no_route"`
	RewriteShifted      int                       `json:"rewrite_shifted"`
	RewriteHonored      int                       `json:"rewrite_honored_inferred"`
	ByComplexity        map[string]int            `json:"by_complexity"`
	DownshiftRate       float64                   `json:"downshift_rate"`
	ObservedChildTokens int64                     `json:"observed_child_tokens"`
	ObservedChildUsage  int                       `json:"observed_child_usage"`
	ChildStatus         map[string]int            `json:"child_status"`
	ReasonCodes         map[string]int            `json:"reason_codes,omitempty"`
	AgentTypes          map[string]AgentTypeCount `json:"agent_types,omitempty"`
	ByEntry             map[string]int            `json:"by_entry,omitempty"`
	DuplicateToolUse    int                       `json:"duplicate_tool_use,omitempty"`
	DCISummary          *codeintel.DCISummary     `json:"dci,omitempty"`
	TokensRerouted      TokensRerouted            `json:"tokens_rerouted"`
	Note                string                    `json:"note"`
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

	childStatus := make(map[string]int, len(stats.ChildStatus))
	for st, count := range stats.ChildStatus {
		if count > 0 {
			childStatus[string(st)] = count
		}
	}

	return Summary{
		SchemaVersion:       3,
		WindowDays:          days,
		GeneratedAt:         now.UTC().Format(time.RFC3339Nano),
		Total:               stats.Total,
		Downshifted:         stats.Downshifted,
		Upshifted:           stats.Upshifted,
		OK:                  stats.OK,
		Unknown:             stats.Unknown,
		Corrected:           stats.Corrected,
		NotApplied:          stats.NotApplied,
		PolicyFloor:         stats.PolicyFloor,
		UsageRecords:        stats.UsageRecords,
		UsageLinked:         stats.UsageLinked,
		Baseline:            stats.Baseline,
		RewriteShifted:      stats.RewriteShifted,
		RewriteHonored:      stats.RewriteHonored,
		ByComplexity:        byComplexity,
		DownshiftRate:       downshiftRate,
		ObservedChildTokens: stats.ObservedChildTokens,
		ObservedChildUsage:  stats.ObservedChildUsage,
		ChildStatus:         childStatus,
		ReasonCodes:         stats.ReasonCodes,
		AgentTypes:          nonEmptyAgentTypes(stats.ByAgentType),
		ByEntry:             nonEmptyCounts(stats.ByEntry),
		DuplicateToolUse:    stats.DuplicateToolUse,
		DCISummary:          exportDCISummary(stats.DCI),
		TokensRerouted:      stats.TokensRerouted,
		Note:                note,
	}
}

func exportDCISummary(dci DCIStats) *codeintel.DCISummary {
	if dci.PromptsEvaluated <= 0 {
		return nil
	}
	rate := 0.0
	if dci.PromptsEvaluated > 0 {
		rate = float64(dci.InjectionsApplied) / float64(dci.PromptsEvaluated)
	}
	return &codeintel.DCISummary{
		TotalPrompts:         dci.PromptsEvaluated,
		InjectionsApplied:    dci.InjectionsApplied,
		InjectionRate:        rate,
		InjectionPct:         rate * 100.0,
		Abstentions:          dci.PromptsEvaluated - dci.InjectionsApplied,
		AbstentionsByReason:  nonEmptyCounts(dci.AbstentionsByReason),
		TotalContextBytes:    dci.ContextBytesEmitted,
		AvgBytesPerInjection: dci.AvgContextBytes,
		LatencyP50Ms:         dci.LatencyP50Ms,
		LatencyP95Ms:         dci.LatencyP95Ms,
	}
}

// nonEmptyCounts copies a count map, dropping zero values so omitempty stays
// honest on the export.
func nonEmptyCounts(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		if v > 0 {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
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

// nonEmptyAgentTypes returns nil for no data so the field is omitted from the
// export, keeping its shape for logs that predate the type label.
func nonEmptyAgentTypes(m map[string]AgentTypeCount) map[string]AgentTypeCount {
	if len(m) == 0 {
		return nil
	}
	return m
}
