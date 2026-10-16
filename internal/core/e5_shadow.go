// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/tiagovilasboas/downshift/internal/semantic/e5shadow"
)

// E5Shadow records a parallel multilingual embedding observation. It does not
// change Tier or Complexity; telemetry only unless a future integration flag
// promotes the candidate.
type E5Shadow struct {
	ModelID        string
	NearestLabel   string
	Similarity     float64
	WouldLabel     string
	WouldLift      bool
	RegexLabel     string
	RegexConfident bool
	LatencyMs      float64
	ErrorCode      string
}

func applyE5Shadow(prompt string, cls Classification, d *Decision) {
	if !e5shadow.Enabled() {
		return
	}
	obs := e5shadow.Observe(prompt, cls.Complexity.String(), cls.Confident)
	if obs == nil {
		return
	}
	d.E5Shadow = E5Shadow{
		ModelID:        obs.ModelID,
		NearestLabel:   obs.NearestLabel,
		Similarity:     obs.Similarity,
		WouldLabel:     obs.WouldLabel,
		WouldLift:      obs.WouldLift,
		RegexLabel:     obs.RegexLabel,
		RegexConfident: obs.RegexConfident,
		LatencyMs:      obs.LatencyMs,
		ErrorCode:      obs.ErrorCode,
	}
}
