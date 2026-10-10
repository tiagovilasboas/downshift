// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

// Tier matches core.Tier ordinals without importing core (avoids an import cycle).
type Tier int

const (
	TierSmall Tier = iota
	TierMid
	TierFrontier
)

func tierToLabel(t Tier) tierLabel {
	switch t {
	case TierSmall:
		return "SMALL"
	case TierMid:
		return "MID"
	default:
		return "FRONTIER"
	}
}

func labelToTier(l tierLabel) (Tier, bool) {
	switch l {
	case "SMALL":
		return TierSmall, true
	case "MID":
		return TierMid, true
	case "FRONTIER":
		return TierFrontier, true
	default:
		return 0, false
	}
}

func clampTier(t Tier) Tier {
	if t < TierSmall {
		return TierSmall
	}
	if t > TierFrontier {
		return TierFrontier
	}
	return t
}
