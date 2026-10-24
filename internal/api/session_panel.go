// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// sessionPanelResponse is GET /api/session/{id} and /api/session/current. The
// presentation (spawns, labels, agent types) is telemetry.BuildPanel's; the
// counts, savings and Compactor are the session's accounting.Numbers; Report is
// the report block.
type sessionPanelResponse struct {
	telemetry.PanelView
	Compactor           *compactorWire     `json:"compactor,omitempty"`
	TokensSavedEstimate int64              `json:"tokens_saved_estimate"`
	Report              accounting.Numbers `json:"report"`
}

// buildSessionPanel renders one session's panel from its Numbers.
func buildSessionPanel(events []telemetry.Event, execEvents []execwrap.Event, report accounting.Numbers, hash string) sessionPanelResponse {
	n := sessionNumbers(events, execEvents, hash)
	v := telemetry.BuildPanel(events, hash)
	setPanelCounts(&v, n)
	return sessionPanelResponse{
		PanelView:           v,
		Compactor:           compactorOf(n.Compactor, shortHash(hash)),
		TokensSavedEstimate: n.TokensSavedEstimate,
		Report:              report,
	}
}

// setPanelCounts takes the panel's savings and spawn counts from the Numbers,
// so the panel and the session list show the same figures.
func setPanelCounts(v *telemetry.PanelView, n accounting.Numbers) {
	v.Tokens = n.Routing
	v.ModelSwitched = n.ModelSwitched
	v.EffortChanged = n.EffortChanged
	v.Switched = n.ModelSwitched + n.EffortChanged
	v.Held = n.Held
	v.HeldGuardrail = n.HeldGuardrail
	v.UpshiftNotApplied = n.UpshiftNotApplied
}
