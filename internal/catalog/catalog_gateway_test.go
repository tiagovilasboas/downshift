// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestLookupByID_GatewaySlugs(t *testing.T) {
	// 9router is the recognized gateway prefix
	for _, gw := range []string{"9router"} {
		id := gw + "/anthropic/claude-haiku-5"
		slug, isGw := gatewaySlug(id)
		if !isGw || slug != "anthropic/claude-haiku-5" {
			t.Errorf("gatewaySlug(%q) = %q, %v, want slug 'anthropic/claude-haiku-5', true", id, slug, isGw)
		}
	}
}

// --- Provider-prefixed model ids from a local gateway ---
//
// A self-hosted gateway exposes provider-prefixed ids. Anthropic-family ids
// behind any prefix resolve by the OpenRouter rule (first provider/ segment
// stripped). Non-Anthropic and combo ids stay unknown: Downshift then has no
// baseline and leaves the spawn unchanged (fail open).

func TestLookupByID_GatewayPrefixedAnthropicID(t *testing.T) {
	c := Load()
	m, ok := c.LookupByID("claude-code", "cc/claude-opus-4-8")
	if !ok {
		t.Fatal("cc/claude-opus-4-8 should match the claude-opus family after normalisation")
	}
	if m.Tier != core.TierFrontier {
		t.Errorf("tier = %s, want frontier", m.Tier)
	}
}

func TestLookupByID_GatewayForeignAndComboStayUnknown(t *testing.T) {
	c := Load()
	for _, id := range []string{"glm/glm-5.2", "kmc/kimi-k2.6", "claude/combo/custo-otimizado", "claude/glm/glm-5.2"} {
		if _, ok := c.LookupByID("claude-code", id); ok {
			t.Errorf("%s must stay unknown so the router fails open", id)
		}
	}
}

// --- OpenRouter gateway rows keep their full "vendor/model" slug ---

const gatewayCatalog = `{
	"entries": [
		{"id":"deepseek-v4.1-flash-free","harness":"opencode","provider":"opencode","tier":"small"},
		{"id":"grok-x","harness":"opencode","provider":"opencode","tier":"small"},
		{"id":"deepseek/deepseek-v4.1-flash","family":"deepseek/deepseek-v4","harness":"opencode","provider":"openrouter","tier":"mid"},
		{"id":"x-ai/grok-x","harness":"opencode","provider":"openrouter","tier":"frontier"}
	]
}`

func TestLookupByID_GatewayRowWinsByFullSlug(t *testing.T) {
	c, err := parse([]byte(gatewayCatalog))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Exact full slug: the gateway row, not the native row sharing its tail.
	m, ok := c.LookupByID("opencode", "x-ai/grok-x")
	if !ok || m.ID != "x-ai/grok-x" || m.Tier != core.TierFrontier {
		t.Errorf("x-ai/grok-x = %+v ok=%v, want the gateway row (frontier)", m, ok)
	}
	// The native id itself still resolves to the native row.
	m, ok = c.LookupByID("opencode", "grok-x")
	if !ok || m.ID != "grok-x" || m.Tier != core.TierSmall {
		t.Errorf("grok-x = %+v ok=%v, want the native row (small)", m, ok)
	}
}

func TestLookupByID_GatewayFamilyMatchesTheFullSlug(t *testing.T) {
	c, err := parse([]byte(gatewayCatalog))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m, ok := c.LookupByID("opencode", "deepseek/deepseek-v4.2-flash")
	if !ok || m.ID != "deepseek/deepseek-v4.1-flash" || m.Tier != core.TierMid {
		t.Errorf("version bump = %+v ok=%v, want the gateway family row (mid)", m, ok)
	}
	e, ok := c.EntryFor("opencode", "deepseek/deepseek-v4.2-flash")
	if !ok || e.ID != "deepseek/deepseek-v4.1-flash" {
		t.Errorf("EntryFor = %+v ok=%v, want the same row as LookupByID", e, ok)
	}
}

func TestLookupByID_GatewayRowIsNotReachedByItsTail(t *testing.T) {
	c, err := parse([]byte(`{"entries":[
		{"id":"deepseek/deepseek-v4.1-flash","family":"deepseek/deepseek-v4","harness":"opencode","provider":"openrouter","tier":"mid"}
	]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, id := range []string{"deepseek-v4.1-flash", "deepseek-v4.2-flash", "other/deepseek-v4.1-flash"} {
		if m, ok := c.LookupByID("opencode", id); ok {
			t.Errorf("%q resolved to %+v: a gateway row is reachable only by its full slug", id, m)
		}
	}
}

func TestLookupByID_NativeStripUnchangedWithoutGatewayRows(t *testing.T) {
	c, err := parse([]byte(`{"entries":[
		{"id":"grok-x","harness":"opencode","provider":"opencode","tier":"small"}
	]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m, ok := c.LookupByID("opencode", "x-ai/grok-x")
	if !ok || m.ID != "grok-x" {
		t.Errorf("without gateway rows the prefix strip must still apply, got %+v ok=%v", m, ok)
	}
}

func TestParse_GatewayRowNeedsItsFullSlug(t *testing.T) {
	for name, body := range map[string]string{
		"id without a slash":     `{"entries":[{"id":"grok-x","harness":"opencode","provider":"openrouter","tier":"small"}]}`,
		"family without a slash": `{"entries":[{"id":"x-ai/grok-x","family":"grok","harness":"opencode","provider":"openrouter","tier":"small"}]}`,
	} {
		if _, err := parse([]byte(body)); err == nil {
			t.Errorf("%s: want a validation error", name)
		}
	}
}

// --- openrouter/<vendor>/<model>: the OpenCode form of a gateway model ---

func TestGatewayForm_ResolvesOnlyGatewayRows(t *testing.T) {
	c, err := parse([]byte(gatewayCatalog))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cases := []struct {
		name   string
		id     string
		wantID string // "" = unknown
		exact  bool
	}{
		{"gateway form resolves to the gateway row", "openrouter/x-ai/grok-x", "x-ai/grok-x", true},
		{"bare slug still resolves to the gateway row", "x-ai/grok-x", "x-ai/grok-x", true},
		{"native id is unchanged", "grok-x", "grok-x", true},
		{"gateway family matches the slug after the prefix", "openrouter/deepseek/deepseek-v4.2-flash", "deepseek/deepseek-v4.1-flash", false},
		{"bare prefix is unknown", "openrouter/", "", false},
		{"native tail is never reached through the gateway form", "openrouter/grok-x", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, ok := c.LookupByID("opencode", tc.id)
			if ok != (tc.wantID != "") || (ok && m.ID != tc.wantID) {
				t.Errorf("LookupByID(%q) = %q ok=%v, want %q", tc.id, m.ID, ok, tc.wantID)
			}
			e, ok := c.EntryFor("opencode", tc.id)
			if ok != (tc.wantID != "") || (ok && e.ID != tc.wantID) {
				t.Errorf("EntryFor(%q) = %q ok=%v, want %q", tc.id, e.ID, ok, tc.wantID)
			}
			if got := c.IsExactID("opencode", tc.id); got != tc.exact {
				t.Errorf("IsExactID(%q) = %v, want %v", tc.id, got, tc.exact)
			}
		})
	}
}

func TestGatewayForm_AllowedEffortsIsExactAndGatewayOnly(t *testing.T) {
	c, err := parse([]byte(`{"entries":[
		{"id":"grok-x","harness":"opencode","provider":"opencode","tier":"small","efforts":["low"]},
		{"id":"x-ai/grok-x","harness":"opencode","provider":"openrouter","tier":"frontier","efforts":["high"]}
	]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := c.AllowedEfforts("opencode", "openrouter/x-ai/grok-x"); len(got) != 1 || got[0] != "high" {
		t.Errorf("gateway form efforts = %v, want [high]", got)
	}
	if got := c.AllowedEfforts("opencode", "openrouter/grok-x"); got != nil {
		t.Errorf("native tail via gateway form efforts = %v, want nil", got)
	}
	if got := c.AllowedEfforts("opencode", "openrouter/"); got != nil {
		t.Errorf("bare prefix efforts = %v, want nil", got)
	}
}
