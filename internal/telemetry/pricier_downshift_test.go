// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// The embedded Codex catalog prices its only mid model (gpt-5.6-terra, 2/12 USD
// per 1M) above the frontier models a session usually runs (gpt-6-sol and
// gpt-6.1-sol, 2/10). A frontier -> mid "downshift" is then a priced overspend.
// These tests pin what each savings path reports for it: no path may show a
// positive saving, and the real-cost path must show the overspend.

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestPricierDownshift_NoPathReportsASaving(t *testing.T) {
	cat := catalog.Load()
	mid := cat.ModelFor("codex", core.TierMid)
	for _, currentID := range []string{"gpt-6-sol", "gpt-6.1-sol"} {
		t.Run(currentID, func(t *testing.T) {
			current, ok := cat.LookupByID("codex", currentID)
			if !ok || current.Tier != core.TierFrontier {
				t.Fatalf("%s is not a codex frontier model in the catalog", currentID)
			}
			if mid.InputM+mid.OutputM <= current.InputM+current.OutputM {
				t.Skipf("catalog changed: %s is no longer pricier than %s", mid.ID, currentID)
			}
			if got := cat.SavingsRatio(current, mid); got != 0 {
				t.Errorf("SavingsRatio(%s, %s) = %v, want 0 for a pricier target", currentID, mid.ID, got)
			}

			d := core.Route("implement the CSV export feature", "codex", currentID, cat)
			if d.Tier != core.TierMid || d.Model.ID != mid.ID {
				t.Fatalf("route = tier %v model %s, want the mid tier on %s", d.Tier, d.Model.ID, mid.ID)
			}
			// Current behaviour: the tier comparison calls it a downshift, with no saving.
			if d.Verdict != core.VerdictDownshift || d.Savings != 0 {
				t.Errorf("verdict %v savings %v, want DOWNSHIFT with 0 savings", d.Verdict, d.Savings)
			}

			decision := FromDecision(d, "0123456789abcdef0123456789abcdef", "test")
			decision.Harness, decision.SessionID = "codex", "s"
			decision.QuotaStatus = QuotaAvailable
			usage := Event{
				Harness: "codex", SessionID: "s", Outcome: OutcomeUsage,
				LinkedDecision: DecisionKey(decision),
				InputTokens:    pricierTokens(100_000), OutputTokens: pricierTokens(20_000),
			}
			FillRealCost(&usage, current, mid)

			stats := Aggregate([]Event{decision, usage})
			if saved, frac := stats.NormSaved(); saved > 0 || frac > 0 {
				t.Errorf("normalised proxy reports a saving %v (%v) for a pricier target", saved, frac)
			}
			sum := stats.Summary()
			if sum.GrossSavedUSD != 0 || sum.NetSavedUSD >= 0 || sum.OverspendUSD <= 0 {
				t.Errorf("summary = %+v, want no gross saving, a negative net and a positive overspend", sum)
			}
		})
	}
}

func pricierTokens(v int64) *int64 { return &v }
