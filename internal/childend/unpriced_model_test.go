// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package childend

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

type unpricedResolver struct{ *catalog.Catalog }

func (r unpricedResolver) LookupByID(harness, id string) (core.Model, bool) {
	if id == "session-model" {
		return core.Model{ID: id, Harness: harness, Tier: core.TierFrontier}, true
	}
	return r.Catalog.LookupByID(harness, id)
}

// A usage record carries tokens and model ids only: no dollar figure is
// written, and a model the catalog cannot resolve gets no baseline source, so
// no tier comparison is claimed for it.
func TestBuildUsageEvent_TokensOnlyNoDollars(t *testing.T) {
	r := unpricedResolver{catalog.Load()}
	for _, tc := range []struct {
		name, from, to string
		baselineSource bool
	}{
		{"known models", "claude-opus-5-5", "claude-haiku-5-5", true},
		{"session model (tier only)", "claude-opus-5-5", "session-model", true},
		{"unknown requested", "unknown", "claude-haiku-5-5", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := &telemetry.Event{Harness: "claude-code", FromModel: tc.from, ToModel: tc.to}
			ev := buildUsageEvent("claude-code", telemetry.TokenUsage{InputTokens: 1000, OutputTokens: 500}, tc.to, "s", "test", target, r, time.Now())
			if ev.ActualCostUSD != nil || ev.BaselineCostUSD != nil || ev.HasRealCost() {
				t.Fatalf("a dollar figure was written: %#v", ev)
			}
			if !ev.HasTokenUsage() {
				t.Fatalf("tokens not recorded: %#v", ev)
			}
			if got := ev.BaselineSource != ""; got != tc.baselineSource {
				t.Fatalf("baseline source %q, want set=%v", ev.BaselineSource, tc.baselineSource)
			}
		})
	}
}
