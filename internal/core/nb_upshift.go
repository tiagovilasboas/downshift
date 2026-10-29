// Copyright (c) 2026 Tiago Vilas Boas.
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
// No package in this binary registers it (internal/nbopinion was removed), so
// a nil hook is no opinion.
var NBSuggestHook func(prompt string, prodTier int) (upshift bool, to int, margin float64)

// NBUpshiftEnabled reports whether DOWNSHIFT_NB_UPSHIFT=1.
func NBUpshiftEnabled() bool { return nbUpshiftEnv() }
