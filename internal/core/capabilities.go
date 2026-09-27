// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core

// HarnessCapabilities describes what a harness's hook protocol can carry.
// Routing policy is shared; adapters only declare what their wire format
// supports. Add a new harness by adding a named var below — no adapter code
// needs to change.
type HarnessCapabilities struct {
	CanRewriteModel bool // hook output can replace the subagent model ID
	CanApplyEffort  bool // hook output can set reasoning effort level
}

// Per-harness capability table. Adapters import these instead of
// declaring HarnessCapabilities inline.
var (
	// ClaudeCodeCaps — PreToolUse hook honors updatedInput.model.
	ClaudeCodeCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false}

	// CursorCaps — preToolUse hook accepts updated_input.model.
	// Note: model rewrite is silently ignored on free/legacy plans.
	CursorCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false}

	// CodexCaps — PreToolUse hook accepts updatedInput.model + reasoning_effort.
	CodexCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: true}
)

// RewritePlan is the concrete set of protocol actions an adapter should take.
type RewritePlan struct {
	Model            Model
	RewriteModel     bool // write Model.ID into the hook output
	ApplyEffort      bool // write effort value into the hook output
	PreserveExplicit bool // current model is explicit_only; skip all rewrites
}

// Plan translates a harness-agnostic Decision into protocol actions for the
// given harness. It consults the Resolver to detect explicit_only models so
// the plan can signal that no rewrite should happen.
//
// When the classified downgrade cannot be applied (confidence gate, empty
// target, or a target that is not a catalog id for this harness), Plan
// routes to the smallest catalog model for the same harness. Adapters stay
// harness-agnostic: they already honor RewriteModel and Model.ID.
//
// Passing a nil Resolver disables explicit-only detection and the smallest-
// model fallback (used in tests that don't need a catalog).
func (d Decision) Plan(c HarnessCapabilities, r ...Resolver) RewritePlan {
	var res Resolver
	if len(r) > 0 && r[0] != nil {
		res = r[0]
	}
	currentID := d.CurrentModel.ID
	if res != nil && d.ShouldPreserveExplicitModel(currentID, res) {
		return RewritePlan{Model: d.Model, PreserveExplicit: true}
	}

	model := d.Model
	rewrite := c.CanRewriteModel && d.ShouldRewriteModel()
	if res != nil {
		if fb, ok := smallestCatalogFallback(d, res); ok && fb.ID != currentID {
			model = fb
			rewrite = c.CanRewriteModel
		} else if model.ID != "" && !canonicalCatalogID(res, d.Harness, model.ID) {
			// Never emit an alias or another harness's id.
			rewrite = false
			if canonicalCatalogID(res, d.Harness, currentID) {
				model = d.CurrentModel
			} else {
				model = Model{Tier: d.Tier, Harness: d.Harness}
			}
		}
	}
	if model.ID == "" || model.ID == currentID {
		rewrite = false
	}

	return RewritePlan{
		Model:        model,
		RewriteModel: rewrite,
		ApplyEffort:  c.CanApplyEffort && model.ID != "",
	}
}

// smallestCatalogFallback is the shared safety net for every harness.
// A blocked downgrade, an empty target, or a target that is not a real
// catalog id for this harness resolves to ModelFor(harness, TierSmall).
// The returned id is always that harness's catalog id, never an alias and
// never a model borrowed from another harness.
func smallestCatalogFallback(d Decision, res Resolver) (Model, bool) {
	if res == nil || d.Harness == "" {
		return Model{}, false
	}
	blockedDowngrade := d.Verdict == VerdictDownshift && !d.ShouldRewriteModel()
	emptyTarget := d.Model.ID == ""
	foreignTarget := d.Model.ID != "" && !canonicalCatalogID(res, d.Harness, d.Model.ID)
	if !blockedDowngrade && !emptyTarget && !foreignTarget {
		return Model{}, false
	}

	small := res.ModelFor(d.Harness, TierSmall)
	if small.ID == "" || res.IsExplicitOnly(d.Harness, small.ID) {
		return Model{}, false
	}
	if !canonicalCatalogID(res, d.Harness, small.ID) {
		return Model{}, false
	}
	return small, true
}

// canonicalCatalogID reports whether id is the catalog's own id for harness.
// Alias matches and family-prefix matches are not catalog ids: emitting them
// sends another harness's slug (for example claude-haiku-4 on Cursor).
func canonicalCatalogID(res Resolver, harness, id string) bool {
	if res == nil || id == "" {
		return false
	}
	looked, ok := res.LookupByID(harness, id)
	return ok && looked.ID == id
}
