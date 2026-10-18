// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// NBDownshift is the TRIVIAL downshift opinion for prompts where no
// classifier keyword fired. Would is true only when the classifier fell back
// to its Medium default, the risk floor would not
// hold the task at Trivial, and NB labels it TRIVIAL with a margin of at
// least nbtier.DownshiftMargin. Applied is true only when the flag is on;
// even then guardrail R1 (unconfident downshift) holds the move, so the flag
// changes the recorded tier but never rewrites a spawn on its own.
type NBDownshift struct {
	Would   bool
	Applied bool
	Margin  float64
}

// NBTrivialHook reports whether the prompt is a TRIVIAL downshift candidate
// and the small-vs-rest margin. internal/nbopinion registers it in init.
// A nil hook is no opinion. The risk-floor and no-signal guards stay here.
var NBTrivialHook func(prompt string) (ok bool, margin float64)

// NBDownshiftEnabled reports whether DOWNSHIFT_NB_DOWNSHIFT=1.
func NBDownshiftEnabled() bool { return nbDownshiftEnv() }

func nbTrivialOpinion(prompt string, cls Classification) NBDownshift {
	if !cls.NoSignal || cls.Complexity != Medium || cls.RiskFloor {
		return NBDownshift{}
	}
	// Never below the risk floor: a task the floor would lift is left alone.
	if applyRiskFloor(prompt, Classification{Complexity: Trivial}).RiskFloor {
		return NBDownshift{}
	}
	if NBTrivialHook == nil {
		return NBDownshift{}
	}
	ok, margin := NBTrivialHook(prompt)
	if !ok {
		return NBDownshift{Margin: margin}
	}
	return NBDownshift{Would: true, Applied: NBDownshiftEnabled(), Margin: margin}
}
