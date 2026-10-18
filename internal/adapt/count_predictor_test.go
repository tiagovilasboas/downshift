// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import "testing"

func ev(shape string, t Tier, ok bool) FeedbackEvent {
	return FeedbackEvent{Shape: shape, SelectedTier: t, Success: ok, Failed: !ok}
}

func TestCountPredictor_MonotoneAndEvidenceWeighted(t *testing.T) {
	var events []FeedbackEvent
	for i := 0; i < 20; i++ {
		events = append(events, ev("a", TierSmall, true))
	}
	events = append(events, ev("b", TierSmall, true)) // one success only
	p := NewCountPredictor(events, 2, 0.75)

	pa, pb := p.Probabilities("a"), p.Probabilities("b")
	if pa[TierSmall] <= pb[TierSmall] {
		t.Errorf("20 successes (%.3f) must outrank 1 (%.3f)", pa[TierSmall], pb[TierSmall])
	}
	for _, pr := range [][3]float64{pa, pb, p.Probabilities("unseen")} {
		if pr[0] > pr[1] || pr[1] > pr[2] {
			t.Errorf("not monotone: %v", pr)
		}
	}
	if got := p.RequiredTier("a"); got != TierSmall {
		t.Errorf("well-evidenced shape: tier = %v, want small", got)
	}
}

func TestCountPredictor_OneMissDoesNotBlockForever(t *testing.T) {
	var events []FeedbackEvent
	for i := 0; i < 30; i++ {
		events = append(events, ev("a", TierSmall, true))
	}
	events = append(events, ev("a", TierSmall, false))
	if got := NewCountPredictor(events, 2, 0.75).RequiredTier("a"); got != TierSmall {
		t.Errorf("30 hits and 1 miss: tier = %v, want small", got)
	}
}
