// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"math"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Dollars are a valuation of the recorded tokens at report time: a corrected
// catalog price reaches old records, while which records are priced stays what
// the recorder decided.
func TestRepriceUsage(t *testing.T) {
	prices := map[string]core.Model{
		"big":   {InputM: 4, OutputM: 20},
		"small": {InputM: 1, OutputM: 5},
		"free":  {}, // in the catalog, no price
	}
	price := func(_, id string) (core.Model, bool) { m, ok := prices[id]; return m, ok }
	in, out := int64(1_000_000), int64(100_000)
	rec := func(from, to string, actual, baseline *float64) Event {
		return Event{Harness: "h", Outcome: OutcomeUsage, FromModel: from, ToModel: to,
			InputTokens: &in, OutputTokens: &out, ActualCostUSD: actual, BaselineCostUSD: baseline}
	}
	for _, tc := range []struct {
		name               string
		ev                 Event
		wantActual, wantBL *float64
	}{
		{"stale prices are recomputed", rec("big", "small", f64(9), f64(99)), f64(1.5), f64(6)},
		{"no baseline stays no baseline", rec("unknown", "small", f64(9), nil), f64(1.5), nil},
		{"an unpriced record stays unpriced", rec("big", "small", nil, nil), nil, nil},
		{"a model the catalog stopped pricing loses its dollars", rec("big", "free", f64(1), f64(2)), nil, nil},
		{"an unknown ran model keeps what was recorded", rec("big", "unknown", f64(1), f64(2)), f64(1), f64(2)},
		{"a requested model gone from the catalog drops the baseline", rec("gone", "small", f64(1), f64(2)), f64(1.5), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orig := tc.ev
			got := RepriceUsage([]Event{tc.ev}, price)[0]
			same := func(a, b *float64) bool {
				return a == nil && b == nil || a != nil && b != nil && math.Abs(*a-*b) < 1e-9
			}
			if !same(got.ActualCostUSD, tc.wantActual) || !same(got.BaselineCostUSD, tc.wantBL) {
				t.Errorf("actual %v baseline %v, want %v / %v", ptrOr(got.ActualCostUSD), ptrOr(got.BaselineCostUSD), ptrOr(tc.wantActual), ptrOr(tc.wantBL))
			}
			if !same(orig.ActualCostUSD, tc.ev.ActualCostUSD) {
				t.Error("the input events must not be modified")
			}
		})
	}
	decision := Event{Outcome: OutcomeRewriteEmitted, ActualCostUSD: f64(3)}
	if got := RepriceUsage([]Event{decision}, price)[0]; *got.ActualCostUSD != 3 {
		t.Error("only usage records are repriced")
	}
	if evs := []Event{decision}; &RepriceUsage(evs, nil)[0] != &evs[0] {
		t.Error("a nil price func returns the events unchanged")
	}
}

func ptrOr(p *float64) any {
	if p == nil {
		return "nil"
	}
	return *p
}
