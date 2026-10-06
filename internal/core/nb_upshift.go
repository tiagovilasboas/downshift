// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"os"

	"github.com/tiagovilasboas/downshift/internal/nbtier"
)

// NBUpshiftEnv turns the naive-Bayes upshift on. Default OFF: the opinion is
// still computed and recorded (shadow mode) but the routing tier is unchanged.
const NBUpshiftEnv = "DOWNSHIFT_NB_UPSHIFT"

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

// NBUpshiftEnabled reports whether DOWNSHIFT_NB_UPSHIFT=1.
func NBUpshiftEnabled() bool { return os.Getenv(NBUpshiftEnv) == "1" }

func nbSecondOpinion(prompt string, tier Tier) NBUpshift {
	s := nbtier.Suggest(prompt, int(tier))
	out := NBUpshift{From: tier, To: tier, Margin: s.Margin}
	if s.Upshift && Tier(s.To) > tier {
		out.Would = true
		out.To = Tier(s.To)
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
