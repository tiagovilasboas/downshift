// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package contract

import "github.com/tiagovilasboas/downshift/internal/core"

// Tier is the serialized form of core.Tier with an explicit unknown. The
// contract does not reuse core.Tier directly because its zero value is
// TierSmall: a zero Note would claim the small tier. The strings match
// core.Tier.String, so telemetry keeps one spelling ("frontier", not the
// "strong" of the design draft).
type Tier string

const (
	TierUnknown  Tier = ""
	TierSmall    Tier = "small"
	TierMid      Tier = "mid"
	TierFrontier Tier = "frontier"
)

// Tiers lists every known Tier from cheapest to strongest.
func Tiers() []Tier { return []Tier{TierSmall, TierMid, TierFrontier} }

// Valid reports whether t is a known tier.
func (t Tier) Valid() bool { return member(t, Tiers()) }

// TierOf converts a core tier. A value core does not define is TierUnknown.
func TierOf(t core.Tier) Tier {
	switch t {
	case core.TierSmall:
		return TierSmall
	case core.TierMid:
		return TierMid
	case core.TierFrontier:
		return TierFrontier
	default:
		return TierUnknown
	}
}

// Core converts back to core.Tier; ok is false for TierUnknown or any other
// value, so a caller cannot mistake unknown for small.
func (t Tier) Core() (tier core.Tier, ok bool) {
	switch t {
	case TierSmall:
		return core.TierSmall, true
	case TierMid:
		return core.TierMid, true
	case TierFrontier:
		return core.TierFrontier, true
	default:
		return 0, false
	}
}
