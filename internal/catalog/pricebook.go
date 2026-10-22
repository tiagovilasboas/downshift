// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"strings"
)

// Quality indicates the confidence level of a price lookup.
type Quality string

const (
	// QualityExact means all four price fields (input, output, cache_read,
	// cache_write) are present in the catalog.
	QualityExact Quality = "exact"

	// QualityUpperBound means partial data; cache prices assumed zero or max.
	QualityUpperBound Quality = "upper_bound"

	// QualityInferred means a slug was mapped to its family member.
	QualityInferred Quality = "inferred"

	// QualityUnpriced means no resolvable id; excluded from Saved calculations.
	QualityUnpriced Quality = "unpriced"
)

// PriceResult contains the cost rates and quality for a priced model.
type PriceResult struct {
	ModelID string  `json:"model_id"`
	Quality Quality `json:"quality"`

	// Costs per 1M tokens.
	InputCostM      float64 `json:"input_cost_per_1m"`
	OutputCostM     float64 `json:"output_cost_per_1m"`
	CacheReadCostM  float64 `json:"cache_read_cost_per_1m"`
	CacheWriteCostM float64 `json:"cache_write_cost_per_1m"`

	// InferredFrom is set when Quality is QualityInferred, naming the slug
	// that was resolved to ModelID.
	InferredFrom string `json:"inferred_from,omitempty"`
}

// PriceBook wraps a Catalog with the T0-specified PriceFor interface.
// It provides model lookups with explicit quality indicators.
type PriceBook struct {
	cat *Catalog
}

// NewPriceBook creates a PriceBook from the given Catalog.
func NewPriceBook(c *Catalog) *PriceBook {
	return &PriceBook{cat: c}
}

// Version returns the catalog version string.
// Note: The current Catalog implementation doesn't expose version, so this returns empty.
func (pb *PriceBook) Version() string {
	// The Catalog struct doesn't expose the version from catalogFile.
	// This is a known limitation; version tracking would need to be added upstream.
	return ""
}

// AsOf returns the prices_as_of date (YYYY-MM-DD) from the catalog.
func (pb *PriceBook) AsOf() string {
	if pb.cat == nil {
		return ""
	}
	return pb.cat.PricesAsOf()
}

// PriceFor looks up pricing for a model id or slug.
// It returns the price result with an explicit Quality indicator.
//
// Resolution order:
//  1. Exact match on model id → QualityExact (if all prices present)
//  2. Family slug inference (haiku → claude-haiku-5-5) → QualityInferred
//  3. Unknown/empty → QualityUnpriced
func (pb *PriceBook) PriceFor(modelID string) PriceResult {
	if pb.cat == nil || modelID == "" {
		return PriceResult{ModelID: modelID, Quality: QualityUnpriced}
	}

	// 1. Try exact match first (iterate all entries).
	for _, e := range pb.cat.entries {
		if e.ID == modelID || contains(e.Aliases, modelID) {
			return pb.entryToResult(e.ID, e, "", pb.qualityForEntry(e))
		}
	}

	// 2. Try slug inference for common patterns.
	if inferred := pb.inferFromSlug(modelID); inferred != "" {
		for _, e := range pb.cat.entries {
			if e.ID == inferred || contains(e.Aliases, inferred) {
				return pb.entryToResult(e.ID, e, modelID, QualityInferred)
			}
		}
	}

	// 3. Unpriced.
	return PriceResult{ModelID: modelID, Quality: QualityUnpriced}
}

// contains checks if slice contains s.
func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// inferFromSlug maps common gateway slugs to their canonical model ids.
// This handles KiroCrew gateway slugs like "haiku", "sonnet", "opus".
func (pb *PriceBook) inferFromSlug(slug string) string {
	lower := strings.ToLower(slug)
	switch {
	case lower == "haiku":
		return "claude-haiku-5-5"
	case lower == "sonnet":
		return "claude-sonnet-5-5"
	case lower == "opus":
		return "claude-opus-5-5"
	case lower == "unknown":
		return "" // Explicitly unpriced
	default:
		return ""
	}
}

// qualityForEntry determines quality based on which price fields are present.
func (pb *PriceBook) qualityForEntry(e Entry) Quality {
	// All four fields present = exact.
	if e.InputCostM > 0 && e.OutputCostM > 0 {
		// Cache fields are optional; if both present, exact. Otherwise upper_bound.
		if e.CacheReadCostM > 0 && e.CacheWriteCostM > 0 {
			return QualityExact
		}
		return QualityUpperBound
	}
	return QualityUnpriced
}

// entryToResult converts a catalog Entry to a PriceResult.
func (pb *PriceBook) entryToResult(modelID string, e Entry, inferredFrom string, q Quality) PriceResult {
	return PriceResult{
		ModelID:         modelID,
		Quality:         q,
		InputCostM:      e.InputCostM,
		OutputCostM:     e.OutputCostM,
		CacheReadCostM:  e.CacheReadCostM,
		CacheWriteCostM: e.CacheWriteCostM,
		InferredFrom:    inferredFrom,
	}
}

// ComputeCost calculates the cost in USD for given token counts using this price.
func (pr PriceResult) ComputeCost(input, output, cacheRead, cacheWrite int64) float64 {
	const perM = 1_000_000.0
	return float64(input)*pr.InputCostM/perM +
		float64(output)*pr.OutputCostM/perM +
		float64(cacheRead)*pr.CacheReadCostM/perM +
		float64(cacheWrite)*pr.CacheWriteCostM/perM
}

// IsExact returns true if this price has full confidence.
func (pr PriceResult) IsExact() bool {
	return pr.Quality == QualityExact
}

// IsPriced returns true if this price can be used in Saved calculations.
func (pr PriceResult) IsPriced() bool {
	return pr.Quality != QualityUnpriced
}
