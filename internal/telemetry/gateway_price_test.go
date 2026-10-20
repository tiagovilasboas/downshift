// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

const gatewaySlug = "vendor/cheap-model"

// gatewayFixture returns an applied rewrite from a frontier request and the
// usage of its child, which ran on ran and carries price (GatewayPrice).
func gatewayFixture(t *testing.T, ran, price string) (decision, usage Event, requested string) {
	t.Helper()
	requested = catalog.Load().ModelFor("claude-code", core.TierFrontier).ID
	in, out := int64(1000), int64(500)
	decision = Event{CorrelationID: "d1", Harness: "claude-code", SessionID: "s1", Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", FromModel: requested, ToModel: catalog.Load().ModelFor("claude-code", core.TierSmall).ID}
	usage = Event{CorrelationID: "u1", Harness: "claude-code", SessionID: "s1", Outcome: OutcomeUsage, LinkedDecision: "d1",
		FromModel: requested, ToModel: ran, GatewayPrice: price, InputTokens: &in, OutputTokens: &out}
	return decision, usage, requested
}

// snapshotAt prices every gateway model at a multiple of the requested model's
// catalog price, so the test does not depend on the catalog's current numbers.
func snapshotAt(t *testing.T, requested string, factor float64) func(string) (float64, float64, bool) {
	t.Helper()
	req, ok := DirectionCatalog.LookupByID("claude-code", requested)
	if !ok || req.InputM <= 0 || req.OutputM <= 0 {
		t.Fatalf("requested model %q has no catalog price", requested)
	}
	return func(model string) (float64, float64, bool) {
		if model != gatewaySlug {
			return 0, 0, false
		}
		return req.InputM * factor, req.OutputM * factor, true
	}
}

func withGatewayPrices(t *testing.T, lookup func(string) (float64, float64, bool)) {
	t.Helper()
	prev := GatewayPriceLookup
	GatewayPriceLookup = lookup
	t.Cleanup(func() { GatewayPriceLookup = prev })
}

func TestGatewayRun_PricedFromSnapshot(t *testing.T) {
	cases := []struct {
		name       string
		factor     float64
		wantSaved  int
		wantPricer int64
	}{
		{name: "snapshot price below the requested one is saved", factor: 0.1, wantSaved: 1},
		{name: "snapshot price above the requested one is pricier", factor: 10, wantPricer: 1500},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requested := catalog.Load().ModelFor("claude-code", core.TierFrontier).ID
			withGatewayPrices(t, snapshotAt(t, requested, tc.factor))
			decision, usage, _ := gatewayFixture(t, gatewaySlug, GatewayPriceStatus(gatewaySlug))
			s := TokenSavings([]Event{decision, usage}).Total
			if s.SavedChildren != tc.wantSaved {
				t.Fatalf("saved children %d, want %d", s.SavedChildren, tc.wantSaved)
			}
			if s.WorkPricier != tc.wantPricer {
				t.Fatalf("work pricier %d, want %d", s.WorkPricier, tc.wantPricer)
			}
			if s.GatewayUnpriced != 0 {
				t.Fatalf("a priced gateway run counted as unpriced: %d", s.GatewayUnpriced)
			}
		})
	}
}

func TestGatewayRun_WithoutSnapshotIsUnknownAndNotSaved(t *testing.T) {
	requested := catalog.Load().ModelFor("claude-code", core.TierFrontier).ID
	// No snapshot price for the ran slug: a cheap-looking tier must not count.
	withGatewayPrices(t, snapshotAt(t, requested, 0.1))
	decision, usage, _ := gatewayFixture(t, "vendor/other-model", GatewayPriceUnknown)
	s := TokenSavings([]Event{decision, usage}).Total
	if s.SavedChildren != 0 || s.WorkSaved != 0 {
		t.Fatalf("an unpriced gateway run was credited as saved: %+v", s)
	}
	if s.GatewayUnpriced != 1 {
		t.Fatalf("unpriced gateway runs %d, want 1", s.GatewayUnpriced)
	}
	if line := s.Line(); !strings.Contains(line, "1 gateway run without a snapshot price (not counted as saved)") {
		t.Fatalf("savings line does not count the unpriced run: %q", line)
	}
}

func TestGatewayRun_SnapshotGoneAtReadIsUnknown(t *testing.T) {
	// Recorded as priced, but the snapshot no longer lists the model.
	withGatewayPrices(t, func(string) (float64, float64, bool) { return 0, 0, false })
	decision, usage, _ := gatewayFixture(t, gatewaySlug, GatewayPriceSnapshot)
	s := TokenSavings([]Event{decision, usage}).Total
	if s.SavedChildren != 0 || s.GatewayUnpriced != 1 {
		t.Fatalf("saved %d, unpriced %d; want 0 and 1", s.SavedChildren, s.GatewayUnpriced)
	}
}

func TestGatewayPriceStatus(t *testing.T) {
	withGatewayPrices(t, nil)
	if got := GatewayPriceStatus(gatewaySlug); got != GatewayPriceUnknown {
		t.Fatalf("no lookup: %q, want unknown", got)
	}
	withGatewayPrices(t, func(string) (float64, float64, bool) { return 1, 2, true })
	if got := GatewayPriceStatus(gatewaySlug); got != GatewayPriceSnapshot {
		t.Fatalf("priced: %q, want snapshot", got)
	}
}

func TestIsGatewayModel(t *testing.T) {
	for id, want := range map[string]bool{
		"vendor/model":     true,
		"claude-haiku-4-5": false,
		"/model":           false,
		"vendor/":          false,
		"a/b/c":            false,
		"unknown":          false,
		"":                 false,
	} {
		if got := IsGatewayModel(id); got != want {
			t.Fatalf("IsGatewayModel(%q) = %v, want %v", id, got, want)
		}
	}
}

// A native run is priced by tier exactly as before the gateway rule existed:
// the table pins the buckets and the unpriced count a native child produces.
func TestNativeRun_UnchangedByGatewayRule(t *testing.T) {
	cat := catalog.Load()
	requested := cat.ModelFor("claude-code", core.TierFrontier).ID
	small := cat.ModelFor("claude-code", core.TierSmall).ID
	withGatewayPrices(t, func(string) (float64, float64, bool) { return 0, 0, false })
	cases := []struct {
		name       string
		outcome    string
		ran        string
		wantSaved  int
		wantUnpric int
	}{
		{name: "applied rewrite onto a cheaper tier", outcome: OutcomeRewriteEmitted, ran: small, wantSaved: 1},
		{name: "allowed spawn stays on the requested model", outcome: OutcomeAllow, ran: requested},
		{name: "applied rewrite the harness did not honor", outcome: OutcomeRewriteEmitted, ran: requested},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, usage, _ := gatewayFixture(t, tc.ran, "")
			decision.Outcome = tc.outcome
			if tc.outcome != OutcomeRewriteEmitted {
				decision.ToModel = requested
			}
			s := TokenSavings([]Event{decision, usage}).Total
			if s.SavedChildren != tc.wantSaved {
				t.Fatalf("saved children %d, want %d", s.SavedChildren, tc.wantSaved)
			}
			if s.GatewayUnpriced != tc.wantUnpric {
				t.Fatalf("gateway unpriced %d, want %d", s.GatewayUnpriced, tc.wantUnpric)
			}
		})
	}
}
