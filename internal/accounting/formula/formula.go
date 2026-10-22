// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package formula implements the T0-specified Saved formula:
//
//	C_actual   = Σ price(model_i, tokens_i)           # actual models and tokens
//	C_baseline = price(requested_model, Σ tokens_i)  # all tokens at session model
//	Saved      = C_baseline − C_actual
//
// The formula measures routing saving only: the cost avoided by running some
// work on a cheaper model than requested. It does not include Compactor saving
// (bytes → tokens, additive) or quota/overage saving.
//
// Known limitation (D4): When a child runs on a cheaper model, it often
// generates more tokens. The formula prices those extra tokens at the baseline
// rate, causing a systematic positive bias of ~15% on runs with actual
// downshifts. Every published Saved figure must carry this caveat.
package formula

import (
	"github.com/tiagovilasboas/downshift/internal/catalog"
)

// TokenBucket represents one unit of token usage from a model.
type TokenBucket struct {
	Model            string // model id or slug
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

// TotalTokens returns the sum of all token types in this bucket.
func (b TokenBucket) TotalTokens() int64 {
	return b.InputTokens + b.OutputTokens + b.CacheReadTokens + b.CacheWriteTokens
}

// SavedResult contains the formula output with quality metadata.
type SavedResult struct {
	// ActualCost is C_actual: sum of price(model_i, tokens_i) for all buckets.
	ActualCost float64 `json:"actual_cost_usd"`

	// BaselineCost is C_baseline: price(requested_model, Σ tokens_i).
	BaselineCost float64 `json:"baseline_cost_usd"`

	// Saved is C_baseline − C_actual. May be negative if some buckets ran
	// on a more expensive model than requested.
	Saved float64 `json:"saved_usd"`

	// TotalTokens is the sum of all tokens across all buckets.
	TotalTokens int64 `json:"total_tokens"`

	// Quality summarizes the pricing confidence across all buckets.
	Quality SummaryQuality `json:"quality"`

	// UnpricedBuckets is the count of buckets that could not be priced.
	UnpricedBuckets int `json:"unpriced_buckets,omitempty"`

	// InferredBuckets is the count of buckets priced via slug inference.
	InferredBuckets int `json:"inferred_buckets,omitempty"`

	// BaselineQuality is the quality of the requested model's price lookup.
	BaselineQuality catalog.Quality `json:"baseline_quality"`
}

// SummaryQuality summarizes pricing confidence for the whole result.
type SummaryQuality string

const (
	// SummaryExact means all buckets (and baseline) were priced exactly.
	SummaryExact SummaryQuality = "exact"

	// SummaryMixed means some buckets were inferred or upper_bound.
	SummaryMixed SummaryQuality = "mixed"

	// SummaryPartial means some buckets were unpriced and excluded.
	SummaryPartial SummaryQuality = "partial"

	// SummaryUnpriced means the baseline model is unpriced; result is invalid.
	SummaryUnpriced SummaryQuality = "unpriced"
)

// Saved computes the routing saving for a set of token buckets.
//
// The formula:
//
//	C_actual   = Σ price(model_i, tokens_i)
//	C_baseline = price(requested_model, Σ tokens_i)
//	Saved      = C_baseline − C_actual
//
// Unpriced buckets are excluded from C_actual but their tokens still count
// toward TotalTokens and are priced in C_baseline. This is conservative:
// it overstates savings when unpriced models are cheaper, understates when
// they are more expensive.
func Saved(requestedModel string, buckets []TokenBucket, pb *catalog.PriceBook) SavedResult {
	if pb == nil {
		return SavedResult{Quality: SummaryUnpriced}
	}

	// Get baseline pricing.
	baselinePrice := pb.PriceFor(requestedModel)
	if !baselinePrice.IsPriced() {
		return SavedResult{
			Quality:         SummaryUnpriced,
			BaselineQuality: baselinePrice.Quality,
		}
	}

	// Aggregate tokens and compute actual cost.
	var (
		totalInput, totalOutput, totalCacheRead, totalCacheWrite int64
		actualCost                                               float64
		unpricedCount, inferredCount                             int
		allExact                                                 = true
	)

	for _, b := range buckets {
		totalInput += b.InputTokens
		totalOutput += b.OutputTokens
		totalCacheRead += b.CacheReadTokens
		totalCacheWrite += b.CacheWriteTokens

		price := pb.PriceFor(b.Model)
		if !price.IsPriced() {
			unpricedCount++
			allExact = false
			// Unpriced: tokens still count in baseline, but actual cost is unknown.
			// Conservative: assume actual cost = baseline cost for this bucket.
			actualCost += baselinePrice.ComputeCost(
				b.InputTokens, b.OutputTokens, b.CacheReadTokens, b.CacheWriteTokens,
			)
		} else {
			actualCost += price.ComputeCost(
				b.InputTokens, b.OutputTokens, b.CacheReadTokens, b.CacheWriteTokens,
			)
			if price.Quality == catalog.QualityInferred {
				inferredCount++
				allExact = false
			} else if price.Quality != catalog.QualityExact {
				allExact = false
			}
		}
	}

	// Compute baseline cost: all tokens at requested model.
	baselineCost := baselinePrice.ComputeCost(
		totalInput, totalOutput, totalCacheRead, totalCacheWrite,
	)

	// Determine summary quality.
	var quality SummaryQuality
	switch {
	case unpricedCount > 0:
		quality = SummaryPartial
	case !allExact:
		quality = SummaryMixed
	default:
		quality = SummaryExact
	}

	return SavedResult{
		ActualCost:      actualCost,
		BaselineCost:    baselineCost,
		Saved:           baselineCost - actualCost,
		TotalTokens:     totalInput + totalOutput + totalCacheRead + totalCacheWrite,
		Quality:         quality,
		UnpricedBuckets: unpricedCount,
		InferredBuckets: inferredCount,
		BaselineQuality: baselinePrice.Quality,
	}
}

// SavedPositive returns true if the formula computed a positive saving.
// This is a convenience predicate for reporting.
func (r SavedResult) SavedPositive() bool {
	return r.Saved > 0
}

// IsValid returns true if the result can be used for reporting.
// Invalid results have an unpriced baseline.
func (r SavedResult) IsValid() bool {
	return r.Quality != SummaryUnpriced
}

// SavedPercent returns the saving as a percentage of baseline cost.
// Returns 0 if baseline is zero or invalid.
func (r SavedResult) SavedPercent() float64 {
	if r.BaselineCost <= 0 || !r.IsValid() {
		return 0
	}
	return (r.Saved / r.BaselineCost) * 100
}
