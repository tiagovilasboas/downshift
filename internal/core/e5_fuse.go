// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/semantic"
	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
)

// E5 fuse actions, recorded prompt-free in telemetry as e5_fuse.action.
const (
	E5ActionNone    = "none"    // helper answered, classification left as it was
	E5ActionAbstain = "abstain" // reserved centroid (ABSTAIN/FOREIGN) cleared confidence
	E5ActionRaise   = "raise"   // nearest centroid raised the complexity
)

// E5Fuse is the prompt-free trace of the opt-in multilingual fuse
// (DOWNSHIFT_E5_FUSE=1). Enabled is false, and the rest empty, when the flag
// is off; then telemetry writes no e5_fuse object. On a fail-open ErrorCode
// names the closed code (missing_cmd, untrusted_cmd, load_prototypes,
// embed_error) and Action is empty. Never carries the prompt, the vector, the
// helper's stderr or an env path.
type E5Fuse struct {
	Enabled      bool
	Action       string
	ModelID      string
	NearestLabel string
	Similarity   float64
	LatencyMs    float64
	ErrorCode    string
}

// applyE5Fuse is the Phase E production path: opt-in via DOWNSHIFT_E5_FUSE=1.
// Helper is DOWNSHIFT_E5_CMD only (no silent DOWNSHIFT_E5_SHADOW_CMD reuse).
// ok is true when the helper returned a vector so the external MiniLM helper
// can be skipped (the in-process hash floor still runs, see fuseFloor).
// Fail-open: timeout, missing command, or bad prototypes → ok false, cls
// unchanged except for the trace.
func applyE5Fuse(prompt string, cls Classification) (out Classification, ok bool) {
	out = cls
	if !e5shadow.IntegrateEnabled() {
		return cls, false
	}
	out.E5Fuse = E5Fuse{Enabled: true}
	start := time.Now()
	defer func() {
		if recover() != nil {
			out = cls
			out.E5Fuse = E5Fuse{Enabled: true, ErrorCode: "embed_error"}
			ok = false
		}
		out.E5Fuse.LatencyMs = float64(time.Since(start).Milliseconds())
	}()
	look := e5shadow.LookupFuse(prompt)
	if look == nil {
		out.E5Fuse.ErrorCode = "embed_error"
		return out, false
	}
	out.E5Fuse.ModelID = look.ModelID
	if look.ErrorCode != "" {
		out.E5Fuse.ErrorCode = look.ErrorCode
		return out, false
	}
	out.E5Fuse.NearestLabel = look.NearestLabel
	out.E5Fuse.Similarity = look.Similarity
	out.E5Fuse.Action = E5ActionNone
	if cls.NoSignal {
		return out, true
	}

	riskAbstain := cls.RiskAbstain

	if e5shadow.LanguageAbstain(look.NearestLabel) && look.Similarity >= semantic.MinSimilarity {
		out.Confident = false
		out.E5Fuse.Action = E5ActionAbstain
	} else {
		newLabel, lifted := semantic.AugmentVector(look.Vec, cls.Complexity.String(), cls.Confident, look.Store)
		if lifted {
			c, conv := complexityFromLabel(newLabel)
			if conv && !(c == Complex && cls.Scores[Complex] == 0) {
				out.Complexity = c
				out.SemanticLift = out.SemanticLift || c > cls.Complexity
				out.Confident = false
				out.E5Fuse.Action = E5ActionRaise
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

// fuseFloor keeps the fuse-off classification as a floor under a fused one.
// With the fuse on, the external MiniLM helper is skipped (one helper per
// hook), but the in-process hash boost that fuse-off would have applied is
// free; without this floor a prompt that the hash boost lifts could end one
// class lower with the fuse on than off, the pattern the enable gate forbids.
// When the floor wins, the lift belongs to the hash boost, not to E5.
func fuseFloor(prompt string, base, fused Classification) Classification {
	off := applyBoost(prompt, base, semantic.MaybeAugmentHash)
	if off.Complexity <= fused.Complexity {
		return fused
	}
	out := fused
	out.Complexity = off.Complexity
	out.Confident = false
	out.SemanticLift = true
	if out.E5Fuse.Action == E5ActionRaise {
		out.E5Fuse.Action = E5ActionNone
	}
	return applyRiskFloor(prompt, out)
}
