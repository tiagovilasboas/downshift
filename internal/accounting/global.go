// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// SummaryOptions controls BuildGlobalSummary output.
type SummaryOptions struct {
	// Days is the time window; 0 means all time.
	Days int
	// IncludeDetail adds the complexity/status breakdown.
	IncludeDetail bool
}

// BuildGlobalSummary aggregates telemetry and compactor across all sessions.
func BuildGlobalSummary(events []telemetry.Event, opts SummaryOptions) GlobalSummary {
	filtered := telemetry.FilterByDays(events, opts.Days)
	stats := telemetry.Aggregate(filtered)
	saved := telemetry.TokenSavings(filtered)

	summary := GlobalSummary{
		Days: opts.Days,
		Routing: RoutingMetrics{
			Switched:      stats.Downshifted + stats.Upshifted,
			Held:          stats.Corrected,
			WorkTokens:    saved.Total.WorkTokens,
			WorkSaved:     saved.Total.WorkSaved,
			CacheTokens:   saved.Total.CacheTokens,
			CacheSaved:    saved.Total.CacheSaved,
			SavedChildren: saved.Total.SavedChildren,
			AtFloor:       saved.Total.AtFloor,
			Line:          saved.Total.Line(),
		},
	}

	// Add compactor totals
	if compactor := buildGlobalCompactor(); compactor != nil {
		summary.Compactor = *compactor
	}

	// Add detailed stats if requested
	if opts.IncludeDetail {
		summary.Stats = DetailedStats{
			Total:        stats.Total,
			Downshifted:  stats.Downshifted,
			Upshifted:    stats.Upshifted,
			NotApplied:   stats.NotApplied,
			OK:           stats.OK,
			Unknown:      stats.Unknown,
			ByComplexity: stats.ByComplexity,
			ByAgentType:  agentTypeCountsToInt(stats.ByAgentType),
			ChildStatus:  childStatusMap(stats.ChildStatus),
		}
	}

	return summary
}

// childStatusMap converts telemetry.ChildStatus keys to strings.
func childStatusMap(m map[telemetry.ChildStatus]int) map[string]int {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[string(k)] = v
	}
	return out
}

// agentTypeCountsToInt extracts decision counts from AgentTypeCount.
func agentTypeCountsToInt(m map[string]telemetry.AgentTypeCount) map[string]int {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v.Decisions
	}
	return out
}

// buildGlobalCompactor reads all execwrap events.
func buildGlobalCompactor() *CompactorMetrics {
	path, err := execwrap.EventsPath()
	if err != nil {
		return nil
	}

	s, err := execwrap.Summarize(path)
	if err != nil || s.Commands == 0 {
		return nil
	}

	return &CompactorMetrics{
		Commands:             s.Commands,
		Compacted:            s.Compacted,
		RawBytes:             s.RawBytes,
		SavedBytes:           s.SavedBytes,
		EstimatedTokensSaved: s.EstimatedTokensSaved,
		TokenEstimate:        s.TokenEstimate,
	}
}

// BuildComparisonReport builds the routed-vs-control analysis.
func BuildComparisonReport(events []telemetry.Event, days int) ComparisonReport {
	report := telemetry.BuildReport(events, days)

	result := ComparisonReport{
		Days:     report.Days,
		Excluded: report.Excluded,
		Enough:   report.Enough,
		Message:  report.Message,
		Routed: GroupSummary{
			Spawns:        report.Routed.Spawns,
			Ended:         report.Routed.Ended,
			Completed:     report.Routed.Completed,
			TokensPerTask: report.Routed.TokensPerTask,
			ReworkRate:    report.Routed.ReworkRate,
		},
		Control: GroupSummary{
			Spawns:        report.Control.Spawns,
			Ended:         report.Control.Ended,
			Completed:     report.Control.Completed,
			TokensPerTask: report.Control.TokensPerTask,
			ReworkRate:    report.Control.ReworkRate,
		},
	}

	if report.Savings != nil {
		result.Savings = &SavingsEstimate{
			PerTaskTokens:     report.Savings.PerTaskTokens,
			PerTaskTokensLow:  report.Savings.PerTaskTokensLow,
			PerTaskTokensHigh: report.Savings.PerTaskTokensHigh,
		}
	}

	return result
}
