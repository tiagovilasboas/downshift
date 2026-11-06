// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

// gatewayCat holds a native deepseek row (the homonym the vendor strip would
// reuse) and the allowlist rows given by the caller.
func gatewayCat(allow ...catalog.Entry) fakeCat {
	native := catalog.Entry{ID: "deepseek-v4.1-flash-free", Family: "deepseek-v4", Harness: "opencode", Tier: "small"}
	return fakeCat{entries: append([]catalog.Entry{native}, allow...)}
}

func TestOrderGatewayIDsWithoutLiveAllowlistRowAreNotSelectable(t *testing.T) {
	cat := gatewayCat(catalog.Entry{
		ID: "openrouter/known/live", Provider: catalog.AllowlistProvider, Harness: "opencode",
		Tier: "mid", Routing: "automatic",
	})
	r := Order("opencode", []Model{
		{ID: "evil/big-pickle", Gateway: true},
		{ID: "unlisted-vendor/deepseek-v4.1-flash-free", Gateway: true},
	}, cat)
	if len(r.IDs) != 0 {
		t.Fatalf("ordered = %v, unlisted gateway ids must not be selectable", r.IDs)
	}
	if want := []string{"evil/big-pickle", "unlisted-vendor/deepseek-v4.1-flash-free"}; !reflect.DeepEqual(r.Unranked, want) {
		t.Fatalf("unranked = %v, want %v", r.Unranked, want)
	}
}

func TestOrderGatewayIDRankedOnlyByExactLiveRow(t *testing.T) {
	cat := gatewayCat(catalog.Entry{
		ID: "openrouter/known/live", Provider: catalog.AllowlistProvider, Harness: "opencode",
		Tier: "mid", Routing: "automatic",
	})
	r := Order("opencode", []Model{{ID: "known/live", Gateway: true}}, cat)
	if want := []string{"known/live"}; !reflect.DeepEqual(r.IDs, want) {
		t.Fatalf("ordered = %v, want %v", r.IDs, want)
	}
}

func TestOrderGatewayIDStaysOutWhenRowIsShadowOrExplicit(t *testing.T) {
	for _, routing := range []string{catalog.RoutingShadow, "explicit_only"} {
		cat := gatewayCat(catalog.Entry{
			ID: "openrouter/known/row", Provider: catalog.AllowlistProvider, Harness: "opencode",
			Tier: "mid", Routing: routing,
		})
		r := Order("opencode", []Model{{ID: "known/row", Gateway: true}}, cat)
		if slices.Contains(r.IDs, "known/row") {
			t.Fatalf("routing %q: gateway id must not rank", routing)
		}
	}
}

func TestOrderGatewayIDNeverInheritsNativeHomonymTier(t *testing.T) {
	cat := gatewayCat()
	r := Order("opencode", []Model{{ID: "unlisted-vendor/deepseek-v4.1-flash-free", Gateway: true}}, cat)
	if slices.Contains(r.IDs, "unlisted-vendor/deepseek-v4.1-flash-free") {
		t.Fatal("vendor prefix must not be stripped for a gateway id")
	}
}

func TestOrderNativeIDStillUsesCatalogTier(t *testing.T) {
	cat := gatewayCat()
	r := Order("opencode", []Model{{ID: "deepseek-v4.1-flash-free"}}, cat)
	if want := []string{"deepseek-v4.1-flash-free"}; !reflect.DeepEqual(r.IDs, want) {
		t.Fatalf("native id ordered = %v, want %v (unchanged behaviour)", r.IDs, want)
	}
}
