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
// (internal/telemetry/catalogtier) and telemetry never imports the router.
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

// tokenDirection places one priced usage record without comparing dollars. The
// gates come first; then the model that ran is ranked against the model that
// was requested, by tier. Same tier reads unchanged, whatever the prices say.
// A verdict OK child (allow, no correction) is ranked too, with or without an
// applied rewrite (owner decision C3): it saved only when it ran on a cheaper
// tier. Held, blocked and refuted decisions never rank.
func tokenDirection(usage, decision Event, found, unrouted, refuted bool) string {
	switch {
	case unrouted:
		return TokenUnchanged
	case !found:
		return TokenUnattributed
	case refuted:
		return TokenUnchanged
	case !AppliedRewrite(decision.Outcome, decision.Corrections) && !verdictOK(decision):
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

// verdictOK reports a decision the guardrails let through unchanged: an allow
// with no correction. Such a child may still have run on a cheaper model.
func verdictOK(decision Event) bool {
	return decision.Outcome == OutcomeAllow && len(decision.Corrections) == 0
}

// savedWithoutRewrite reports whether a child's saving came with no applied
// rewrite, so the saving is shown apart (SavedWithoutRewrite).
func savedWithoutRewrite(decision Event, found bool) bool {
	return found && !AppliedRewrite(decision.Outcome, decision.Corrections)
}

// atTierFloor reports whether the requested model already sits on the lowest
// catalog tier, so no cheaper model exists to move the child to. An unknown
// model or no catalog is false, never a guess.
func atTierFloor(harness, requested string) bool {
	if DirectionCatalog == nil {
		return false
	}
	_, tier, ok := DirectionCatalog.Lookup(harness, requested)
	return ok && tier == TierRankSmall
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

// gatewayRowKnown reports whether an allowlisted gateway row names ran.
func gatewayRowKnown(ran string) bool {
	if DirectionCatalog == nil {
		return false
	}
	_, ok := DirectionCatalog.GatewayTier(ran)
	return ok
}

// rankTiers is the one ranking rule for native and gateway runs: a ran tier
// below the requested one is saved, above it pricier, equal is unchanged.
func rankTiers(requested, ran int) string {
	switch {
	case ran < requested:
		return TokenCheaper
	case ran > requested:
		return TokenPricier
	}
	return TokenUnchanged
}
