// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package fromcore

import (
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Install wires telemetry to the router at the composition root: r becomes
// the catalog that ranks and names models (telemetry.DirectionCatalog), and
// the declared harnesses come from core.AllHarnessCaps. A nil r leaves no
// catalog, so priced records stay unattributed.
func Install(r core.Resolver) {
	telemetry.DirectionCatalog = nil
	if r != nil {
		telemetry.DirectionCatalog = catalog.TierViewOf(r)
	}
	telemetry.DeclaredHarnesses = HarnessSeams()
}

// HarnessSeams lists every declared harness, in core.AllHarnessCaps order,
// with its child-end event and token usage source.
func HarnessSeams() []telemetry.HarnessSeam {
	all := core.AllHarnessCaps()
	out := make([]telemetry.HarnessSeam, 0, len(all))
	for _, n := range all {
		out = append(out, telemetry.HarnessSeam{Harness: n.Harness, ChildEndEvent: n.Caps.ChildEndEvent, UsageSource: n.Caps.UsageSource})
	}
	return out
}
