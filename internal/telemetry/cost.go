// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// TokenUsage carries provider-reported token counts for one routed spawn.
// All fields are optional at the Event level (pointers + omitempty) so old
// JSONL lines without usage data keep parsing. When absent, cost output
// falls back to normalised estimate units — never to fake dollars.
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
	CachedTokens int64
}

// CostUSD computes the real provider cost for one spawn from catalog list
// prices (USD per 1M tokens). Cached tokens are billed at the input rate:
// most providers discount them, so this is a conservative upper bound and
// is documented as such wherever it is displayed.
func CostUSD(model core.Model, usage TokenUsage) float64 {
	input := float64(usage.InputTokens+usage.CachedTokens) * model.InputM / 1_000_000.0
	output := float64(usage.OutputTokens) * model.OutputM / 1_000_000.0
	if input < 0 {
		input = 0
	}
	if output < 0 {
		output = 0
	}
	return input + output
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
