// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package routeadapt wires local tier memory into core.Route without import cycles.
package routeadapt

import (
	"github.com/tiagovilasboas/downshift/internal/adapt"
	"github.com/tiagovilasboas/downshift/internal/core"
)

func init() {
	core.MemoryAdjustHook = Apply
}

// Apply records the tier adapt-memory.json would select. It does not change
// d.Tier, d.Model, or d.Verdict. Promotion out of shadow needs an independent
// outcome suite and a measured cost per completed task, which this repo does
// not have yet.
func Apply(prompt string, d *core.Decision, res core.Resolver) {
	mem, err := adapt.LoadDefault()
	if err != nil || mem == nil {
		return
	}
	key := adapt.MemoryKeyForPrompt(prompt, "")
	if key == "" {
		return
	}
	newTier := adapt.AdjustTier(adapt.Tier(d.Tier), key, mem)
	if core.Tier(newTier) == d.Tier {
		return
	}
	d.AdaptWould = core.Tier(newTier)
	d.AdaptWouldSet = true
}
