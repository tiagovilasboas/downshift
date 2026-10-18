// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package nbopinion registers the naive-Bayes second opinion with core.
// core keeps the guards and the Decision fields; this package owns the
// nbtier calls so core does not import nbtier. The downshift binary
// blank-imports this package.
package nbopinion

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/nbtier"
)

func init() {
	core.NBSuggestHook = suggest
	core.NBTrivialHook = trivial
}

func suggest(prompt string, prodTier int) (bool, int, float64) {
	s := nbtier.Suggest(prompt, prodTier)
	return s.Upshift, s.To, s.Margin
}

func trivial(prompt string) (bool, float64) {
	return nbtier.Trivial(prompt)
}
