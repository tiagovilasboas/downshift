// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// GatewayShadowResolver is an optional Resolver extension. It lists the
// allowlisted gateway rows, which are shadow-only: they are never an
// automatic target, only a candidate for Decision.Shadow.GatewayWould.
// Resolvers that do not implement it record no gateway candidate.
type GatewayShadowResolver interface {
	ShadowGateways() []Model
}

// RecordGatewayWould stamps Shadow.GatewayWould: the allowlisted row whose tier
// equals the decision tier, chosen by id. It never changes Model, Tier, Verdict
// or the rewrite plan. The choice is the same for every harness, because the
// allowlist is a user fact and not a harness fact.
func (d *Decision) RecordGatewayWould(res Resolver) {
	d.Shadow.GatewayWould = ""
	gr, ok := res.(GatewayShadowResolver)
	if !ok {
		return
	}
	d.Shadow.GatewayWould = gatewayWouldFor(d.Tier, gr.ShadowGateways())
}

// gatewayWouldFor returns the id of the row of the given tier with the lowest
// id, or empty. Price is not read (owner decision 2026-10-31: routing and its
// shadow decide by tier only); the id tiebreak keeps the result deterministic.
func gatewayWouldFor(tier Tier, rows []Model) string {
	best, ok := smallestRow(rows, func(m Model) bool { return m.Tier == tier })
	if !ok {
		return ""
	}
	return best.ID
}

// smallestRow returns the gateway row that match accepts with the lowest id.
// ok is false when no row matches.
func smallestRow(rows []Model, match func(Model) bool) (best Model, ok bool) {
	for _, m := range rows {
		if m.ID == "" || !match(m) {
			continue
		}
		if !ok || m.ID < best.ID {
			best, ok = m, true
		}
	}
	return best, ok
}
