// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// rowTier is the tier a default profile row asks for, and whether it asks for
// one at all. Standard mode keeps a row that keeps the requested model, unless
// the row has a ClassTier and the classifier vouches for a routine, riskless
// task. Economy mode lowers it one step (see economyTier).
func rowTier(d policy.DefaultProfile, risky, routine bool, requested contract.Tier, economy bool) (contract.Tier, bool) {
	if economy {
		return economyTier(d, risky, requested)
	}
	if d.Keeps() {
		return d.TierForClass(routine, risky)
	}
	return d.TierFor(risky), true
}

// economyTier is the economy step of a default row. A row with a tier starts
// from that tier; a keeping row starts from the requested tier. The step is one
// tier down. The result never goes below the floor: mid on risk, small
// otherwise. It never rises above the requested tier, and an unknown requested
// tier asks for nothing. The guard still decides whether the move happens.
func economyTier(d policy.DefaultProfile, risky bool, requested contract.Tier) (contract.Tier, bool) {
	if !requested.Valid() {
		return contract.TierUnknown, false
	}
	base := requested
	if !d.Keeps() {
		base = d.TierFor(risky)
	}
	floor := contract.TierSmall
	if risky {
		floor = contract.TierMid
	}
	tiers := contract.Tiers()
	i := max(rank(base)-1, rank(floor))
	t := tiers[i]
	if rank(t) > rank(requested) {
		return contract.TierUnknown, false
	}
	return t, true
}
