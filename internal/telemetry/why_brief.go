// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "strings"

// briefWords is each shaping reason code in a few words, for one-line "why"
// output (`downshift try`). Guardrail codes use ReasonWords instead.
var briefWords = map[string]string{
	ReasonNoSignal:             "no clear signal",
	ReasonSemanticBoost:        "raised by meaning-based matching",
	ReasonGraphEscalation:      "wide change in the dependency graph",
	ReasonNBUpshiftApplied:     "raised by the second-opinion model",
	ReasonNBDownshiftApplied:   "lowered by the second-opinion model",
	ReasonTierFloor:            "project tier floor",
	ReasonScoreRouted:          "picked by public capability scores",
	ReasonBaselineFromSettings: "baseline inferred from the configured model",
	ReasonCreditHeld:           "credit gate held the rewrite",
	ReasonPaused:               "routing paused",
}

// ReasonBrief renders reason codes as one short phrase such as
// "no risk, confident": risk first, then confidence, then any lift or hold.
// It reads only codes, never prompt text.
func ReasonBrief(codes []string) string {
	risk, confident, unconfident := false, false, false
	var extra, held []string
	for _, c := range codes {
		switch {
		case c == ReasonRiskCategory || c == ReasonRiskFloor:
			risk = true
		case c == ReasonClassifierConfident:
			confident = true
		case c == ReasonClassifierUnconfident:
			unconfident = true
		case isGuardrail(c):
			if w, ok := briefWords[c]; ok {
				held = append(held, w)
			} else {
				held = append(held, ReasonWords(c))
			}
		default:
			if w, ok := briefWords[c]; ok {
				extra = append(extra, w)
			}
		}
	}
	var parts []string
	if risk {
		parts = append(parts, "touches a risk area")
	} else {
		parts = append(parts, "no risk")
	}
	switch {
	case unconfident:
		parts = append(parts, "unsure")
	case confident:
		parts = append(parts, "confident")
	}
	parts = append(parts, extra...)
	parts = append(parts, held...)
	return strings.Join(parts, ", ")
}
