// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core

// inheritID is a session sentinel used by more than one harness. It means
// "the parent model", not a selectable target.
const inheritID = "inherit"

// CanWriteCatalogID is the write guard for every harness.
// The string emitted must be a canonical id of this harness's own catalog
// entry, present in the session, and not marked exhausted. Aliases, family
// prefixes, inherit, and another harness's canonical id all fail.
func CanWriteCatalogID(harness, id string, session SessionList, res Resolver) bool {
	if harness == "" || id == "" || id == inheritID || res == nil {
		return false
	}
	if !session.Known || !session.Contains(id) || session.Blocks(id) || res.IsExplicitOnly(harness, id) {
		return false
	}
	if exact, ok := res.(ExactIDResolver); ok && !exact.IsExactID(harness, id) {
		return false
	}
	m, ok := res.LookupByID(harness, id)
	return ok && m.Harness == harness && m.ID == id
}

// HarnessOwnsID reports whether id belongs to this harness.
// Session membership counts. Exact catalog ids and aliases count.
// A family-prefix match does not.
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

// PlanForSession is the harness-agnostic routing plan. The emitted model id
// is always a member of session. Catalog data only ranks ids that are also
// in the session. An unknown session leaves the current model in place.
func (d Decision) PlanForSession(c HarnessCapabilities, res Resolver, session SessionList) RewritePlan {
	currentID := d.CurrentModel.ID
	requested := d.RequestedID
	if requested == "" {
		requested = currentID
	}
	// A payload id from another harness is not ours to rewrite. Family-prefix
	// lookup must not turn "grok-4.7-xhigh" into another product's "grok-4.6".
	if requested != "" && !HarnessOwnsID(d.Harness, requested, session, res) {
		return RewritePlan{Model: d.CurrentModel, HoldForeign: true}
	}
	if res != nil && d.ShouldPreserveExplicitModel(currentID, res) {
		return RewritePlan{Model: d.Model, PreserveExplicit: true}
	}
	if res == nil || !session.Known {
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
	return RewritePlan{
		Model:        model,
		RewriteModel: c.CanRewriteModel,
		ApplyEffort:  c.CanApplyEffort,
	}
}

// selectSessionTarget picks the id to write. The classified catalog id is
// used only when that same model is in the session. Otherwise routing
// depends on the verdict:
//
//   - Held by a guardrail (Checked and SafeVerdict == VerdictOK, e.g. R1/R2/R5
//     on a downshift): do not rewrite. The classified verdict must never be
//     used to bypass the safe action.
//   - VerdictUpshift: use strongest session model (never downgrade if escalation fails).
//   - VerdictDownshift: use the cheapest session model at or above the
//     classified tier when the catalog target is not in the session.
//   - VerdictUnknown or Model.ID=="": same, cheapest at or above the tier.
//
// The fallback never goes below the classified tier: a MEDIUM (or
// risk-floored) task is never written to a small model just because the mid
// catalog model is missing from the session.
//   - Otherwise (VerdictOK): do not rewrite.
//
// Unlabeled session ids stay eligible only for small-tier targets: when none
// are in the catalog, the first id is the cheapest (operators list
// cheapest-first). Upshifts and mid/frontier targets do not guess.
// A selectable id must still be this harness's canonical catalog id.
func selectSessionTarget(d Decision, res Resolver, session SessionList) (string, Model, bool) {
	exhaustedCurrent := session.Blocks(d.CurrentModel.ID) || session.Blocks(d.RequestedID)
	if d.ShouldRewriteModel() && d.Model.ID != "" && !session.Blocks(d.Model.ID) {
		if id, model, ok := sessionForm(d.Harness, d.Model.ID, session, res); ok {
			return id, model, true
		}
	}
	if d.Checked && d.SafeVerdict == VerdictOK {
		return "", Model{}, false
	}
	if exhaustedCurrent {
		if id, model, ok := availableAtTier(d.Harness, d.Tier, session, res); ok {
			return id, model, true
		}
		// Stay at or above the classified tier. Do not pick a cheaper model
		// just because the current id has no token budget left.
		return cheapestSessionModel(d.Harness, session, res, d.Tier)
	}

	switch d.Verdict {
	case VerdictUpshift:
		return strongestSessionModel(d.Harness, session, res)
	case VerdictDownshift, VerdictUnknown:
		return cheapestSessionModel(d.Harness, session, res, d.Tier)
	default:
		if d.Model.ID == "" && d.Verdict != VerdictOK {
			return cheapestSessionModel(d.Harness, session, res, d.Tier)
		}
		return "", Model{}, false
	}
}

// sessionForm returns the session string for a catalog id. The string written
// to the hook is the session member, which may be the catalog id or an alias
// the session actually listed.
func sessionForm(harness, id string, session SessionList, res Resolver) (string, Model, bool) {
	if id == "" || res == nil || !session.Known || session.Blocks(id) || id == inheritID {
		return "", Model{}, false
	}
	if session.Contains(id) && !res.IsExplicitOnly(harness, id) {
		lookedUp, ok := res.LookupByID(harness, id)
		if !ok || lookedUp.Harness != harness || lookedUp.ID != id {
			return "", Model{}, false
		}
		if alt, model, ok := preferIncludedSibling(harness, id, session, res); ok {
			return alt, model, true
		}
		lookedUp.ID = id
		return id, lookedUp, true
	}
	looked, ok := res.LookupByID(harness, id)
	if !ok || looked.ID == "" {
		return "", Model{}, false
	}
	if session.Contains(looked.ID) && !res.IsExplicitOnly(harness, looked.ID) {
		return looked.ID, looked, true
	}
	for _, sid := range session.IDs {
		if sid == "" || sid == inheritID || session.Blocks(sid) || res.IsExplicitOnly(harness, sid) {
			continue
		}
		sm, ok := res.LookupByID(harness, sid)
		if ok && sm.Harness == harness && sm.ID == sid && sm.ID == looked.ID {
			return sid, sm, true
		}
	}
	return "", Model{}, false
}

func modelForSessionID(harness, id string, res Resolver) Model {
	if m, ok := res.LookupByID(harness, id); ok && m.ID != "" {
		m.ID = id
		if m.Harness == "" {
			m.Harness = harness
		}
		return m
	}
	return Model{ID: id, Harness: harness}
}

type ranked struct {
	id       string
	model    Model
	cost     float64
	included bool
}

// cheapestSessionModel returns the cheapest session model whose tier is at
// least minTier. Unlabeled ids are a fallback only for small-tier targets,
// and only when rankSession still returns them. Canonical catalog ids are
// preferred; a foreign harness id is never unlabeled-eligible.
func cheapestSessionModel(harness string, session SessionList, res Resolver, minTier Tier) (string, Model, bool) {
	labeled, unlabeled := rankSession(harness, session, res)
	var eligible []ranked
	for _, r := range labeled {
		if r.model.Tier >= minTier {
			eligible = append(eligible, r)
		}
	}
	if best, ok := pickRanked(eligible, true); ok {
		return best.id, best.model, true
	}
	if len(labeled) == 0 && minTier == TierSmall && len(unlabeled) > 0 {
		return unlabeled[0].id, unlabeled[0].model, true
	}
	return "", Model{}, false
}

func strongestSessionModel(harness string, session SessionList, res Resolver) (string, Model, bool) {
	labeled, _ := rankSession(harness, session, res)
	best, ok := pickRanked(labeled, false)
	if !ok {
		return "", Model{}, false
	}
	return best.id, best.model, true
}

func rankSession(harness string, session SessionList, res Resolver) (labeled, unlabeled []ranked) {
	for _, id := range session.IDs {
		if id == "" || id == inheritID || session.Blocks(id) || res.IsExplicitOnly(harness, id) {
			continue
		}
		m, ok := res.LookupByID(harness, id)
		// Only this harness's canonical catalog id. An alias or a family
		// prefix returns a different ID and must not be written.
		if !ok || m.Harness != harness || m.ID != id || res.IsExplicitOnly(harness, m.ID) {
			continue
		}
		labeled = append(labeled, ranked{
			id:       id,
			model:    m,
			cost:     m.InputM + m.OutputM,
			included: session.IsIncluded(id),
		})
	}
	return labeled, unlabeled
}

func availableAtTier(harness string, tier Tier, session SessionList, res Resolver) (string, Model, bool) {
	labeled, _ := rankSession(harness, session, res)
	var at []ranked
	for _, candidate := range labeled {
		if candidate.model.Tier == tier {
			at = append(at, candidate)
		}
	}
	best, ok := pickRanked(at, true)
	if !ok {
		return "", Model{}, false
	}
	return best.id, best.model, true
}

// preferIncludedSibling keeps a classified id unless the same tier has a
// session model the operator marked as still having token budget.
func preferIncludedSibling(harness, id string, session SessionList, res Resolver) (string, Model, bool) {
	if session.IsIncluded(id) || res == nil {
		return "", Model{}, false
	}
	current, ok := res.LookupByID(harness, id)
	if !ok {
		return "", Model{}, false
	}
	var pool []ranked
	for _, sid := range session.IDs {
		if sid == "" || sid == id || sid == inheritID || session.Blocks(sid) || !session.IsIncluded(sid) {
			continue
		}
		if res.IsExplicitOnly(harness, sid) {
			continue
		}
		sm, ok := res.LookupByID(harness, sid)
		if !ok || sm.Harness != harness || sm.ID != sid || sm.Tier != current.Tier {
			continue
		}
		pool = append(pool, ranked{
			id:       sid,
			model:    sm,
			cost:     sm.InputM + sm.OutputM,
			included: true,
		})
	}
	best, ok := pickRanked(pool, true)
	if !ok {
		return "", Model{}, false
	}
	return best.id, best.model, true
}

func pickRanked(in []ranked, cheapest bool) (ranked, bool) {
	if len(in) == 0 {
		return ranked{}, false
	}
	best := in[0]
	for _, candidate := range in[1:] {
		if cheapest && cheaperThan(candidate, best) {
			best = candidate
			continue
		}
		if !cheapest && strongerThan(candidate, best) {
			best = candidate
		}
	}
	var included []ranked
	for _, candidate := range in {
		if candidate.included && candidate.model.Tier == best.model.Tier {
			included = append(included, candidate)
		}
	}
	if len(included) == 0 {
		return best, true
	}
	best = included[0]
	for _, candidate := range included[1:] {
		if cheapest && cheaperThan(candidate, best) {
			best = candidate
			continue
		}
		if !cheapest && strongerThan(candidate, best) {
			best = candidate
		}
	}
	return best, true
}

func cheaperThan(a, b ranked) bool {
	if a.model.Tier != b.model.Tier {
		return a.model.Tier < b.model.Tier
	}
	return a.cost < b.cost
}

func strongerThan(a, b ranked) bool {
	if a.model.Tier != b.model.Tier {
		return a.model.Tier > b.model.Tier
	}
	return a.cost > b.cost
}
