// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core

// PlanForSession is the harness-agnostic routing plan. The emitted model id
// is always a member of session. Catalog data only ranks ids that are also
// in the session. An unknown session leaves the current model in place.
func (d Decision) PlanForSession(c HarnessCapabilities, res Resolver, session SessionList) RewritePlan {
	currentID := d.CurrentModel.ID
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
// used only when that same model is in the session. Otherwise a downgrade
// (or a blocked one) uses the cheapest session model, and an upshift uses
// the strongest session model. Unlabeled session ids stay eligible: when
// none of the session ids are in the catalog, the first id is the cheapest
// (operators list cheapest-first) and upshifts do not guess.
func selectSessionTarget(d Decision, res Resolver, session SessionList) (string, Model, bool) {
	if d.ShouldRewriteModel() && d.Model.ID != "" {
		if id, model, ok := sessionForm(d.Harness, d.Model.ID, session, res); ok {
			return id, model, true
		}
	}

	switch d.Verdict {
	case VerdictUpshift:
		return strongestSessionModel(d.Harness, session, res)
	case VerdictDownshift, VerdictUnknown:
		return cheapestSessionModel(d.Harness, session, res)
	default:
		if d.Model.ID == "" && d.Verdict != VerdictOK {
			return cheapestSessionModel(d.Harness, session, res)
		}
		return "", Model{}, false
	}
}

// sessionForm returns the session string for a catalog id. The string written
// to the hook is the session member, which may be the catalog id or an alias
// the session actually listed.
func sessionForm(harness, id string, session SessionList, res Resolver) (string, Model, bool) {
	if id == "" || res == nil || !session.Known {
		return "", Model{}, false
	}
	if session.Contains(id) && !res.IsExplicitOnly(harness, id) {
		return id, modelForSessionID(harness, id, res), true
	}
	looked, ok := res.LookupByID(harness, id)
	if !ok || looked.ID == "" {
		return "", Model{}, false
	}
	if session.Contains(looked.ID) && !res.IsExplicitOnly(harness, looked.ID) {
		return looked.ID, looked, true
	}
	for _, sid := range session.IDs {
		if sid == "" || res.IsExplicitOnly(harness, sid) {
			continue
		}
		sm, ok := res.LookupByID(harness, sid)
		if ok && sm.ID == looked.ID {
			sm.ID = sid
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
	id    string
	model Model
	cost  float64
}

func cheapestSessionModel(harness string, session SessionList, res Resolver) (string, Model, bool) {
	labeled, unlabeled := rankSession(harness, session, res)
	best, ok := pickRanked(labeled, true)
	if ok {
		return best.id, best.model, true
	}
	if len(unlabeled) > 0 {
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
		if id == "" || res.IsExplicitOnly(harness, id) {
			continue
		}
		m, ok := res.LookupByID(harness, id)
		if !ok || m.ID == "" || res.IsExplicitOnly(harness, m.ID) {
			unlabeled = append(unlabeled, ranked{
				id:    id,
				model: Model{ID: id, Harness: harness},
			})
			continue
		}
		m.ID = id
		if m.Harness == "" {
			m.Harness = harness
		}
		labeled = append(labeled, ranked{
			id:    id,
			model: m,
			cost:  m.InputM + m.OutputM,
		})
	}
	return labeled, unlabeled
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
