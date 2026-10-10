// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package routeadapt wires local tier memory into core.Route without import cycles.
package routeadapt

import (
	"github.com/tiagovilasboas/downshift/internal/adapt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routingv2/domain"
	"github.com/tiagovilasboas/downshift/internal/routingv2/extractor"
)

func init() {
	core.MemoryAdjustHook = Apply
}

// Apply adjusts d when adapt-memory.json exists and policy selects another tier.
func Apply(prompt string, d *core.Decision, res core.Resolver) {
	mem, err := adapt.LoadDefault()
	if err != nil || mem == nil {
		return
	}
	fv := extractor.Extract(prompt)
	shape := domain.DominantFeature(fv)
	newTier := adapt.AdjustTier(adapt.Tier(d.Tier), shape, mem)
	if core.Tier(newTier) == d.Tier {
		return
	}
	currentID := d.CurrentModel.ID
	d.Tier = core.Tier(newTier)
	d.Effort = core.EffortFor(d.Tier)
	d.Model = resolveModel(d.Harness, d.Tier, res)
	verdict, savings, current := compareToCurrent(d.Harness, currentID, d.Model, res)
	d.Verdict = verdict
	d.Savings = savings
	if current.ID != "" {
		d.CurrentModel = current
	}
	report := core.Check(*d)
	d.SafeVerdict = report.Safe
	d.Checked = true
	d.Corrections = nil
	for _, v := range report.Violations {
		d.Corrections = append(d.Corrections, v.Rule)
	}
}

func resolveModel(harness string, tier core.Tier, r core.Resolver) core.Model {
	if r != nil {
		return r.ModelFor(harness, tier)
	}
	return core.Model{Tier: tier, Harness: harness}
}

func compareToCurrent(harness, currentModelID string, recommended core.Model, r core.Resolver) (core.Verdict, float64, core.Model) {
	if r == nil || currentModelID == "" {
		return core.VerdictUnknown, 0, core.Model{}
	}
	current, known := r.LookupByID(harness, currentModelID)
	if !known {
		return core.VerdictUnknown, 0, core.Model{}
	}
	switch {
	case current.Tier == recommended.Tier:
		return core.VerdictOK, 0, current
	case current.Tier > recommended.Tier:
		return core.VerdictDownshift, r.SavingsRatio(current, recommended), current
	default:
		return core.VerdictUpshift, 0, current
	}
}
