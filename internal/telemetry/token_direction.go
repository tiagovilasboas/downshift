// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "github.com/tiagovilasboas/downshift/internal/core"

// DirectionCatalog ranks the requested and the ran model by catalog tier
// (small < mid < frontier). The composition root sets it at startup; with nil
// no record can be placed, so every priced record is unattributed.
var DirectionCatalog core.Resolver

// tokenDirection places one priced usage record without comparing dollars. The
// gates come first; then the model that ran is ranked against the model that
// was requested, by tier. Same tier reads unchanged, whatever the prices say.
func tokenDirection(usage, decision Event, found, unrouted, refuted bool) string {
	switch {
	case unrouted:
		return TokenUnchanged
	case !found:
		return TokenUnattributed
	case !AppliedRewrite(decision.Outcome, decision.Corrections), refuted:
		return TokenUnchanged
	case !usage.HasTokenUsage():
		return TokenUnattributed
	}
	requested, ran := tokenModels(usage, decision, found, refuted)
	return tierDirection(usage.Harness, requested, ran)
}

// atTierFloor reports whether the requested model already sits on the lowest
// catalog tier, so no cheaper model exists to move the child to. An unknown
// model or no catalog is false, never a guess.
func atTierFloor(harness, requested string) bool {
	if DirectionCatalog == nil {
		return false
	}
	m, ok := DirectionCatalog.LookupByID(harness, requested)
	return ok && m.Tier == core.TierSmall
}

// tierDirection ranks a ran model against a requested one by catalog tier.
// A model the catalog does not know is unattributed, never a guess.
func tierDirection(harness, requested, ran string) string {
	if DirectionCatalog == nil {
		return TokenUnattributed
	}
	req, okReq := DirectionCatalog.LookupByID(harness, requested)
	got, okRan := DirectionCatalog.LookupByID(harness, ran)
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
