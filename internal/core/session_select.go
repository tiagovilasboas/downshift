// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core

// inheritID is a session sentinel used by more than one harness. It means
// "the parent model", not a selectable target.
const inheritID = "inherit"

// CanWriteSessionID is the write guard for every harness. Current-session
// membership is authoritative; catalog metadata is optional and never gates
// a session-provided ID.
func CanWriteSessionID(harness, id string, session SessionList, res Resolver) bool {
	if harness == "" || id == "" || id == inheritID || !session.Known || !session.Contains(id) || session.Blocks(id) {
		return false
	}
	return res == nil || !res.IsExplicitOnly(harness, id)
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

// selectableSessionIDs keeps the operator's ordering while excluding only
// sentinels, exhausted IDs, and catalog-known explicit-only models.
func selectableSessionIDs(harness string, session SessionList, res Resolver) []string {
	if !session.Known {
		return nil
	}
	ids := make([]string, 0, len(session.IDs))
	for _, id := range session.IDs {
		if id == "" || id == inheritID || session.Blocks(id) || (res != nil && res.IsExplicitOnly(harness, id)) {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// leastSessionModelAtOrAbove maps abstract tiers onto the session's ordered
// choices. One model serves every tier; with multiple models, small maps to
// the first, mid to the midpoint, and frontier to the last. It skips exhausted
// and explicit-only models without consulting names or prices.
func leastSessionModelAtOrAbove(harness string, session SessionList, res Resolver, minTier Tier) (string, Model, bool) {
	ids := selectableSessionIDs(harness, session, res)
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
	for _, id := range ids[start:] {
		if session.IsIncluded(id) {
			return id, modelForSessionID(harness, id, res), true
		}
	}
	return ids[start], modelForSessionID(harness, ids[start], res), true
}

func strongestSessionModel(harness string, session SessionList, res Resolver) (string, Model, bool) {
	ids := selectableSessionIDs(harness, session, res)
	if len(ids) == 0 {
		return "", Model{}, false
	}
	return ids[len(ids)-1], modelForSessionID(harness, ids[len(ids)-1], res), true
}
