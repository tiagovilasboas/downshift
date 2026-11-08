// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "github.com/tiagovilasboas/downshift-ai/internal/quotamodel"

// A pool-gated model (catalog flag pool_gated, Model.PoolGated) is a model
// whose use draws on a token pool the hook can only trust when this call
// carries positive evidence about it. A rewrite never names such a model on
// absence of evidence alone; the catalog decides which models are gated, so a
// later slug in the same family is covered without a permanent ban.

// poolGatedModel reports whether id is a pool-gated catalog entry of the
// harness or one of its aliases, and returns that entry.
func poolGatedModel(harness, id string, res Resolver) (Model, bool) {
	if id == "" || res == nil {
		return Model{}, false
	}
	m, ok := res.LookupByID(harness, id)
	if !ok || !m.PoolGated {
		return Model{}, false
	}
	return m, true
}

// withPoolGatedInclusion lets an included catalog id stand for the session
// slug the harness actually listed. Unavailable still wins: a blocked gated id
// is not copied onto the allowlist.
func withPoolGatedInclusion(harness string, session SessionList, res Resolver) SessionList {
	if !session.Known || len(session.Included) == 0 {
		return session
	}
	extra := make([]string, 0, 1)
	for _, id := range session.IDs {
		m, ok := poolGatedModel(harness, id, res)
		if !ok || session.IsIncluded(id) || poolBlocked(harness, session, id, m.ID, res) {
			continue
		}
		if session.IsIncluded(m.ID) {
			extra = append(extra, id)
		}
	}
	if len(extra) == 0 {
		return session
	}
	session.Included = append(append([]string(nil), session.Included...), extra...)
	return session
}

// holdPoolGated stops a rewrite from naming a pool-gated model unless this
// call has positive token-pool evidence. An upshift whose strongest session
// candidate is gated, and that id is unavailable or exhausted, holds the whole
// rewrite instead of substituting a weaker model. It reports the hold, whether
// the credit gate caused it, and whether the pool is exhausted.
func holdPoolGated(harness string, decision Decision, plan RewritePlan, session SessionList, res Resolver) (hold, creditHeld, exhausted bool) {
	ids := make([]string, 0, 4)
	strongest := false
	if decision.Verdict == VerdictUpshift {
		if id := lastSessionID(harness, session, res); id != "" {
			if m, ok := poolGatedModel(harness, id, res); ok {
				strongest = true
				ids = append(ids, id, m.ID)
			}
		}
	}
	writes := false
	if plan.RewriteModel {
		for _, id := range []string{plan.Model.ID, plan.WriteName} {
			if m, ok := poolGatedModel(harness, id, res); ok {
				writes = true
				ids = append(ids, id, m.ID)
			}
		}
	}
	if !strongest && !writes {
		return false, false, false
	}
	ids = uniqueIDs(ids)
	blocked := false
	for _, id := range ids {
		if poolBlocked(harness, session, id, id, res) {
			blocked = true
		}
		if session.Usage != nil && session.QuotaStatus(harness, id) == quotamodel.Exhausted {
			exhausted = true
		}
	}
	if blocked || exhausted {
		return true, true, exhausted
	}
	if (writes || strongest) && !poolPositive(harness, session, ids) {
		// Unobserved pool: do not emit the gated model and do not call the hold
		// credit. A non-gated rewrite already chosen is left alone.
		if writes || !plan.RewriteModel {
			return true, false, false
		}
	}
	return false, false, false
}

func poolBlocked(harness string, session SessionList, sessionID, catalogID string, res Resolver) bool {
	if session.Blocks(sessionID) || (catalogID != "" && catalogID != sessionID && session.Blocks(catalogID)) {
		return true
	}
	for _, id := range session.Exhausted {
		if id == sessionID || id == catalogID {
			return true
		}
		if m, ok := poolGatedModel(harness, id, res); ok && (m.ID == catalogID || m.ID == sessionID || id == sessionID) {
			return true
		}
	}
	return false
}

func poolPositive(harness string, session SessionList, ids []string) bool {
	for _, id := range ids {
		if session.IsIncluded(id) {
			return true
		}
	}
	if session.Usage == nil {
		return false
	}
	for _, id := range ids {
		if session.QuotaStatus(harness, id) == quotamodel.Available {
			return true
		}
	}
	return false
}

// lastSessionID returns the last non-inherit, non-explicit_only session id.
func lastSessionID(harness string, session SessionList, res Resolver) string {
	if !session.Known {
		return ""
	}
	last := ""
	for _, id := range session.IDs {
		if id == "" || id == inheritID {
			continue
		}
		if res != nil && res.IsExplicitOnly(harness, id) {
			continue
		}
		last = id
	}
	return last
}

func uniqueIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// applyPoolGate is the pool-gate step of PlanSession. When the gate holds the
// rewrite it clears the plan's writes and records the credit evidence on the
// decision: absent marks are not credit; exhausted and unavailable are.
func (d *Decision) applyPoolGate(plan RewritePlan, session SessionList, res Resolver) RewritePlan {
	hold, creditHeld, exhausted := holdPoolGated(d.Harness, *d, plan, session, res)
	if !hold {
		return plan
	}
	d.Session.CreditHeld = creditHeld
	if exhausted {
		d.Session.QuotaStatus = string(quotamodel.Exhausted)
	} else if d.Session.QuotaStatus == string(quotamodel.Available) {
		d.Session.QuotaStatus = string(quotamodel.Unknown)
	}
	plan.RewriteModel, plan.ApplyEffort = false, false
	return plan
}
