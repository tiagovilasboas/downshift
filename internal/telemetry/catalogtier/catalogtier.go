// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package catalogtier adapts the routing catalog (core.Resolver) to the plain
// view telemetry ranks models with (telemetry.ModelCatalog). It imports core
// and never telemetry, so telemetry's own tests can use it too.
package catalogtier

import "github.com/tiagovilasboas/downshift/internal/core"

// View is a core.Resolver seen through telemetry.ModelCatalog. Tier ranks are
// int(core.Tier): small 0, mid 1, frontier 2.
type View struct {
	r core.Resolver
}

// Of wraps r. The composition root assigns it to telemetry.DirectionCatalog.
func Of(r core.Resolver) View { return View{r: r} }

// Lookup returns the catalog id and tier rank of id for harness.
func (v View) Lookup(harness, id string) (string, int, bool) {
	m, ok := v.r.LookupByID(harness, id)
	return m.ID, int(m.Tier), ok
}

// IsExactID reports whether id is an exact catalog id or a declared alias. A
// resolver without core.ExactIDResolver knows no exact id.
func (v View) IsExactID(harness, id string) bool {
	exact, ok := v.r.(core.ExactIDResolver)
	return ok && exact.IsExactID(harness, id)
}

// GatewayTier returns the tier rank of the allowlisted gateway row whose id is
// id: shadow rows first, then live rows (a live row is the one a run goes
// through when the user made it live). A resolver with neither optional
// interface knows no gateway tier.
func (v View) GatewayTier(id string) (int, bool) {
	for _, m := range v.gatewayRows() {
		if m.ID == id {
			return int(m.Tier), true
		}
	}
	return 0, false
}

func (v View) gatewayRows() []core.Model {
	var rows []core.Model
	if gr, ok := v.r.(core.GatewayShadowResolver); ok {
		rows = append(rows, gr.ShadowGateways()...)
	}
	if lr, ok := v.r.(core.GatewayLiveResolver); ok {
		rows = append(rows, lr.LiveGateways()...)
	}
	return rows
}
