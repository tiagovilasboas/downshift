// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The structure of this predictor (global Beta-posterior mean, shrunk toward
// a per-group posterior, probabilities made monotone over tiers, first tier
// above a threshold) is adapted from litellm/router_strategy/complexity_router/
// tier_predictor.py in BerriAI/litellm (commit 4b8a846), MIT License,
// Copyright (c) 2023 Berri AI. See NOTICE. The Go code, the two-level
// hierarchy (tier, shape), the prior masses and the threshold are Downshift's
// own; upstream constants are not reused.

package adapt

// tierCount holds successes and observations for one (group, tier).
type tierCount struct{ succ, obs float64 }

// CountPredictor estimates P(tier succeeds | shape) from counts, so one
// success and fifty successes no longer weigh the same (unlike the set-based
// Memory). It is shadow-only until evaluated on an independent suite.
type CountPredictor struct {
	global [3]tierCount
	shapes map[string][3]tierCount
	// PriorMass is how many pseudo-observations the global mean is worth
	// against a shape's own counts. Larger means slower to leave the global.
	PriorMass float64
	// Threshold is the success probability a tier must reach to be chosen.
	Threshold float64
}

// NewCountPredictor builds counts from feedback. Unknown outcomes are skipped.
func NewCountPredictor(events []FeedbackEvent, priorMass, threshold float64) *CountPredictor {
	p := &CountPredictor{shapes: map[string][3]tierCount{}, PriorMass: priorMass, Threshold: threshold}
	for _, e := range events {
		if e.Shape == "" || !(e.Success || e.Retry || e.Failed) {
			continue
		}
		t := clampTier(e.SelectedTier)
		s := p.shapes[e.Shape]
		p.global[t].obs++
		s[t].obs++
		if e.Success {
			p.global[t].succ++
			s[t].succ++
		}
		p.shapes[e.Shape] = s
	}
	return p
}

// Probabilities returns P(success) per tier for a shape, made monotone: a
// stronger tier is never rated below a weaker one.
func (p *CountPredictor) Probabilities(shape string) [3]float64 {
	var out [3]float64
	s := p.shapes[shape]
	best := 0.0
	for t := 0; t < 3; t++ {
		g := (p.global[t].succ + 1) / (p.global[t].obs + 2)
		prob := (s[t].succ + p.PriorMass*g) / (s[t].obs + p.PriorMass)
		if prob < best {
			prob = best
		}
		best = prob
		out[t] = prob
	}
	return out
}

// RequiredTier is the cheapest tier whose probability meets the threshold,
// or frontier when none does.
func (p *CountPredictor) RequiredTier(shape string) Tier {
	probs := p.Probabilities(shape)
	for t := TierSmall; t <= TierFrontier; t++ {
		if probs[t] >= p.Threshold {
			return t
		}
	}
	return TierFrontier
}
