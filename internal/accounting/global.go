// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"sort"
	"time"

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

// DailySummary is one UTC day's routing and compactor metrics.
type DailySummary struct {
	Date      string           `json:"date"`
	Sessions  int              `json:"sessions"`
	Routing   RoutingMetrics   `json:"routing"`
	Compactor CompactorMetrics `json:"compactor"`
}

// BuildDailyTrend returns a per-UTC-day breakdown over the last days days
// (0 = all time), oldest first. Days with no activity on either source are
// omitted.
func BuildDailyTrend(days int) ([]DailySummary, error) {
	events, err := loadRoutingEvents()
	if err != nil {
		return nil, err
	}
	routingByDay := bucketRoutingByDay(telemetry.FilterByDays(events, days))

	compactorByDay := map[string]CompactorMetrics{}
	if recs, cErr := loadCompactorEvents(); cErr == nil {
		compactorByDay = bucketCompactorByDay(recs, days)
	}

	dates := unionDays(routingByDay, compactorByDay)
	sort.Strings(dates)

	trend := make([]DailySummary, 0, len(dates))
	for _, date := range dates {
		day := DailySummary{Date: date}
		if r, ok := routingByDay[date]; ok {
			day.Routing = r.metrics
			day.Sessions = len(r.sessions)
		}
		if c, ok := compactorByDay[date]; ok {
			day.Compactor = c
		}
		trend = append(trend, day)
	}
	return trend, nil
}

// dayRouting holds one day's routing aggregate plus the distinct session ids
// seen, so the per-day session count is accurate.
type dayRouting struct {
	metrics  RoutingMetrics
	sessions map[string]struct{}
}

// bucketRoutingByDay groups routing events by UTC date and folds each bucket
// through BuildGlobalSummary so the per-day metrics match the global roll-up.
func bucketRoutingByDay(events []telemetry.Event) map[string]dayRouting {
	grouped := map[string][]telemetry.Event{}
	sessions := map[string]map[string]struct{}{}
	for _, ev := range events {
		date, ok := utcDate(ev.Timestamp)
		if !ok {
			continue
		}
		grouped[date] = append(grouped[date], ev)
		if ev.SessionID != "" {
			if sessions[date] == nil {
				sessions[date] = map[string]struct{}{}
			}
			sessions[date][ev.SessionID] = struct{}{}
		}
	}

	out := make(map[string]dayRouting, len(grouped))
	for date, evs := range grouped {
		summary := BuildGlobalSummary(evs, SummaryOptions{Days: 0})
		out[date] = dayRouting{metrics: summary.Routing, sessions: sessions[date]}
	}
	return out
}

// loadCompactorEvents is the exec-log load seam for the daily trend. Tests
// override it to inject records without touching the real exec log.
var loadCompactorEvents = func() ([]execwrap.Event, error) {
	path, err := execwrap.EventsPath()
	if err != nil {
		return nil, err
	}
	return execwrap.ReadEvents(path)
}

// bucketCompactorByDay groups exec records by UTC date within the window and
// totals each day's bytes. days <= 0 keeps every record.
func bucketCompactorByDay(recs []execwrap.Event, days int) map[string]CompactorMetrics {
	var cutoff time.Time
	if days > 0 {
		cutoff = time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	}
	out := map[string]CompactorMetrics{}
	for _, ev := range recs {
		parsed, err := time.Parse(time.RFC3339, ev.TS)
		if err != nil {
			continue
		}
		if days > 0 && !parsed.After(cutoff) {
			continue
		}
		date := parsed.UTC().Format("2006-01-02")
		m := out[date]
		m.Commands++
		if ev.Compacted {
			m.Compacted++
		}
		m.RawBytes += ev.RawBytes
		if ev.RawBytes > ev.EmittedBytes {
			m.SavedBytes += ev.RawBytes - ev.EmittedBytes
		}
		m.EstimatedTokensSaved = m.SavedBytes / execwrap.BytesPerToken
		m.TokenEstimate = "estimated: saved_bytes / 4"
		out[date] = m
	}
	return out
}

// utcDate parses an RFC3339/RFC3339Nano timestamp and returns its UTC date as
// YYYY-MM-DD. ok is false when the timestamp is unparsable.
func utcDate(ts string) (string, bool) {
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, ts)
	}
	if err != nil {
		return "", false
	}
	return parsed.UTC().Format("2006-01-02"), true
}

// unionDays returns the distinct dates present in either bucket.
func unionDays(a map[string]dayRouting, b map[string]CompactorMetrics) []string {
	seen := map[string]struct{}{}
	for d := range a {
		seen[d] = struct{}{}
	}
	for d := range b {
		seen[d] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	return out
}
