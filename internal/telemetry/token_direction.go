// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "github.com/tiagovilasboas/downshift/internal/core"

// tokenDirectionByRank places one priced usage record without comparing
// dollars. It applies the same gates as tokenDirection, then ranks the model
// that ran against the model requested by catalog tier (small < mid <
// frontier). Same tier reads unchanged, whatever the prices say.
//
// Parallel to tokenDirection while part 4a is in review; it is not wired into
// the aggregate yet (tokens-only part 4b).
func tokenDirectionByRank(usage, decision Event, found, unrouted, refuted bool, cat core.Resolver) string {
	switch {
	case unrouted:
		return TokenUnchanged
	case !found:
		return TokenUnattributed
	case !AppliedRewrite(decision.Outcome, decision.Corrections), refuted:
		return TokenUnchanged
	case !usage.HasRealCost():
		return TokenUnattributed
	}
	requested, ran := tokenModels(usage, decision, found, refuted)
	req, okReq := cat.LookupByID(usage.Harness, requested)
	got, okRan := cat.LookupByID(usage.Harness, ran)
	if !okReq || !okRan {
		return TokenUnattributed
	}
	switch {
	case got.Tier < req.Tier:
		return TokenCheaper
	case got.Tier > req.Tier:
		return TokenPricier
	}
	return TokenUnchanged
}
