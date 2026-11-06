// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// The model of a usage record, and the tier directions the one comparator
// (directionVs) returns. The routing figure itself is RoutingTokens.

// Where a priced usage record's tokens land.
const (
	// TokenCheaper: an applied rewrite whose model cost less than the requested one.
	TokenCheaper = "cheaper"
	// TokenPricier: an applied rewrite whose model cost more (an upshift).
	TokenPricier = "pricier"
	// TokenUnchanged: the child ran on the requested model, or at the same
	// price: an allow, a held or blocked decision, a paused spawn, a
	// rewrite the harness did not honor, or an applied rewrite priced the same.
	TokenUnchanged = "unchanged"
	// TokenUnattributed: the record cannot be placed: no linked decision in
	// this window, or an applied rewrite whose requested model has no price.
	TokenUnattributed = "unattributed"
)

func knownModel(id string) bool { return id != "" && id != "unknown" }

// tokenModels returns the requested and ran model of a usage record. The record
// is the observation, so it wins; the linked decision fills a missing id. A
// rewrite the harness did not honor ran on the requested model.
func tokenModels(usage, decision Event, found, refuted bool) (requested, ran string) {
	requested, ran = usage.FromModel, usage.ToModel
	if !knownModel(requested) && found {
		requested = decision.FromModel
	}
	if refuted {
		return requested, requested
	}
	if !knownModel(ran) && found {
		if AppliedRewrite(decision.Outcome, decision.Corrections) {
			ran = decision.ToModel
		} else {
			ran = decision.FromModel
		}
	}
	return requested, ran
}
