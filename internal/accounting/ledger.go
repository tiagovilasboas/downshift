// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Ledger is the token ledger every surface shows over one Scope, tokens only,
// never money. Routing tokens are per harness (Numbers.Routing) and never
// summed across harnesses, so the ledger holds only what is not per harness
// routing: the Compactor estimate and the delegate workers.
type Ledger struct {
	// CompactorSaved is the output `downshift compactor` kept out of the
	// context: saved bytes / execwrap.BytesPerToken. An estimate.
	CompactorSaved int64 `json:"compactor_saved"`
	// CompactorEstimate names how CompactorSaved was derived.
	CompactorEstimate string `json:"compactor_estimate"`
	// Workers is the tokens the MCP delegate workers spent, per CLI. They are
	// additional to the routed spawns, never part of them (see WorkerTokens). Absent when no
	// worker ended in the scope; worker events carry no session, so a session
	// scope has none.
	Workers []WorkerTokens `json:"workers,omitempty"`
}

// compactorEstimateNote is the method behind Ledger.CompactorSaved.
var compactorEstimateNote = fmt.Sprintf("estimate: saved bytes / %d", execwrap.BytesPerToken)

// ledgerOf builds the Ledger from the Numbers Compute already holds, so the
// ledger and every other figure share one window.
func ledgerOf(n Numbers) Ledger {
	return Ledger{
		CompactorSaved:    n.Compactor.Tokens,
		CompactorEstimate: compactorEstimateNote,
		Workers:           n.workers,
	}
}

// LedgerLine is the printed ledger for scope, e.g.
// "Token ledger (last 7 days): 45k cut by the compactor (estimate: saved bytes / 4)".
// Routing tokens are per harness and printed apart (telemetry.PrintRouting).
func LedgerLine(l Ledger, scope Scope) string {
	return fmt.Sprintf("Token ledger (%s): %s cut by the compactor (%s)",
		scope.Label(),
		telemetry.ShortTokens(l.CompactorSaved),
		l.CompactorEstimate)
}
