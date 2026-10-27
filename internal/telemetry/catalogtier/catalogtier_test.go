// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalogtier_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/catalogtier"
)

var _ telemetry.ModelCatalog = catalogtier.View{}

// The adapter reports int(core.Tier); telemetry ranks on its own scale. The
// two must stay the same numbers.
func TestTierRanksMatchCoreTiers(t *testing.T) {
	for tier, rank := range map[core.Tier]int{
		core.TierSmall:    telemetry.TierRankSmall,
		core.TierMid:      telemetry.TierRankMid,
		core.TierFrontier: telemetry.TierRankFrontier,
	} {
		if int(tier) != rank {
			t.Errorf("core tier %s = %d, telemetry rank %d", tier, int(tier), rank)
		}
	}
	if core.TierFrontier.String() != telemetry.TierFrontier {
		t.Errorf("frontier name %q != %q", core.TierFrontier.String(), telemetry.TierFrontier)
	}
	if core.UsageNone != telemetry.UsageNone {
		t.Errorf("usage none %q != %q", core.UsageNone, telemetry.UsageNone)
	}
}

type shadowAndLive struct {
	core.Resolver
	shadow, live []core.Model
}

func (r shadowAndLive) ShadowGateways() []core.Model { return r.shadow }
func (r shadowAndLive) LiveGateways() []core.Model   { return r.live }

func TestViewLooksUpCatalogAndGatewayRows(t *testing.T) {
	cat := catalog.Load()
	small := cat.ModelFor("claude-code", core.TierSmall)
	v := catalogtier.Of(shadowAndLive{
		Resolver: cat,
		shadow:   []core.Model{{ID: "x/a", Tier: core.TierMid}},
		live:     []core.Model{{ID: "x/a", Tier: core.TierSmall}, {ID: "x/b", Tier: core.TierFrontier}},
	})
	if id, tier, ok := v.Lookup("claude-code", small.ID); !ok || id != small.ID || tier != telemetry.TierRankSmall {
		t.Fatalf("Lookup = %q %d %v", id, tier, ok)
	}
	if !catalogtier.Of(cat).IsExactID("claude-code", small.ID) {
		t.Fatalf("%s should be an exact id", small.ID)
	}
	if v.IsExactID("claude-code", small.ID) {
		t.Fatal("a resolver without core.ExactIDResolver knows no exact id")
	}
	if tier, ok := v.GatewayTier("x/a"); !ok || tier != telemetry.TierRankMid {
		t.Fatalf("shadow row must win: %d %v", tier, ok)
	}
	if tier, ok := v.GatewayTier("x/b"); !ok || tier != telemetry.TierRankFrontier {
		t.Fatalf("live row: %d %v", tier, ok)
	}
	if _, ok := v.GatewayTier("x/none"); ok {
		t.Fatal("unknown gateway id must not resolve")
	}
	if _, ok := catalogtier.Of(cat).GatewayTier("x/a"); ok {
		t.Fatal("a resolver without gateway rows knows no gateway tier")
	}
}
