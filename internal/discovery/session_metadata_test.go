// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery_test

// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/discovery"
	"reflect"
	"testing"
)

func TestSessionMetadataAcceptance_OpaqueNativeIDsWithoutCatalog(t *testing.T) {
	for _, h := range core.AllHarnessCaps() {
		// Lexical names and picker order intentionally disagree with capability.
		models := []discovery.Model{{ID: "aaa", Tier: "frontier"}, {ID: "zzz", Tier: "small"}, {ID: "mmm", Tier: "mid", Efforts: []string{"low", "high"}}, {ID: "unknown"}}
		ranked := discovery.Order(h.Harness, models, nil)
		if !reflect.DeepEqual(ranked.IDs, []string{"zzz", "mmm", "aaa"}) {
			t.Errorf("%s native tiers not honored: %+v", h.Harness, ranked)
		}
		if !reflect.DeepEqual(ranked.Unranked, []string{"unknown"}) {
			t.Errorf("%s unknown capability guessed: %+v", h.Harness, ranked)
		}
		if len(ranked.Metadata) != 4 || !reflect.DeepEqual(ranked.Metadata[2].Efforts, []string{"low", "high"}) {
			t.Errorf("%s native metadata discarded: %+v", h.Harness, ranked.Metadata)
		}
		for _, m := range ranked.Metadata {
			if m.InputM != nil || m.OutputM != nil {
				t.Errorf("%s price fabricated: %+v", h.Harness, m)
			}
		}
	}
}

func TestSessionMetadataAcceptance_NativePriorityAndInvalidTier(t *testing.T) {
	cat := catalog.Load()
	for _, h := range core.AllHarnessCaps() {
		id := cat.ModelFor(h.Harness, core.TierFrontier).ID
		small := cat.ModelFor(h.Harness, core.TierSmall).ID
		ranked := discovery.Order(h.Harness, []discovery.Model{{ID: id, Tier: "small"}, {ID: small, Tier: "frontier"}}, cat)
		if !reflect.DeepEqual(ranked.IDs, []string{id, small}) {
			t.Errorf("%s catalog beat native tier: %+v", h.Harness, ranked)
		}
		invalid := discovery.Order(h.Harness, []discovery.Model{{ID: id, Tier: "super-ultra"}}, cat)
		if len(invalid.IDs) != 0 || !reflect.DeepEqual(invalid.Unranked, []string{id}) {
			t.Errorf("%s malformed explicit tier reopened catalog: %+v", h.Harness, invalid)
		}
	}
}
