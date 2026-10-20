// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// SummaryBasis labels every dollar figure in a Summary: the numbers are a
// catalog list-price counterfactual, not provider billing.
const SummaryBasis = "estimated, list price"

// MaxLineBytes is the longest event-log line the readers accept. A line
// above it is reported as an error instead of silently truncating the log.
const MaxLineBytes = 1 << 20

// SavingsSummary is the one definition of "money saved" shared by `downshift stats`,
// the dashboard /api/status and the dsmon widget. Every figure comes from
// events with real provider usage and is an estimate at catalog list price.
type SavingsSummary struct {
	// Basis says what the dollar figures are; show it next to them.
	Basis string `json:"basis"`
	// CostEvents counts events that carried both real cost fields.
	CostEvents int `json:"cost_events"`
	// InferredBaselineEvents counts CostEvents whose baseline was guessed
	// from settings rather than requested by the harness.
	InferredBaselineEvents int `json:"inferred_baseline_events"`
	// GrossSavedUSD sums only the events that cost less than their baseline.
	GrossSavedUSD float64 `json:"gross_saved_usd"`
	// OverspendUSD sums, as a positive number, the events that cost more.
	OverspendUSD float64 `json:"overspend_usd"`
	// NetSavedUSD is GrossSavedUSD minus OverspendUSD. Negative means the
	// router cost more than the baseline overall. This is the headline.
	NetSavedUSD float64 `json:"net_saved_usd"`
	// GrossSavedTokens counts the tokens that ran on a cheaper tier than the
	// one requested (TokensRerouted, cheaper bucket). Tokens only: no price.
	GrossSavedTokens int64 `json:"gross_saved_tokens"`
	// OverspendTokens counts the tokens that ran on a pricier tier.
	OverspendTokens int64 `json:"overspend_tokens"`
	// NetSavedTokens is GrossSavedTokens minus OverspendTokens.
	NetSavedTokens int64 `json:"net_saved_tokens"`
}

// Summarize aggregates events into the single savings definition.
func Summarize(events []Event) SavingsSummary {
	return Aggregate(events).Summary()
}

// Summary projects the money fields of Stats into the shared definition.
func (s Stats) Summary() SavingsSummary {
	var saved, over int64
	if t := s.TokensRerouted.Totals; t != nil {
		saved, over = t.Cheaper.Total, t.Pricier.Total
	}
	return SavingsSummary{
		GrossSavedTokens:       saved,
		OverspendTokens:        over,
		NetSavedTokens:         saved - over,
		Basis:                  SummaryBasis,
		CostEvents:             s.RealCostEvents,
		InferredBaselineEvents: s.RealCostInferred,
		GrossSavedUSD:          s.RealSavedUSD,
		OverspendUSD:           s.RealOverspendUSD,
		NetSavedUSD:            s.RealCostDeltaUSD,
	}
}
