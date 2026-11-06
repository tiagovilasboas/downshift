// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package fuse applies the hash semantic boost on a regex classification.
// It may raise a hesitant class. It skips a no-signal prompt, does not create
// COMPLEX without a regex Complex score, and reapplies the risk floor after a lift.
package fuse

import (
	"github.com/tiagovilasboas/downshift/internal/classify"
	"github.com/tiagovilasboas/downshift/internal/semantic"
)

// Apply runs the regex classifier plus the in-process FNV hash semantic
// boost (DOWNSHIFT_SEMANTIC_BOOST).
func Apply(prompt string) classify.Classification {
	return apply(prompt, classify.Classify(prompt))
}

func apply(prompt string, base classify.Classification) classify.Classification {
	// The boost only raises. A prompt with no regex content signal is Medium by
	// default, not by evidence, so a nearest-centroid match on noise (a stray
	// word such as "hook") must not lift it to Complex and spend frontier money.
	if base.NoSignal {
		return base
	}
	return applyBoost(prompt, base, semantic.MaybeAugment)
}

// augmentFunc is the nearest-centroid step: (prompt, label, confident) to
// (new label, fired).
type augmentFunc func(prompt, label string, confident bool) (string, bool)

func applyBoost(prompt string, base classify.Classification, augment augmentFunc) classify.Classification {
	label := base.Complexity.String()
	newLabel, ok := augment(prompt, label, base.Confident)
	if !ok {
		return base
	}
	c, ok := complexityFromLabel(newLabel)
	if !ok {
		return base
	}
	// Frontier is the expensive, invisible error: the hash embedder's centroids
	// are nearly parallel, so its nearest class on a weak-signal prompt is close
	// to noise. The boost may lift toward Medium, but Complex needs evidence
	// from the regex rules (the risk floor adds its own lift).
	if c == classify.Complex && base.Scores[classify.Complex] == 0 {
		return base
	}
	out := base
	out.Complexity = c
	out.Confident = false
	out.SemanticLift = c > base.Complexity
	// The semantic boost is monotonic and only raises complexity; the risk floor still holds.
	return classify.ApplyRiskFloor(prompt, out)
}

func complexityFromLabel(label string) (classify.Complexity, bool) {
	switch label {
	case "TRIVIAL":
		return classify.Trivial, true
	case "SIMPLE":
		return classify.Simple, true
	case "MEDIUM":
		return classify.Medium, true
	case "COMPLEX":
		return classify.Complex, true
	default:
		return classify.Medium, false
	}
}
