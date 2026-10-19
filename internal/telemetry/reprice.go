// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "github.com/tiagovilasboas/downshift/internal/core"

// PriceFunc resolves a model id to its catalog entry for one harness.
type PriceFunc func(harness, modelID string) (core.Model, bool)

// RepriceUsage returns a copy of events in which every priced usage record that
// carries tokens has its dollar fields recomputed from those tokens at the
// prices price returns now: actual on the model that ran, baseline on the
// model the harness asked for. Dollars are a valuation of the measured tokens,
// done at report time, so a catalog correction reaches the whole history.
// Which records are priced stays what the recorder decided (an unknown
// baseline stays unknown, a deliberately unpriced child stays unpriced); only
// the numbers change. A model the catalog no longer prices loses its dollar
// field (unpriced, never $0). The log itself is never rewritten.
func RepriceUsage(events []Event, price PriceFunc) []Event {
	if price == nil {
		return events
	}
	out := make([]Event, len(events))
	copy(out, events)
	for i := range out {
		ev := &out[i]
		if ev.Outcome != OutcomeUsage {
			continue
		}
		usage := ev.TokenUsageOrZero()
		if ev.ActualCostUSD == nil || usage == (TokenUsage{}) || !knownModel(ev.ToModel) {
			continue
		}
		ran, ok := price(ev.Harness, ev.ToModel)
		if !ok || !HasModelPrices(ran) {
			ev.ActualCostUSD, ev.BaselineCostUSD = nil, nil
			continue
		}
		actual := CostUSD(ran, usage)
		ev.ActualCostUSD = &actual
		if ev.BaselineCostUSD == nil || !knownModel(ev.FromModel) {
			continue // no baseline was claimed at record time
		}
		if req, ok := price(ev.Harness, ev.FromModel); ok && HasModelPrices(req) {
			base := CostUSD(req, usage)
			ev.BaselineCostUSD = &base
		} else {
			ev.BaselineCostUSD = nil
		}
	}
	return out
}
