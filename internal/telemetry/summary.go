// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// SummaryBasis labels every figure in a Summary: tokens as the harness reported
// them, sorted by the tier they ran on. No price enters.
const SummaryBasis = "tokens as reported by the harness, by the tier they ran on"

// MaxLineBytes is the longest event-log line the readers accept. A line
// above it is reported as an error instead of silently truncating the log.
const MaxLineBytes = 1 << 20

// SavingsSummary is the one definition of "tokens saved" shared by `downshift
// stats`, the dashboard /api/status and the dsmon widget. Every figure comes
// from usage records with token counts, placed by the tier comparison.
type SavingsSummary struct {
	// Basis says what the figures are; show it next to them.
	Basis string `json:"basis"`
	// UsageEvents counts usage records that carried token counts.
	UsageEvents int `json:"usage_events"`
	// GrossSavedTokens counts the tokens that ran on a cheaper tier than the
	// one requested (TokensRerouted, cheaper bucket).
	GrossSavedTokens int64 `json:"gross_saved_tokens"`
	// OverspendTokens counts the tokens that ran on a pricier tier.
	OverspendTokens int64 `json:"overspend_tokens"`
	// NetSavedTokens is GrossSavedTokens minus OverspendTokens. Negative means
	// the router moved more tokens up than down overall.
	NetSavedTokens int64 `json:"net_saved_tokens"`
}

// Summarize aggregates events into the single savings definition.
func Summarize(events []Event) SavingsSummary {
	return Aggregate(events).Summary()
}

// Summary projects the token fields of Stats into the shared definition.
func (s Stats) Summary() SavingsSummary {
	var saved, over, records int64
	if t := s.TokensRerouted.Totals; t != nil {
		saved, over = t.Cheaper.Total, t.Pricier.Total
		records = int64(t.Cheaper.Records + t.Pricier.Records + t.Unchanged.Records)
	}
	return SavingsSummary{
		Basis:            SummaryBasis,
		UsageEvents:      int(records),
		GrossSavedTokens: saved,
		OverspendTokens:  over,
		NetSavedTokens:   saved - over,
	}
}
