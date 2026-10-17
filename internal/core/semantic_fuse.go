// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/tiagovilasboas/downshift/internal/semantic"
)

// ClassifyWithSemantic runs the regex classifier plus optional MiniLM boost
// and, when DOWNSHIFT_E5_FUSE=1, the multilingual E5 fuse.
func ClassifyWithSemantic(prompt string) Classification {
	return classifyWithSemantic(prompt, Classify(prompt))
}

func classifyWithSemantic(prompt string, base Classification) Classification {
	// The boost only raises. A prompt with no regex content signal is Medium by
	// default, not by evidence, so a nearest-centroid match on noise (a stray
	// word such as "hook") must not lift it to Complex and spend frontier money.
	if base.NoSignal {
		return base
	}
	fused, used := applyE5Fuse(prompt, base)
	if used {
		return fuseFloor(prompt, base, fused)
	}
	// Fail-open keeps the fuse trace (error code) on the MiniLM result.
	out := applyBoost(prompt, base, semantic.MaybeAugment)
	out.E5Fuse = fused.E5Fuse
	return out
}

// augmentFunc is the nearest-centroid step: (prompt, label, confident) to
// (new label, fired). semantic.MaybeAugment uses the configured embedder,
// semantic.MaybeAugmentHash only the in-process hash one.
type augmentFunc func(prompt, label string, confident bool) (string, bool)

func applyBoost(prompt string, base Classification, augment augmentFunc) Classification {
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
	// from the regex rules (risk floor and graph escalation add their own).
	if c == Complex && base.Scores[Complex] == 0 {
		return base
	}
	out := base
	out.Complexity = c
	out.Confident = false
	out.SemanticLift = c > base.Complexity
	// The semantic boost is monotonic and only raises complexity; the risk floor still holds.
	return applyRiskFloor(prompt, out)
}

func complexityFromLabel(label string) (Complexity, bool) {
	switch label {
	case "TRIVIAL":
		return Trivial, true
	case "SIMPLE":
		return Simple, true
	case "MEDIUM":
		return Medium, true
	case "COMPLEX":
		return Complex, true
	default:
		return Medium, false
	}
}
