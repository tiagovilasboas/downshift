// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/tiagovilasboas/downshift/internal/semantic"
)

// ClassifyWithSemantic runs the regex classifier plus optional MiniLM boost.
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
	label := base.Complexity.String()
	newLabel, ok := semantic.MaybeAugment(prompt, label, base.Confident)
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
