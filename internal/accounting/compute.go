// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Scope kinds. Every surface (report, Agents panel, progress bar) asks Compute
// with one Scope, so the routing and the Compactor share one window.
const (
	ScopeDays    = "days"
	ScopeSession = "session"
	ScopeAll     = "all"
)

// Scope is the window of one set of Numbers. Days applies to ScopeDays (<= 0
// means all time, as telemetry.FilterByDays does). SessionHash applies to
// ScopeSession and is the hashed id (telemetry.HashSessionID), never the raw one.
type Scope struct {
	Kind        string `json:"kind"`
	Days        int    `json:"days,omitempty"`
	SessionHash string `json:"session_hash,omitempty"`
}

// Label names the window for people: "last 30 days", "this session", "all time".
func (s Scope) Label() string {
	switch {
	case s.Kind == ScopeSession:
		return "this session"
	case s.Kind == ScopeDays && s.Days > 0:
		return fmt.Sprintf("last %d days", s.Days)
	}
	return "all time"
}

// Numbers is what every surface shows for one Scope. Counts and tokens only:
// no prompt, path, command text or raw session id.
type Numbers struct {
	Scope Scope `json:"scope"`
	// Routing is the tokens that ran on a cheaper model than requested, from
	// telemetry.TokenSavings over the scoped events.
	Routing telemetry.TokenSaving `json:"routing"`
	// Compactor is the shell-output compression over the same scope.
	Compactor Compactor `json:"compactor"`
	// ModelSwitched and EffortChanged split the spawns Downshift changed.
	ModelSwitched int `json:"model_switched"`
	EffortChanged int `json:"effort_changed"`
	// Held is every spawn left unchanged; HeldGuardrail is those kept by a
	// guardrail, and UpshiftNotApplied the upshift verdicts that did not apply.
	Held              int `json:"held"`
	HeldGuardrail     int `json:"held_guardrail"`
	UpshiftNotApplied int `json:"upshift_not_applied"`
	// TokensSavedEstimate is routing work and cache saved plus the Compactor's
	// estimated tokens (telemetry.TokensSavedEstimate). An estimate.
	TokensSavedEstimate int64 `json:"tokens_saved_estimate"`
	// Ledger is the three token figures side by side: spent, saved by
	// routing, saved by the Compactor (an estimate).
	Ledger Ledger `json:"ledger"`
	// workers feeds Ledger.Workers; it is not part of the JSON itself.
	workers []WorkerTokens
	// Display holds the ready-to-print text of the figures above.
	Display Display `json:"display"`
}

// Compactor totals the exec log over one scope. Bytes are measured; Tokens is
// SavedBytes / execwrap.BytesPerToken and is an estimate.
type Compactor struct {
	Commands   int64 `json:"commands"`
	Compacted  int64 `json:"compacted"`
	RawBytes   int64 `json:"raw_bytes"`
	SavedBytes int64 `json:"saved_bytes"`
	Tokens     int64 `json:"tokens"`
}

// Display is the final text of the figures, so no surface formats them again.
type Display struct {
	TokensSaved string `json:"tokens_saved"`
	// TokensSavedShort is the estimate in the CLI token format ("20.7M"), "0" when zero.
	TokensSavedShort string `json:"tokens_saved_short"`
	// ScopeLabel is Scope.Label(): "last 30 days", "this session", "all time".
	ScopeLabel string `json:"scope_label"`
	Routing    string `json:"routing"`
	Compactor  string `json:"compactor"`
	// Ledger is LedgerLine(Numbers.Ledger, Scope).
	Ledger string `json:"ledger"`
}

// Compute returns the Numbers for scope. It is pure: the caller reads the
// telemetry log and the exec log (see ReadExecEvents) and passes them in.
//
// Exec events carry a session only from #433 on. For ScopeSession, an exec
// event with no session belongs to no session and is left out; for the other
// scopes every exec event in the window counts. Telemetry decisions with no
// session id count in Routing but not in the spawn counts, because
// telemetry.BuildPanel reads one session at a time.
func Compute(events []telemetry.Event, execEvents []execwrap.Event, scope Scope) Numbers {
	scoped := scopeTelemetry(events, scope)
	n := Numbers{Scope: scope}
	n.Routing = telemetry.TokenSavings(scoped).Total
	n.ModelSwitched, n.EffortChanged, n.Held, n.HeldGuardrail, n.UpshiftNotApplied = countSpawns(scoped)
	n.Compactor = compactorTotals(scopeExec(execEvents, scope, time.Now()))
	n.TokensSavedEstimate = telemetry.TokensSavedEstimate(n.Routing, n.Compactor.Tokens)
	n.workers = workerTokensOf(scoped)
	n.Ledger = ledgerOf(n)
	n.Display = displayOf(n)
	return n
}

// scopeTelemetry keeps the telemetry events of scope.
func scopeTelemetry(events []telemetry.Event, scope Scope) []telemetry.Event {
	switch scope.Kind {
	case ScopeSession:
		var out []telemetry.Event
		for _, ev := range events {
			if scope.SessionHash != "" && ev.SessionID == scope.SessionHash {
				out = append(out, ev)
			}
		}
		return out
	case ScopeDays:
		return telemetry.FilterByDays(events, scope.Days)
	}
	return events
}

// scopeExec keeps the exec events of scope. A timestamp that does not parse
// is outside every day window (fail-closed, as telemetry.FilterByDays).
func scopeExec(execEvents []execwrap.Event, scope Scope, now time.Time) []execwrap.Event {
	var out []execwrap.Event
	switch scope.Kind {
	case ScopeSession:
		for _, ev := range execEvents {
			if scope.SessionHash != "" && ev.Session == scope.SessionHash {
				out = append(out, ev)
			}
		}
	case ScopeDays:
		if scope.Days <= 0 {
			return execEvents
		}
		cutoff := now.Add(-time.Duration(scope.Days) * 24 * time.Hour)
		for _, ev := range execEvents {
			if t, err := time.Parse(time.RFC3339, ev.TS); err == nil && t.After(cutoff) {
				out = append(out, ev)
			}
		}
	default:
		return execEvents
	}
	return out
}

// compactorTotals sums the scoped exec events. It mirrors execwrap.Summarize,
// which only reads a file; a pure summarizer belongs in execwrap.
func compactorTotals(evs []execwrap.Event) Compactor {
	var c Compactor
	for _, ev := range evs {
		c.Commands++
		if ev.Compacted {
			c.Compacted++
		}
		c.RawBytes += ev.RawBytes
		if ev.RawBytes > ev.EmittedBytes {
			c.SavedBytes += ev.RawBytes - ev.EmittedBytes
		}
	}
	c.Tokens = c.SavedBytes / execwrap.BytesPerToken
	return c
}

func displayOf(n Numbers) Display {
	return Display{
		TokensSaved:      telemetry.TokensSavedEstimateLine(n.TokensSavedEstimate) + " (" + n.Scope.Label() + ": routing + compactor)",
		TokensSavedShort: telemetry.ShortTokens(n.TokensSavedEstimate),
		ScopeLabel:       n.Scope.Label(),
		Routing:          n.Routing.Line(),
		Compactor:        fmt.Sprintf("Commands: %d · Compacted: %d", n.Compactor.Commands, n.Compactor.Compacted),
		Ledger:           LedgerLine(n.Ledger, n.Scope),
	}
}
