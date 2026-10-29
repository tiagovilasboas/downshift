// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Ledger is the one token ledger every surface shows: three numbers over the
// same Scope, tokens only, never money. The three are shown side by side and
// never summed with each other.
type Ledger struct {
	// Spent is the subagent tokens the harnesses reported: input + output
	// (work) plus cache read + write, for children with a token record.
	Spent int64 `json:"spent"`
	// RoutingSaved is the work and cache that ran on a smaller model than
	// requested (telemetry.TokenSavings). Measured.
	RoutingSaved int64 `json:"routing_saved"`
	// CompactorSaved is the output `downshift compactor` kept out of the
	// context: saved bytes / execwrap.BytesPerToken. An estimate.
	CompactorSaved int64 `json:"compactor_saved"`
	// CompactorEstimate names how CompactorSaved was derived.
	CompactorEstimate string `json:"compactor_estimate"`
	// Workers is the tokens the MCP delegate workers spent, per CLI. They are
	// additional to Spent, never part of it (see WorkerTokens). Absent when no
	// worker ended in the scope; worker events carry no session, so a session
	// scope has none.
	Workers []WorkerTokens `json:"workers,omitempty"`
}

// compactorEstimateNote is the method behind Ledger.CompactorSaved.
var compactorEstimateNote = fmt.Sprintf("estimate: saved bytes / %d", execwrap.BytesPerToken)

// ledgerOf builds the Ledger from the Numbers Compute already holds, so the
// ledger and every other figure share one window.
// The routing figure reuses telemetry.TokensSavedEstimate with no compactor
// part, so the saved-field sum stays in one place.
func ledgerOf(n Numbers) Ledger {
	return Ledger{
		Spent:             n.Routing.WorkTokens + n.Routing.CacheTokens,
		RoutingSaved:      telemetry.TokensSavedEstimate(n.Routing, 0),
		CompactorSaved:    n.Compactor.Tokens,
		CompactorEstimate: compactorEstimateNote,
		Workers:           n.workers,
	}
}

// LedgerLine is the printed ledger for scope, e.g.
// "Token ledger (last 7 days): 1.2M spent · 300k saved by routing · 45k saved by compactor (estimate: saved bytes / 4)".
func LedgerLine(l Ledger, scope Scope) string {
	return fmt.Sprintf("Token ledger (%s): %s spent · %s saved by routing · %s saved by compactor (%s)",
		scope.Label(),
		telemetry.ShortTokens(l.Spent),
		telemetry.ShortTokens(l.RoutingSaved),
		telemetry.ShortTokens(l.CompactorSaved),
		l.CompactorEstimate)
}
