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
