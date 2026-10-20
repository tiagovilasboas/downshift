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
	if IsGatewayModel(ran) {
		return gatewayDirection(usage.Harness, requested, ran)
	}
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
	return rankTiers(req.Tier, got.Tier)
}

// gatewayDirection ranks a run that went through a gateway by the tier of its
// allowlisted row, under the same rule as a native run. An id no allowlisted
// row names is unattributed, never a guess.
func gatewayDirection(harness, requested, ran string) string {
	if DirectionCatalog == nil {
		return TokenUnattributed
	}
	req, ok := DirectionCatalog.LookupByID(harness, requested)
	if !ok {
		return TokenUnattributed
	}
	tier, ok := gatewayTier(ran)
	if !ok {
		return TokenUnattributed
	}
	return rankTiers(req.Tier, tier)
}

// gatewayTier returns the tier of the allowlisted gateway row whose id is ran.
// The catalog exposes shadow rows and live rows through the optional
// core.GatewayShadowResolver and core.GatewayLiveResolver; a resolver with
// neither knows no gateway tier.
func gatewayTier(ran string) (core.Tier, bool) {
	for _, m := range allowlistRows() {
		if m.ID == ran {
			return m.Tier, true
		}
	}
	return 0, false
}

// allowlistRows lists every allowlisted gateway row the catalog exposes, shadow
// and live. A live row is the one a run goes through when the user made it live.
func allowlistRows() []core.Model {
	var rows []core.Model
	if gr, ok := DirectionCatalog.(core.GatewayShadowResolver); ok {
		rows = append(rows, gr.ShadowGateways()...)
	}
	if lr, ok := DirectionCatalog.(core.GatewayLiveResolver); ok {
		rows = append(rows, lr.LiveGateways()...)
	}
	return rows
}

// gatewayRowKnown reports whether an allowlisted gateway row names ran.
func gatewayRowKnown(ran string) bool {
	_, ok := gatewayTier(ran)
	return ok
}

// rankTiers is the one ranking rule for native and gateway runs: a ran tier
// below the requested one is saved, above it pricier, equal is unchanged.
func rankTiers(requested, ran core.Tier) string {
	switch {
	case ran < requested:
		return TokenCheaper
	case ran > requested:
		return TokenPricier
	}
	return TokenUnchanged
}
