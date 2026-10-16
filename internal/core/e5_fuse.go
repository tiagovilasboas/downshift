// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/tiagovilasboas/downshift/internal/semantic"
	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
)

// applyE5Fuse is the Phase E production path: opt-in via DOWNSHIFT_E5_FUSE=1.
// Helper is DOWNSHIFT_E5_CMD only (no silent DOWNSHIFT_E5_SHADOW_CMD reuse).
// ok is true when the helper returned a vector so MiniLM can be skipped.
// Fail-open: timeout, missing command, or bad prototypes → ok false, cls unchanged.
func applyE5Fuse(prompt string, cls Classification) (out Classification, ok bool) {
	out = cls
	if !e5shadow.IntegrateEnabled() {
		return cls, false
	}
	defer func() {
		if recover() != nil {
			out = cls
			ok = false
		}
	}()
	look := e5shadow.LookupFuse(prompt)
	if look == nil || look.ErrorCode != "" {
		return cls, false
	}
	if cls.NoSignal {
		return cls, true
	}

	riskAbstain := cls.RiskAbstain
	out = cls

	if e5shadow.LanguageAbstain(look.NearestLabel) && look.Similarity >= semantic.MinSimilarity {
		out.Confident = false
	} else {
		newLabel, lifted := semantic.AugmentVector(look.Vec, cls.Complexity.String(), cls.Confident, look.Store)
		if lifted {
			c, conv := complexityFromLabel(newLabel)
			if conv && !(c == Complex && cls.Scores[Complex] == 0) {
				out.Complexity = c
				out.SemanticLift = out.SemanticLift || c > cls.Complexity
				out.Confident = false
			}
		}
	}

	if cls.UnsupportedLanguage {
		out.Confident = false
	}
	if riskAbstain {
		out.RiskAbstain = true
		out.Confident = false
	}

	return applyRiskFloor(prompt, out), true
}
