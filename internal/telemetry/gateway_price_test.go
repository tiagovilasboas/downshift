// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

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

func TestGatewayRun_WithoutSnapshotIsUnknownAndNotSaved(t *testing.T) {
	// No price source is read: a gateway run is unknown and never credited.
	decision, usage, _ := gatewayFixture(t, "vendor/other-model", GatewayPriceUnknown)
	s := TokenSavings([]Event{decision, usage}).Total
	if s.SavedChildren != 0 || s.WorkSaved != 0 {
		t.Fatalf("an unpriced gateway run was credited as saved: %+v", s)
	}
	if s.GatewayUnattributed != 1 {
		t.Fatalf("unattributed gateway runs %d, want 1", s.GatewayUnattributed)
	}
	if line := s.Line(); !strings.Contains(line, "1 gateway run unattributed: no tier for this gateway id") {
		t.Fatalf("savings line does not count the unattributed run: %q", line)
	}
}

func TestGatewayRun_AllowlistedIdIsNotCalledUnpriced(t *testing.T) {
	withAllowlist(t)
	requested := catalog.Load().ModelFor(rankHarness, core.TierMid).ID
	decision, usage := gatewayEvents(requested, allowlistSmallID)
	s := TokenSavings([]Event{decision, usage}).Total
	if s.SavedChildren != 1 || s.GatewayUnattributed != 0 {
		t.Fatalf("saved %d, unattributed %d; want 1 and 0", s.SavedChildren, s.GatewayUnattributed)
	}
	if line := s.Line(); strings.Contains(line, "snapshot") || strings.Contains(line, "unattributed") {
		t.Fatalf("allowlisted gateway run described by price or as unattributed: %q", line)
	}
}

func TestGatewayRun_UnknownIdIsNoTier(t *testing.T) {
	withAllowlist(t)
	requested := catalog.Load().ModelFor(rankHarness, core.TierMid).ID
	decision, usage := gatewayEvents(requested, "vendor/unlisted-model")
	s := TokenSavings([]Event{decision, usage}).Total
	if s.SavedChildren != 0 || s.GatewayUnattributed != 1 {
		t.Fatalf("saved %d, unattributed %d; want 0 and 1", s.SavedChildren, s.GatewayUnattributed)
	}
	if line := s.Line(); !strings.Contains(line, "1 gateway run unattributed: no tier for this gateway id") {
		t.Fatalf("unknown gateway id not reported as no tier: %q", line)
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
			if s.GatewayUnattributed != tc.wantUnpric {
				t.Fatalf("gateway unattributed %d, want %d", s.GatewayUnattributed, tc.wantUnpric)
			}
		})
	}
}
