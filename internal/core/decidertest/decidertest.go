// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package decidertest holds fixed, deterministic core.Decider stubs for
// contract tests. A contract test checks the wire (JSON shape, sibling
// fields, fail-open, multi-child, honor), not the routing policy, so it pins
// the decision with a stub instead of relying on what the v1 classifier
// happens to say about a prompt. Test code only: production never imports it.
package decidertest

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Keep returns a Decider that always keeps the requested model: a checked,
// safe OK verdict on the current model, so no adapter rewrites the spawn.
func Keep() core.Decider {
	return core.DeciderFunc(func(req core.Request) core.Decision {
		d := base(req)
		cur, known := lookup(req)
		if known {
			d.Tier, d.Effort, d.Model = cur.Tier, core.EffortFor(cur.Tier), cur
			d.Baseline.CurrentModel = cur
			preserveExplicit(&d, req, cur)
		}
		d.Verdict, d.SafeVerdict = core.VerdictOK, core.VerdictOK
		return d
	})
}

// Tier returns a Decider that always routes to tier t: the catalog model for
// the harness and tier, compared to the current model, with the verdict
// taken as safe. It runs no guardrail, so a contract test sees a rewrite
// whenever the session allows one.
func Tier(t core.Tier) core.Decider {
	return core.DeciderFunc(func(req core.Request) core.Decision {
		d := base(req)
		d.Tier, d.Effort, d.Complexity = t, core.EffortFor(t), complexityOf(t)
		d.Model = core.Model{Tier: t, Harness: req.Harness}
		if req.Resolver != nil {
			d.Model = req.Resolver.ModelFor(req.Harness, t)
		}
		d.Verdict = core.VerdictUnknown
		if cur, known := lookup(req); known {
			d.Baseline.CurrentModel = cur
			preserveExplicit(&d, req, cur)
			switch {
			case cur.Tier == t:
				d.Verdict = core.VerdictOK
			case cur.Tier > t:
				d.Verdict = core.VerdictDownshift
			default:
				d.Verdict = core.VerdictUpshift
			}
		}
		d.SafeVerdict = d.Verdict
		return d
	})
}

// Use installs d as the active Decider for the rest of the test and restores
// the previous one on cleanup. Tests that call it must not run in parallel.
func Use(t testing.TB, d core.Decider) {
	t.Helper()
	t.Cleanup(core.SetDecider(d))
}

func base(req core.Request) core.Decision {
	return core.Decision{
		Complexity: core.Medium,
		Harness:    req.Harness,
		Confident:  true,
		Checked:    true,
	}
}

// preserveExplicit marks the decision with the preserved intent when the
// current model is explicit_only: the owner's deliberate choice, which routing
// never replaces.
func preserveExplicit(d *core.Decision, req core.Request, cur core.Model) {
	if req.Resolver.IsExplicitOnly(req.Harness, cur.ID) {
		d.Intent = core.PreservedIntent
	}
}

func lookup(req core.Request) (core.Model, bool) {
	if req.Resolver == nil || req.CurrentModelID == "" {
		return core.Model{}, false
	}
	return req.Resolver.LookupByID(req.Harness, req.CurrentModelID)
}

// complexityOf is the label a tier-t decision reports; it only feeds readouts.
func complexityOf(t core.Tier) core.Complexity {
	switch t {
	case core.TierSmall:
		return core.Simple
	case core.TierFrontier:
		return core.Complex
	default:
		return core.Medium
	}
}

// Stub is one named fixed Decider for a table-driven contract test. Keeps is
// true when the stub leaves the requested model in place; otherwise it routes
// to Tier.
type Stub struct {
	Name    string
	Decider core.Decider
	Keeps   bool
	Tier    core.Tier
}

// Stubs is the pair every table-driven contract test runs under: "keep"
// (no rewrite) and "small" (always the small tier).
func Stubs() []Stub {
	return []Stub{
		{Name: "keep", Decider: Keep(), Keeps: true},
		{Name: "small", Decider: Tier(core.TierSmall), Tier: core.TierSmall},
	}
}

// Each runs f once per stub in Stubs, as a subtest with that stub installed.
func Each(t *testing.T, f func(t *testing.T, s Stub)) {
	t.Helper()
	for _, s := range Stubs() {
		t.Run(s.Name, func(t *testing.T) {
			Use(t, s.Decider)
			f(t, s)
		})
	}
}
