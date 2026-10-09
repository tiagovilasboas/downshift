// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// inheritID is a session sentinel used by more than one harness. It means
// "the parent model", not a selectable target.
const inheritID = "inherit"

// CanWriteSessionID is the write guard for every harness. Current-session
// membership is authoritative; catalog metadata is optional and never gates
// a session-provided ID. An exhausted id is always refused. A non-empty
// Included set is closed: an id outside it is refused. An empty Included
// set is not a credit denial. An explicit_only id is writable only when
// explicit upshift is on.
func CanWriteSessionID(harness, id string, session SessionList, res Resolver) bool {
	if harness == "" || id == "" || id == inheritID || !session.Known || !session.Contains(id) || session.Blocks(id) {
		return false
	}
	if session.CreditsReported && len(session.Included) > 0 && !session.IsIncluded(id) {
		return false
	}
	if res != nil && res.IsExplicitOnly(harness, id) && !ExplicitUpshiftEnabled() {
		return false
	}
	return true
}

// HarnessOwnsID reports whether id belongs to this harness. Session membership
// counts; otherwise only exact catalog identities are used to avoid treating
// a family-prefix match as ownership.
func HarnessOwnsID(harness, id string, session SessionList, res Resolver) bool {
	if id == "" {
		return false
	}
	if session.Known && session.Contains(id) {
		return true
	}
	if res == nil {
		return false
	}
	if exact, ok := res.(ExactIDResolver); ok {
		return exact.IsExactID(harness, id)
	}
	m, ok := res.LookupByID(harness, id)
	return ok && m.ID == id
}

// PlanForSession is the harness-agnostic routing plan. The session list is the
// only candidate source and is ordered least-to-most capable. Catalog metadata
// is optional. An unknown session leaves the current model in place.
func (d Decision) PlanForSession(c HarnessCapabilities, res Resolver, session SessionList) RewritePlan {
	currentID := d.CurrentModel.ID
	requested := d.RequestedID
	if requested == "" {
		requested = currentID
	}
	if requested != "" && !HarnessOwnsID(d.Harness, requested, session, res) {
		return RewritePlan{Model: d.CurrentModel, HoldForeign: true}
	}
	if res != nil && d.ShouldPreserveExplicitModel(currentID, res) {
		return RewritePlan{Model: d.Model, PreserveExplicit: true}
	}
	if !session.Known {
		return RewritePlan{Model: d.CurrentModel}
	}

	id, model, ok := selectSessionTarget(d, res, session)
	if !ok || id == "" || !session.Contains(id) || id == currentID {
		apply := c.CanApplyEffort && currentID != "" && session.Contains(currentID) && d.Model.ID != ""
		return RewritePlan{Model: d.CurrentModel, ApplyEffort: apply}
	}
	model.ID = id
	if model.Harness == "" {
		model.Harness = d.Harness
	}
	rewrite := c.CanRewriteModel
	if c.StrictModelName && model.Native == "" {
		rewrite = false
	}
	plan := RewritePlan{Model: model, RewriteModel: rewrite, ApplyEffort: c.CanApplyEffort}
	if rewrite {
		plan.WriteName = model.WriteName()
	}
	return plan
}

func selectSessionTarget(d Decision, res Resolver, session SessionList) (string, Model, bool) {
	if d.Checked && d.SafeVerdict == VerdictOK {
		return "", Model{}, false
	}
	if session.Blocks(d.CurrentModel.ID) || session.Blocks(d.RequestedID) {
		return leastSessionModelAtOrAbove(d.Harness, session, res, d.Tier)
	}
	switch d.Verdict {
	case VerdictUpshift:
		return strongestSessionModel(d.Harness, session, res)
	case VerdictDownshift, VerdictUnknown:
		return leastSessionModelAtOrAbove(d.Harness, session, res, d.Tier)
	default:
		if d.Model.ID == "" && d.Verdict != VerdictOK {
			return leastSessionModelAtOrAbove(d.Harness, session, res, d.Tier)
		}
		return "", Model{}, false
	}
}

func modelForSessionID(harness, id string, res Resolver) Model {
	if res != nil {
		if m, ok := res.LookupByID(harness, id); ok && m.ID != "" {
			m.ID = id
			if m.Harness == "" {
				m.Harness = harness
			}
			return m
		}
	}
	return Model{ID: id, Harness: harness}
}

// selectableSessionIDs keeps this harness's session order. It drops empty
// ids, inherit, and exhausted ids. explicit_only ids stay only when
// allowExplicit is set. A non-empty Included set is closed: ids outside it
// are dropped, and an empty intersection is an empty result (no fallback).
// An empty Included set does not invent credits and does not consult another
// harness. The selector never adds an id the credit set omitted.
func selectableSessionIDs(harness string, session SessionList, res Resolver, allowExplicit bool) []string {
	if !session.Known {
		return nil
	}
	creditClosed := session.CreditsReported && len(session.Included) > 0
	ids := make([]string, 0, len(session.IDs))
	for _, id := range session.IDs {
		if id == "" || id == inheritID || session.Blocks(id) {
			continue
		}
		explicit := res != nil && res.IsExplicitOnly(harness, id)
		if explicit && !allowExplicit {
			continue
		}
		if creditClosed && !session.IsIncluded(id) {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// leastSessionModelAtOrAbove maps abstract tiers onto the filtered session
// order. One model serves every tier; with multiple models, small maps to
// the first, mid to the midpoint, and frontier to the last. Downshift and
// ordinary tier mapping skip explicit_only. It does not consult names or prices.
func leastSessionModelAtOrAbove(harness string, session SessionList, res Resolver, minTier Tier) (string, Model, bool) {
	ids := selectableSessionIDs(harness, session, res, false)
	if len(ids) == 0 {
		return "", Model{}, false
	}
	start := 0
	switch minTier {
	case TierMid:
		start = len(ids) / 2
	case TierFrontier:
		start = len(ids) - 1
	}
	id := ids[start]
	return id, modelForSessionID(harness, id, res), true
}

func strongestSessionModel(harness string, session SessionList, res Resolver) (string, Model, bool) {
	ids := selectableSessionIDs(harness, session, res, ExplicitUpshiftEnabled())
	if len(ids) == 0 {
		return "", Model{}, false
	}
	id := ids[len(ids)-1]
	return id, modelForSessionID(harness, id, res), true
}
