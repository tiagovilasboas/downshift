// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// GatewayLiveResolver is an optional Resolver extension. It lists the
// allowlisted gateway rows the user marked live. Unlike a shadow row, a live
// row can be the target of a rewrite, and only for the harness the row names.
type GatewayLiveResolver interface {
	LiveGateways() []Model
}

// applyLiveGateway replaces the target of a rewrite with the smallest live
// gateway row of the decision's harness and tier. It never adds a rewrite:
// a plan that keeps the current model, holds or preserves stays as it is, so
// a live row changes where a spawn that is already moving goes, nothing else.
func (d Decision) applyLiveGateway(plan RewritePlan, res Resolver) RewritePlan {
	if !plan.RewriteModel {
		return plan
	}
	live, ok := res.(GatewayLiveResolver)
	if !ok {
		return plan
	}
	row, ok := smallestRow(live.LiveGateways(), func(m Model) bool {
		return m.Harness == d.Harness && m.Tier == d.Tier && m.Gateway != ""
	})
	if !ok {
		return plan
	}
	plan.Model = row
	plan.WriteName = row.WriteName()
	return plan
}

// liveGatewayWritable reports whether id is the id of a live gateway row for
// harness. The write guard accepts it only when the session is known; the
// row is not a session member, so the planner is the only source of it.
func liveGatewayWritable(harness, id string, session SessionList, res Resolver) bool {
	live, ok := res.(GatewayLiveResolver)
	if !ok || id == "" || !session.Known {
		return false
	}
	for _, m := range live.LiveGateways() {
		if m.Harness == harness && m.Gateway != "" && m.ID == id {
			return true
		}
	}
	return false
}
