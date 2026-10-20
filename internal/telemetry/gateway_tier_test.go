// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

const (
	allowlistSmallID = "vendor/cheap-model"
	allowlistMidID   = "vendor/mid-model"
)

// allowlistResolver is the catalog plus two allowlisted gateway rows, so the
// tests do not depend on the owner's openrouter.json.
type allowlistResolver struct {
	core.Resolver
	rows []core.Model
}

func (r allowlistResolver) ShadowGateways() []core.Model { return r.rows }

func withAllowlist(t *testing.T) {
	t.Helper()
	prev := DirectionCatalog
	DirectionCatalog = allowlistResolver{Resolver: catalog.Load(), rows: []core.Model{
		{ID: allowlistSmallID, Tier: core.TierSmall, Harness: "opencode"},
		{ID: allowlistMidID, Tier: core.TierMid, Harness: "opencode"},
	}}
	t.Cleanup(func() { DirectionCatalog = prev })
}

// gatewayEvents builds an applied rewrite and its priced usage record for a
// child that ran on ran. Native and gateway ids take the same shape.
func gatewayEvents(requested, ran string) (usage, decision Event) {
	in, out := int64(1000), int64(500)
	decision = Event{
		CorrelationID: "d", Harness: rankHarness, SessionID: "s",
		Outcome: OutcomeRewriteEmitted, FromModel: requested, ToModel: ran,
	}
	usage = Event{
		CorrelationID: "u", Harness: rankHarness, SessionID: "s", Outcome: OutcomeUsage,
		LinkedDecision: "d", FromModel: requested, ToModel: ran,
		BaselineSource: BaselineRequested, InputTokens: &in, OutputTokens: &out,
	}
	return usage, decision
}

func TestTokenDirection_GatewayRankedByAllowlistTier(t *testing.T) {
	withAllowlist(t)
	cat := catalog.Load()
	mid := cat.ModelFor(rankHarness, core.TierMid).ID
	small := cat.ModelFor(rankHarness, core.TierSmall).ID
	haiku := "claude-haiku-4-5"
	if m, ok := cat.LookupByID(rankHarness, haiku); !ok || m.Tier != core.TierSmall {
		t.Fatalf("%s must be a small-tier native model for this table", haiku)
	}
	cases := []struct {
		name      string
		requested string
		ran       string
		refuted   bool
		held      bool
		want      string
	}{
		{name: "small gateway row for a requested mid is saved", requested: mid, ran: allowlistSmallID, want: TokenCheaper},
		{name: "mid gateway row for a requested haiku is pricier", requested: haiku, ran: allowlistMidID, want: TokenPricier},
		{name: "same-tier gateway row is unchanged", requested: mid, ran: allowlistMidID, want: TokenUnchanged},
		{name: "unknown gateway id is unattributed", requested: mid, ran: "vendor/unlisted-model", want: TokenUnattributed},
		{name: "refuted gateway run is unchanged", requested: mid, ran: allowlistSmallID, refuted: true, want: TokenUnchanged},
		{name: "held gateway decision is unchanged", requested: mid, ran: allowlistSmallID, held: true, want: TokenUnchanged},
		{name: "native small for a requested mid is saved", requested: mid, ran: small, want: TokenCheaper},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			usage, decision := gatewayEvents(tc.requested, tc.ran)
			if tc.held {
				decision = withCorrections(decision)
			}
			if got := tokenDirection(usage, decision, true, false, tc.refuted); got != tc.want {
				t.Fatalf("tokenDirection = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGatewayRun_SmallRowSavedInTotals(t *testing.T) {
	withAllowlist(t)
	mid := catalog.Load().ModelFor(rankHarness, core.TierMid).ID
	usage, decision := gatewayEvents(mid, allowlistSmallID)
	s := TokenSavings([]Event{decision, usage}).Total
	if s.SavedChildren != 1 || s.GatewayUnattributed != 0 {
		t.Fatalf("saved %d, unattributed %d; want 1 and 0", s.SavedChildren, s.GatewayUnattributed)
	}
}
