// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core

import (
	"os"

	"github.com/tiagovilasboas/harness-downshift/internal/nbtier"
)

// NBDownshiftEnv turns the naive-Bayes TRIVIAL downshift on. Default OFF:
// the opinion is still computed and recorded (shadow mode) but routing is
// unchanged.
const NBDownshiftEnv = "DOWNSHIFT_NB_DOWNSHIFT"

// NBDownshift is the TRIVIAL downshift opinion for prompts where no
// classifier keyword fired. Would is true only when the classifier fell back
// to its Medium default, graphify did not escalate, the risk floor would not
// hold the task at Trivial, and NB labels it TRIVIAL with a margin of at
// least nbtier.DownshiftMargin. Applied is true only when the flag is on.
type NBDownshift struct {
	Would   bool
	Applied bool
	Margin  float64
}

// NBDownshiftEnabled reports whether DOWNSHIFT_NB_DOWNSHIFT=1.
func NBDownshiftEnabled() bool { return os.Getenv(NBDownshiftEnv) == "1" }

func nbTrivialOpinion(prompt string, cls Classification, graphEscalated bool) NBDownshift {
	if !cls.NoSignal || cls.Complexity != Medium || graphEscalated || cls.RiskFloor {
		return NBDownshift{}
	}
	// Never below the risk floor: a task the floor would lift is left alone.
	if applyRiskFloor(prompt, Classification{Complexity: Trivial}).RiskFloor {
		return NBDownshift{}
	}
	ok, margin := nbtier.Trivial(prompt)
	if !ok {
		return NBDownshift{Margin: margin}
	}
	return NBDownshift{Would: true, Applied: NBDownshiftEnabled(), Margin: margin}
}
