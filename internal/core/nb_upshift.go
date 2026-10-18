// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// NBUpshift is the upshift-only second opinion from internal/nbtier.
// Would is true when NB rates the task a higher tier than the classifier
// with a margin of at least nbtier.Margin; Applied is true only when the
// flag is on and the tier was actually raised. It never lowers a tier.
type NBUpshift struct {
	Would   bool
	Applied bool
	From    Tier // tier before the second opinion
	To      Tier // tier NB would route to (== From when it has no upshift)
	Margin  float64
}

// NBSuggestHook is that opinion: upshift, destination tier, and margin.
// internal/nbopinion registers it in init. A nil hook is no opinion.
var NBSuggestHook func(prompt string, prodTier int) (upshift bool, to int, margin float64)

// NBUpshiftEnabled reports whether DOWNSHIFT_NB_UPSHIFT=1.
func NBUpshiftEnabled() bool { return nbUpshiftEnv() }

func nbSecondOpinion(prompt string, tier Tier) NBUpshift {
	out := NBUpshift{From: tier, To: tier}
	if NBSuggestHook == nil {
		return out
	}
	upshift, to, margin := NBSuggestHook(prompt, int(tier))
	out.Margin = margin
	if upshift && Tier(to) > tier {
		out.Would = true
		out.To = Tier(to)
		out.Applied = NBUpshiftEnabled()
	}
	return out
}

func complexityForTier(t Tier) Complexity {
	switch t {
	case TierFrontier:
		return Complex
	case TierMid:
		return Medium
	default:
		return Simple
	}
}
