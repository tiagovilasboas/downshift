// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import "github.com/tiagovilasboas/downshift/internal/core"

// TierView is a core.Resolver seen through the plain view telemetry ranks
// models with (telemetry.ModelCatalog). Its method set matches that interface,
// so telemetry takes it without importing this package. Tier ranks are
// int(core.Tier): small 0, mid 1, frontier 2.
//
// It lives here and not under internal/telemetry because it needs core, and
// the telemetry tree imports core only through fromcore.
type TierView struct {
	r core.Resolver
}

// TierViewOf wraps r. fromcore.Install assigns it to telemetry.DirectionCatalog.
func TierViewOf(r core.Resolver) TierView { return TierView{r: r} }

// Lookup returns the catalog id and tier rank of id for harness.
func (v TierView) Lookup(harness, id string) (string, int, bool) {
	m, ok := v.r.LookupByID(harness, id)
	return m.ID, int(m.Tier), ok
}

// IsExactID reports whether id is an exact catalog id or a declared alias. A
// resolver without core.ExactIDResolver knows no exact id.
func (v TierView) IsExactID(harness, id string) bool {
	exact, ok := v.r.(core.ExactIDResolver)
	return ok && exact.IsExactID(harness, id)
}

// GatewayTier returns the tier rank of the allowlisted gateway row whose id is
// id: shadow rows first, then live rows (a live row is the one a run goes
// through when the user made it live). A resolver with neither optional
// interface knows no gateway tier.
func (v TierView) GatewayTier(id string) (int, bool) {
	for _, m := range v.gatewayRows() {
		if m.ID == id {
			return int(m.Tier), true
		}
	}
	return 0, false
}

func (v TierView) gatewayRows() []core.Model {
	var rows []core.Model
	if gr, ok := v.r.(core.GatewayShadowResolver); ok {
		rows = append(rows, gr.ShadowGateways()...)
	}
	if lr, ok := v.r.(core.GatewayLiveResolver); ok {
		rows = append(rows, lr.LiveGateways()...)
	}
	return rows
}
