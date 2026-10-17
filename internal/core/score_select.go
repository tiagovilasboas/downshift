// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// Capability selection: choose the session model by the capability the task
// needs, measured with public scores, instead of by the catalog tier of the
// session ids. It is harness-agnostic: it reads only the session list, the
// resolver and ScoreHook, and every adapter inherits it through PlanSession.
//
// Shadow by default. The pick is always computed and recorded as
// Decision.ScoreWould; it changes the routing output only when
// DOWNSHIFT_SCORE_ROUTING=1.

// ScoreHook returns one comparable 0-100 capability score for a model id of a
// harness. ok is false when the score is unknown. It is nil unless the score
// index (internal/scoreindex) is linked in, which registers it in init like
// TierFloorHook. The hook must be offline and fast: it runs inside the hook.
var ScoreHook func(harness, id string) (score float64, ok bool)

// ScoreRoutingEnabled reports whether the capability pick may change the model.
func ScoreRoutingEnabled() bool { return scoreRoutingEnv() }

// The capability a task needs, as a fraction of the span between the weakest
// and the strongest selectable model of the session itself:
//
//	need(tier) = lo + (hi-lo) * f(tier)
//
// These are the only tuning knobs of the capability path.
const (
	ScoreFracSmall    = 0.0
	ScoreFracMid      = 0.60
	ScoreFracFrontier = 0.95
)

// scoreEpsilon absorbs float rounding when a score sits exactly on the need.
const scoreEpsilon = 1e-9

func scoreFraction(t Tier) float64 {
	switch t {
	case TierSmall:
		return ScoreFracSmall
	case TierFrontier:
		return ScoreFracFrontier
	default:
		return ScoreFracMid
	}
}

// scoreAbstains lists the decisions that stay on the catalog-tier path: no
// hook, risk, COMPLEX work, and any deterministic lift (risk floor, graph
// escalation, NB upshift, operator tier floor). Those choose a tier floor on
// purpose, and a score must never undercut it.
func scoreAbstains(d Decision) bool {
	return ScoreHook == nil || d.Complexity == Complex || len(d.Risk) > 0 || deterministicLift(d)
}

// scoreCandidates are the session ids the capability path may pick from, in
// session order: selectable (not exhausted, not explicit_only, inside a closed
// credit set) and writable once the quota gate is set aside (the gate applies
// at pick time, so an exhausted id never moves the range).
func scoreCandidates(d Decision, res Resolver, session SessionList) []string {
	ids := selectableSessionIDs(d.Harness, session, res, false)
	open := session
	open.QuotaRequired = false
	out := ids[:0:0]
	for _, id := range ids {
		if CanWriteSessionID(d.Harness, id, open, res) {
			out = append(out, id)
		}
	}
	return out
}

// scoreSelect returns the cheapest candidate whose score reaches the need of
// d.Tier, relative to the session's own range. ok is false (abstain) when the
// hook is absent, fewer than two candidates exist, any candidate has no score,
// the decision is one the path leaves alone, or nothing usable passes.
func scoreSelect(d Decision, res Resolver, session SessionList) (string, Model, bool) {
	if scoreAbstains(d) || !session.Known {
		return "", Model{}, false
	}
	ids := scoreCandidates(d, res, session)
	if len(ids) < 2 {
		return "", Model{}, false
	}
	scores := make(map[string]float64, len(ids))
	lo, hi := 0.0, 0.0
	for i, id := range ids {
		s, ok := ScoreHook(d.Harness, id)
		if !ok {
			return "", Model{}, false
		}
		scores[id] = s
		if i == 0 || s < lo {
			lo = s
		}
		if i == 0 || s > hi {
			hi = s
		}
	}
	need := lo + (hi-lo)*scoreFraction(d.Tier)

	// A public score never overrides the catalog tier: a cheap small model
	// with a high score is not a mid-tier model.
	var passing []Model
	for _, id := range ids {
		if m, ok := lookupTier(res, d.Harness, id); ok && m.Tier < d.Tier {
			continue
		}
		if scores[id] >= need-scoreEpsilon && session.quotaAllows(d.Harness, id) {
			passing = append(passing, modelForSessionID(d.Harness, id, res))
		}
	}
	if len(passing) == 0 {
		return "", Model{}, false
	}
	best := passing[0]
	if allPriced(passing) {
		for _, m := range passing[1:] {
			if price(m) < price(best) {
				best = m
			}
		}
	}
	// An unconfident decision never goes above the model already running.
	if !d.Confident {
		cur := d.Baseline.RequestedID
		if cur == "" {
			cur = d.Baseline.CurrentModel.ID
		}
		if cs, ok := ScoreHook(d.Harness, cur); ok && scores[best.ID] > cs {
			return "", Model{}, false
		}
	}
	return best.ID, best, true
}

// price is the catalog list price (input plus output per 1M); 0 means unknown.
func price(m Model) float64 { return m.InputM + m.OutputM }

func allPriced(ms []Model) bool {
	for _, m := range ms {
		if price(m) <= 0 {
			return false
		}
	}
	return true
}

// RecordScoreWould computes the capability pick for the session and stamps it
// on the decision: ScoreWould (the id), ScoreDiffers (it is not the model the
// catalog-tier path leaves the child on) and ScoreWouldCostRatio (the pick's
// list price over that model's, when both are known). It never changes Model,
// Tier or Verdict, and the comparison is always against the catalog path, so
// the figures mean the same in shadow and in live mode. PlanSession calls it;
// adapters that call PlanForSession directly call it after planning.
func (d *Decision) RecordScoreWould(res Resolver, session SessionList) {
	d.Shadow.ScoreWould, d.Shadow.ScoreDiffers, d.Shadow.ScoreWouldCostRatio = "", false, 0
	id, m, ok := scoreSelect(*d, res, session)
	if !ok {
		return
	}
	d.Shadow.ScoreWould = id
	base := d.Baseline.CurrentModel.ID
	if !(d.Checked && d.SafeVerdict == VerdictOK) {
		if cid, _, cok, _ := catalogSessionTarget(*d, res, session); cok && cid != "" && session.Contains(cid) {
			base = cid
		}
	}
	d.Shadow.ScoreDiffers = id != base
	if base == "" {
		return
	}
	if b := modelForSessionID(d.Harness, base, res); price(b) > 0 && price(m) > 0 {
		d.Shadow.ScoreWouldCostRatio = price(m) / price(b)
	}
}

// lookupTier resolves id in the catalog; ok is false without a resolver.
func lookupTier(res Resolver, harness, id string) (Model, bool) {
	if res == nil {
		return Model{}, false
	}
	return res.LookupByID(harness, id)
}
