// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"github.com/tiagovilasboas/downshift/internal/core"
)

// TokenUsage carries provider-reported token counts for one routed spawn.
// All fields are optional at the Event level (pointers + omitempty) so old
// JSONL lines without usage data keep parsing. When absent, cost output
// falls back to normalised estimate units — never to fake dollars.
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
	// CachedTokens is cache-creation plus cache-read input (the harness totals).
	CachedTokens int64
	// CacheWriteTokens is the cache-creation part of CachedTokens, billed at
	// the model's cache-write price. Zero means the split is unknown and the
	// whole CachedTokens is billed at the cache-read price.
	CacheWriteTokens int64
}

// CostUSD computes the provider cost for one spawn from catalog list prices
// (USD per 1M tokens). Cached tokens are billed at the model's cache-read
// price when the catalog has one. When it has none (CacheReadM == 0) they are
// billed at the input rate, which is an upper bound: most providers discount
// cached reads. Cache writes (CacheWriteTokens) are billed at CacheWriteM when
// the catalog has one, else at the cached-read rate. No cache price is ever
// invented.
func CostUSD(model core.Model, usage TokenUsage) float64 {
	cachedRate := model.CacheReadM
	if cachedRate <= 0 {
		cachedRate = model.InputM
	}
	writeRate := model.CacheWriteM
	if writeRate <= 0 {
		writeRate = cachedRate
	}
	cachedTotal := max(usage.CachedTokens, 0)
	writes := min(max(usage.CacheWriteTokens, 0), cachedTotal)
	reads := cachedTotal - writes
	input := float64(max(usage.InputTokens, 0)) * model.InputM / 1_000_000.0
	cached := float64(reads) * cachedRate / 1_000_000.0
	cacheWrite := float64(writes) * writeRate / 1_000_000.0
	output := float64(max(usage.OutputTokens, 0)) * model.OutputM / 1_000_000.0
	return input + cached + cacheWrite + output
}

// RealSavingsUSD returns baseline minus routed cost for the same usage.
// Returns 0 when the routed model is not cheaper (upshift / OK).
func RealSavingsUSD(baseline, routed core.Model, usage TokenUsage) float64 {
	base := CostUSD(baseline, usage)
	routedCost := CostUSD(routed, usage)
	if routedCost >= base {
		return 0
	}
	return base - routedCost
}
