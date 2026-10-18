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

// Session discovery can establish identity and tier without establishing list
// prices. Known identity must not fabricate free spend or a free baseline.
func TestBuildUsageEvent_UnpricedModelKeepsCostsUnknown(t *testing.T) {
	r := unpricedResolver{catalog.Load()}
	for _, tc := range []struct {
		name, from, to   string
		actual, baseline bool
	}{
		{"unknown actual", "claude-opus-5-5", "session-model", false, true},
		{"unknown baseline", "session-model", "claude-haiku-5-5", true, false},
		{"both unknown", "session-model", "session-model", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := &telemetry.Event{Harness: "claude-code", FromModel: tc.from, ToModel: tc.to}
			ev := buildUsageEvent("claude-code", telemetry.TokenUsage{InputTokens: 1000, OutputTokens: 500}, tc.to, "s", "test", target, r, time.Now())
			if (ev.ActualCostUSD != nil) != tc.actual || (ev.BaselineCostUSD != nil) != tc.baseline {
				t.Fatalf("actual=%v baseline=%v; want priced %v/%v", ev.ActualCostUSD, ev.BaselineCostUSD, tc.actual, tc.baseline)
			}
			if ev.HasRealCost() || ev.RealSavedUSD() != 0 {
				t.Fatalf("missing price fabricated comparative cost: %#v", ev)
			}
		})
	}
}
