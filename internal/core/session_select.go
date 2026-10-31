// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// inheritID is a session sentinel used by more than one harness. It means
// "the parent model", not a selectable target.
const inheritID = "inherit"

// CanWriteSessionID is the write guard for every harness. Current-session
// membership is authoritative; catalog metadata is optional and never gates
// a session-provided ID. An exhausted id is always refused. A non-empty
// Included set is closed: an id outside it is refused. An empty Included
// set is not a credit denial. An explicit_only id is never written by
// routing: it is the owner's deliberate choice, left in place.
func CanWriteSessionID(harness, id string, session SessionList, res Resolver) bool {
	if rejectedSessionModel(harness, id, res) || shadowSessionModel(harness, id, res) {
		return false
	}
	if harness == "" || id == "" || id == inheritID || !session.Known || !session.Contains(id) || session.Blocks(id) || !session.QuotaAllows(harness, id) {
		return false
	}
	if session.NativeAvailability || session.Unordered {
		if res == nil {
			return false
		}
		m, ok := res.LookupByID(harness, id)
		if !ok || m.ID == "" || m.Tier < TierSmall || m.Tier > TierFrontier {
			return false
		}
	}
	if session.CreditsReported && len(session.Included) > 0 && !session.IsIncluded(id) {
		return false
	}
	if res != nil && res.IsExplicitOnly(harness, id) {
		return false
	}
	return true
}

// HarnessOwnsID reports whether id belongs to this harness. Session membership
// counts, including a slug whose catalog resolution is already a member of
// this session. Otherwise only exact catalog identities count, so a
// family-prefix match of a model the session does not list is not ownership.
func HarnessOwnsID(harness, id string, session SessionList, res Resolver) bool {
	if id == "" {
		return false
	}
	if session.Known && session.Contains(id) {
		return true
	}
	if res != nil && session.Known {
		if m, ok := res.LookupByID(harness, id); ok && m.ID != "" && session.Contains(m.ID) {
			return true
		}
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
// only candidate source. Operator/hook/discovery lists are ordered
// least-to-most capable; native lists instead require resolver tier metadata.
// An unknown session leaves the current model in place.
func (d Decision) PlanForSession(c HarnessCapabilities, res Resolver, session SessionList) RewritePlan {
	currentID := d.Baseline.CurrentModel.ID
	requested := d.Baseline.RequestedID
	if requested == "" {
		requested = currentID
	}
	if requested != "" && !HarnessOwnsID(d.Harness, requested, session, res) {
		return RewritePlan{Model: d.Baseline.CurrentModel, HoldForeign: true}
	}
	if (res != nil && d.ShouldPreserveExplicitModel(currentID, res)) || d.Baseline.Pinned {
		return RewritePlan{Model: d.Model, PreserveExplicit: true}
	}
	if !session.Known {
		return RewritePlan{Model: d.Baseline.CurrentModel}
	}

	id, model, ok, creditHeld := selectSessionTarget(d, res, session)
	if !ok || id == "" || !session.Contains(id) || isBaselineModel(d, id, res) {
		apply := c.CanApplyEffort && currentID != "" && session.Contains(currentID) && d.Model.ID != "" && session.QuotaAllows(d.Harness, currentID)
		return RewritePlan{Model: d.Baseline.CurrentModel, ApplyEffort: apply, CreditHeld: creditHeld}
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

func selectSessionTarget(d Decision, res Resolver, session SessionList) (string, Model, bool, bool) {
	if d.Checked && d.SafeVerdict == VerdictOK {
		return "", Model{}, false, false
	}
	return catalogSessionTarget(d, res, session)
}

// catalogSessionTarget is the catalog-tier path that chooses the session model.
func catalogSessionTarget(d Decision, res Resolver, session SessionList) (string, Model, bool, bool) {
	if session.NativeAvailability || session.Unordered {
		return nativeSessionTarget(d, res, session)
	}
	if session.Blocks(d.Baseline.CurrentModel.ID) || session.Blocks(d.Baseline.RequestedID) {
		return leastSessionModelAtOrAbove(d.Harness, session, res, d.Tier)
	}
	switch d.Verdict {
	case VerdictUpshift:
		// An UPSHIFT verdict only reaches here from the operator tier floor,
		// which raised the tier above a weaker current model. Downshift never
		// raises a tier on its own, so the target is the least-capable session
		// model that meets the floored tier, never the strongest available.
		floor := TierSmall
		if d.Baseline.CurrentModel.ID != "" {
			floor = d.Baseline.CurrentModel.Tier
		}
		return upshiftSessionModel(d.Harness, session, res, d.Tier, floor)
	case VerdictDownshift, VerdictUnknown:
		return leastSessionModelAtOrAbove(d.Harness, session, res, d.Tier)
	default:
		if d.Model.ID == "" && d.Verdict != VerdictOK {
			return leastSessionModelAtOrAbove(d.Harness, session, res, d.Tier)
		}
		return "", Model{}, false, false
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
// ids, inherit, and exhausted ids. explicit_only ids are always dropped:
// routing never selects the owner's deliberate choice. A non-empty Included
// set is closed: ids outside it are dropped, and an empty intersection is an
// empty result (no fallback). An empty Included set does not invent credits
// and does not consult another harness. The selector never adds an id the
// credit set omitted.
func selectableSessionIDs(harness string, session SessionList, res Resolver) []string {
	if !session.Known {
		return nil
	}
	creditClosed := session.CreditsReported && len(session.Included) > 0
	ids := make([]string, 0, len(session.IDs))
	for _, id := range session.IDs {
		if id == "" || id == inheritID || session.Blocks(id) || rejectedSessionModel(harness, id, res) {
			continue
		}
		if shadowSessionModel(harness, id, res) {
			// An allowlist shadow row is never a selection.
			continue
		}
		if res != nil && res.IsExplicitOnly(harness, id) {
			continue
		}
		if creditClosed && !session.IsIncluded(id) {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// shadowSessionModel reports whether id resolves to an allowlist shadow row.
// Resolvers that do not expose shadow rows report false.
func shadowSessionModel(harness, id string, res Resolver) bool {
	if checked, ok := res.(interface{ IsShadow(string, string) bool }); ok {
		return checked.IsShadow(harness, id)
	}
	return false
}

func rejectedSessionModel(harness, id string, res Resolver) bool {
	if checked, ok := res.(interface{ SessionModelRejected(string, string) bool }); ok {
		return checked.SessionModelRejected(harness, id)
	}
	return false
}

// leastSessionModelAtOrAbove picks the first id, in session order (least to
// most capable), whose catalog tier is at or above minTier. When any filtered
// id has no catalog tier (operator ids the catalog does not know), the catalog
// cannot rank the list and the index mapping applies instead: one model serves
// every tier; with several, small maps to the first, mid to the midpoint and
// frontier to the last. Both paths skip explicit_only and establish the quality
// floor before the quota filter, so an exhausted id never lowers the floor.
// It does not consult names or prices.
func leastSessionModelAtOrAbove(harness string, session SessionList, res Resolver, minTier Tier) (string, Model, bool, bool) {
	ids := selectableSessionIDs(harness, session, res)
	if len(ids) == 0 {
		return "", Model{}, false, false
	}
	if tiers, ok := catalogTiers(harness, ids, res); ok {
		// Find the model with the lowest tier that is at or above the floor and
		// passes quota. The session list is not guaranteed to be sorted by tier
		// (e.g. KiroCrew may list opus, sonnet, haiku), so scan all candidates.
		found := false
		bestIdx := -1
		bestTier := TierFrontier + 1 // sentinel above any real tier
		for i, id := range ids {
			if tiers[i] < minTier {
				continue
			}
			found = true
			if !session.QuotaAllows(harness, id) {
				continue
			}
			if tiers[i] < bestTier {
				bestIdx, bestTier = i, tiers[i]
			}
		}
		if bestIdx >= 0 {
			return ids[bestIdx], modelForSessionID(harness, ids[bestIdx], res), true, false
		}
		// found: every id at or above the floor was refused by the credit gate.
		// Otherwise nothing in the list reaches the floor: hold, never go below.
		return "", Model{}, false, found
	}
	start := 0
	switch minTier {
	case TierMid:
		start = len(ids) / 2
	case TierFrontier:
		start = len(ids) - 1
	}
	for _, id := range ids[start:] {
		if session.QuotaAllows(harness, id) {
			return id, modelForSessionID(harness, id, res), true, false
		}
	}
	// Every id in the tier window was refused by the credit gate.
	return "", Model{}, false, true
}

// catalogTiers returns the catalog tier of every id, or ok=false when any id
// does not resolve to a valid tier.
func catalogTiers(harness string, ids []string, res Resolver) ([]Tier, bool) {
	if res == nil {
		return nil, false
	}
	tiers := make([]Tier, len(ids))
	for i, id := range ids {
		m, ok := res.LookupByID(harness, id)
		if !ok || m.ID == "" || m.Tier < TierSmall || m.Tier > TierFrontier {
			return nil, false
		}
		tiers[i] = m.Tier
	}
	return tiers, true
}

// upshiftSessionModel serves an UPSHIFT verdict, which only ever comes from
// the operator tier floor: the floor raised the tier above a weaker current
// model. Downshift never raises a tier on its own, so the floor's needed tier
// is the target. The floor is a minimum, so the least tier at or above it that
// a quota-allowed, non-explicit id reaches wins (later session position first
// within a tier). When no id reaches the floor, the strongest available id at
// or above the current model's tier satisfies as much of the operator minimum
// as the session allows, never dropping below it and never selecting an
// explicit_only id. When the catalog cannot rank the list, the position
// mapping applies, as leastSessionModelAtOrAbove does.
func upshiftSessionModel(harness string, session SessionList, res Resolver, minTier, floor Tier) (string, Model, bool, bool) {
	ids := selectableSessionIDs(harness, session, res)
	if len(ids) == 0 {
		return "", Model{}, false, false
	}
	need := max(minTier, floor)
	if tiers, ok := catalogTiers(harness, ids, res); ok {
		// First choice: the least tier at or above the needed tier.
		for tier := need; tier <= TierFrontier; tier++ {
			for i := len(ids) - 1; i >= 0; i-- {
				if tiers[i] == tier && session.QuotaAllows(harness, ids[i]) {
					return ids[i], modelForSessionID(harness, ids[i], res), true, false
				}
			}
		}
		// Nothing reaches the needed tier: take the strongest id the session
		// allows at or above the current model's tier, never below the floor.
		found := false
		for tier := TierFrontier; tier >= floor && tier >= TierSmall; tier-- {
			for i := len(ids) - 1; i >= 0; i-- {
				if tiers[i] != tier {
					continue
				}
				found = true
				if session.QuotaAllows(harness, ids[i]) {
					return ids[i], modelForSessionID(harness, ids[i], res), true, false
				}
			}
		}
		// found: every id at or above the floor was refused by the credit gate.
		// Otherwise nothing in the list reaches the floor: hold, never go below.
		return "", Model{}, false, found
	}
	start := 0
	if minTier == TierMid {
		start = len(ids) / 2
	} else if minTier == TierFrontier {
		start = len(ids) - 1
	}
	for i := len(ids) - 1; i >= start; i-- {
		id := ids[i]
		if session.QuotaAllows(harness, id) {
			return id, modelForSessionID(harness, id, res), true, false
		}
	}
	return "", Model{}, false, true
}
