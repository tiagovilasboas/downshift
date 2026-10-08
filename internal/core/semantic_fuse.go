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
	label := base.Complexity.String()
	newLabel, ok := semantic.MaybeAugment(prompt, label, base.Confident)
	if !ok {
		return base
	}
	c, ok := complexityFromLabel(newLabel)
	if !ok {
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
