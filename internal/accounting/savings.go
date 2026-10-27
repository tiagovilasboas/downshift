// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// UnifiedSavings is the combined token-savings view for a period: routing
// tokens saved from the requested model plus compactor tokens estimated from
// bytes dropped. It is a directional dogfood figure, not a billing claim.
//
// The accounting-layer ADR named a "SavingsEstimate" for this role, but that
// type name is already taken in types.go by the routed-vs-control
// confidence-interval estimate used by ComparisonReport. This struct keeps the
// ADR's intent under a distinct name so both can coexist in one package.
type UnifiedSavings struct {
	// Period is the human-readable window, e.g. "7d" or "all time".
	Period string `json:"period"`
	// RoutingTokens is work+cache tokens that ran on a cheaper model than the
	// one the harness requested (telemetry TokenSavings).
	RoutingTokens int64 `json:"routing_tokens_saved"`
	// CompactorTokens is the estimated tokens saved by shell-output
	// compaction (execwrap saved_bytes / 4). Labelled as an estimate.
	CompactorTokens int64 `json:"compactor_tokens_saved"`
	// TotalTokens is RoutingTokens + CompactorTokens.
	TotalTokens int64 `json:"total_tokens_saved"`
}

// loadRoutingEvents is the routing-telemetry load seam. Tests override it to
// inject events without touching the real event log.
var loadRoutingEvents = telemetry.ReadEvents

// loadCompactorSummary is the compactor-metrics load seam. Tests override it
// to inject execwrap totals without touching the real exec log. The returned
// Commands == 0 means "no wrapped commands", not an error.
var loadCompactorSummary = func() (execwrap.Summary, error) {
	path, err := execwrap.EventsPath()
	if err != nil {
		return execwrap.Summary{}, err
	}
	return execwrap.Summarize(path)
}

// CalculateSavings computes combined routing + compactor token savings for the
// last days (0 = all time). Compactor totals are not windowed: the exec log
// carries no per-record age filter here, so they are the running total and are
// labelled accordingly by callers that need the distinction.
func CalculateSavings(days int) (UnifiedSavings, error) {
	events, err := loadRoutingEvents()
	if err != nil {
		return UnifiedSavings{}, err
	}
	filtered := telemetry.FilterByDays(events, days)
	saved := telemetry.TokenSavings(filtered).Total
	routingTokens := saved.WorkSaved + saved.CacheSaved

	var compactorTokens int64
	if s, cErr := loadCompactorSummary(); cErr == nil {
		compactorTokens = s.EstimatedTokensSaved
	}

	return UnifiedSavings{
		Period:          periodLabel(days),
		RoutingTokens:   routingTokens,
		CompactorTokens: compactorTokens,
		TotalTokens:     telemetry.TokensSavedEstimate(telemetry.TokenSaving{WorkSaved: routingTokens}, compactorTokens),
	}, nil
}

// FormatSavings renders a one-line, human-readable savings summary. The
// compactor figure is marked an estimate because it comes from a bytes/4 proxy.
func FormatSavings(s UnifiedSavings) string {
	return fmt.Sprintf(
		"%s: saved ~%s tokens (routing %s | compactor ~%s est.)",
		s.Period,
		formatTokenCount(s.TotalTokens),
		formatTokenCount(s.RoutingTokens),
		formatTokenCount(s.CompactorTokens),
	)
}

// periodLabel renders the window for display and for the Period field.
func periodLabel(days int) string {
	if days <= 0 {
		return "all time"
	}
	return fmt.Sprintf("%dd", days)
}

// formatTokenCount renders a token count compactly: 1234 -> "1.2k",
// 1500000 -> "1.5M". Small counts print as-is.
func formatTokenCount(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
