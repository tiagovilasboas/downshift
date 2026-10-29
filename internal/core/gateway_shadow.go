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

// RecordGatewayWould stamps Shadow.GatewayWould: the smallest allowlisted row
// whose tier equals the decision tier. It never changes Model, Tier, Verdict
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

// gatewayWouldFor returns the id of the smallest row of the given tier, or
// empty. Smallest means the lowest known list price; rows with no price rank
// after priced rows, and ties break on id so the result is deterministic.
func gatewayWouldFor(tier Tier, rows []Model) string {
	best, ok := smallestRow(rows, func(m Model) bool { return m.Tier == tier })
	if !ok {
		return ""
	}
	return best.ID
}

// smallestRow returns the smallest gateway row that match accepts, using
// smallerGateway. ok is false when no row matches.
func smallestRow(rows []Model, match func(Model) bool) (best Model, ok bool) {
	for _, m := range rows {
		if m.ID == "" || !match(m) {
			continue
		}
		if !ok || smallerGateway(m, best) {
			best, ok = m, true
		}
	}
	return best, ok
}

// price is the catalog list price (input plus output per 1M); 0 means unknown.
func price(m Model) float64 { return m.InputM + m.OutputM }

// smallerGateway orders gateway rows: priced before unpriced, then by price,
// then by id.
func smallerGateway(a, b Model) bool {
	pa, pb := price(a), price(b)
	if (pa > 0) != (pb > 0) {
		return pa > 0
	}
	if pa != pb {
		return pa < pb
	}
	return a.ID < b.ID
}
