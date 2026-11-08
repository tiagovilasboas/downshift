// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "testing"

// A rewrite the harness refuted can have run on a third model: another hook on
// the same call set it and won (docs/guides/companions.md, hook coexistence).
// Every view must name the model the resolved record reports, fall back to the
// requested model only when the record names none, and count the rewrite as
// refuted, not unobserved.
func TestHonesty_RefutedRewriteNamesTheModelThatRan(t *testing.T) {
	decision := Event{CorrelationID: "d1", Timestamp: "2026-11-08T10:00:00Z", Harness: "claude-code", SessionID: hvSess,
		Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", Complexity: "SIMPLE",
		FromModel: "claude-opus-5-5", ToModel: "claude-haiku-5-5", QuotaStatus: QuotaAvailable}
	resolved := func(served string, honored bool) Event {
		return Event{CorrelationID: "r1", Timestamp: "2026-11-08T10:01:00Z", Harness: "claude-code", SessionID: hvSess,
			Outcome: OutcomeResolved, LinkedDecision: "d1", FromModel: "claude-haiku-5-5", ToModel: served,
			RewriteHonored: hvBool(honored)}
	}
	cases := []struct {
		name                          string
		events                        []Event
		wantModel                     string // the one entry in ModelMix and the statusline top model
		observed, refuted, unobserved int
	}{
		{"another hook set a third model", []Event{decision, resolved("claude-sonnet-5-5", false)}, "sonnet", 0, 1, 0},
		{"refuted, record names no model", []Event{decision, resolved("", false)}, "opus", 0, 1, 0},
		{"refuted, harness kept the requested model", []Event{decision, resolved("claude-opus-5-5", false)}, "opus", 0, 1, 0},
		{"honored", []Event{decision, resolved("claude-haiku-5-5", true)}, "haiku", 1, 0, 0},
		{"no resolved record", []Event{decision}, "haiku", 0, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := Aggregate(c.events)
			if len(s.ModelMix) != 1 || s.ModelMix[c.wantModel] != 1 {
				t.Errorf("ModelMix = %v, want only %s 1", s.ModelMix, c.wantModel)
			}
			if s.HonorObserved != c.observed || s.HonorRefuted != c.refuted || s.HonorUnobserved != c.unobserved {
				t.Errorf("honor observed/refuted/unobserved = %d/%d/%d, want %d/%d/%d",
					s.HonorObserved, s.HonorRefuted, s.HonorUnobserved, c.observed, c.refuted, c.unobserved)
			}
			if top := SessionSummary(c.events, hvSess).TopModel; top != c.wantModel {
				t.Errorf("statusline TopModel = %q, want %q", top, c.wantModel)
			}
		})
	}
}
