// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// Catalog tier ranks, lowest first. A ModelCatalog reports tiers on this scale.
const (
	TierRankSmall = iota
	TierRankMid
	TierRankFrontier
)

// TierFrontier is the event-log name of the frontier tier (Event.Tier).
const TierFrontier = "frontier"

// ModelCatalog is the view of the model catalog telemetry needs: the catalog
// id and tier rank of a model, whether an id is an exact catalog id, and the
// tier rank of an allowlisted gateway row (shadow or live). It uses plain
// values only, so the routing catalog is adapted to it at the edge
// (catalog.TierView, wired by fromcore) and telemetry never imports the router.
type ModelCatalog interface {
	Lookup(harness, id string) (catalogID string, tier int, ok bool)
	IsExactID(harness, id string) bool
	GatewayTier(id string) (tier int, ok bool)
}

// DirectionCatalog ranks the requested and the ran model by catalog tier
// (small < mid < frontier), and Record writes model ids as its catalog ids
// (canonicalizeModels). The composition root sets it at startup; with nil no
// record can be placed, so every priced record is unattributed, and model ids
// are recorded as the harness sent them.
var DirectionCatalog ModelCatalog

// directionVs is the one lower-tier computation: it ranks the model that ran
// against a baseline model by catalog tier, native or gateway. The baseline is
// the session model or, as a labelled fallback, the requested model (RoutingTokens); a
// model the catalog does not know is unattributed, never a guess.
func directionVs(harness, baseline, ran string) string {
	if IsGatewayModel(ran) {
		return gatewayDirection(harness, baseline, ran)
	}
	return tierDirection(harness, baseline, ran)
}

// tierDirection ranks a ran model against a requested one by catalog tier.
// A model the catalog does not know is unattributed, never a guess.
func tierDirection(harness, requested, ran string) string {
	if DirectionCatalog == nil {
		return TokenUnattributed
	}
	_, req, okReq := DirectionCatalog.Lookup(harness, requested)
	_, got, okRan := DirectionCatalog.Lookup(harness, ran)
	if !okReq || !okRan {
		return TokenUnattributed
	}
	return rankTiers(req, got)
}

// gatewayDirection ranks a run that went through a gateway by the tier of its
// allowlisted row, under the same rule as a native run. An id no allowlisted
// row names is unattributed, never a guess.
func gatewayDirection(harness, requested, ran string) string {
	if DirectionCatalog == nil {
		return TokenUnattributed
	}
	_, req, ok := DirectionCatalog.Lookup(harness, requested)
	if !ok {
		return TokenUnattributed
	}
	tier, ok := DirectionCatalog.GatewayTier(ran)
	if !ok {
		return TokenUnattributed
	}
	return rankTiers(req, tier)
}

// rankTiers is the one ranking rule for native and gateway runs: a ran tier
// below the baseline one is cheaper, above it pricier, equal is unchanged.
func rankTiers(requested, ran int) string {
	switch {
	case ran < requested:
		return TokenCheaper
	case ran > requested:
		return TokenPricier
	}
	return TokenUnchanged
}
