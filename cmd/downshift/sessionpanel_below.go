// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// belowSessionRow is one harness's routing tokens (below the session model,
// the requested-model fallback kept apart), the same numbers as the report's
// per-harness figures. Observed false means no spawn of that harness had a
// session model: the pane reads "unobserved", never 0. Rows are never summed
// across harnesses. Counts and token numbers only: no prompt, path or id.
type belowSessionRow struct {
	Harness  string `json:"harness"`
	Observed bool   `json:"observed"`
	telemetry.RoutingRow
}

// belowSessionRows reads the report's per-harness rows over scope.
func belowSessionRows(events []telemetry.Event, execEvents []execwrap.Event, scope accounting.Scope) []belowSessionRow {
	rows := accounting.TokensByHarness(events, execEvents, scope, nil)
	out := make([]belowSessionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, belowSessionRow{Harness: r.Harness, Observed: r.Routing.Observed(), RoutingRow: r.Routing})
	}
	return out
}
