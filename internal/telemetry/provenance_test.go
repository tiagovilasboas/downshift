// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestReasonCodes_SemanticBoostIsRecorded(t *testing.T) {
	d := core.Decision{Harness: "claude-code", Tier: core.TierMid, Guards: core.Guards{SemanticLift: true}}
	if !hasCode(telemetry.ReasonCodes(d), telemetry.ReasonSemanticBoost) {
		t.Fatalf("a semantic lift must leave SEMANTIC_BOOST in the trace: %v", telemetry.ReasonCodes(d))
	}
	d.Guards.SemanticLift = false
	if hasCode(telemetry.ReasonCodes(d), telemetry.ReasonSemanticBoost) {
		t.Fatal("no lift must not record SEMANTIC_BOOST")
	}
}

func hasCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

// A frontier decision on an unconfident classification with no deterministic
// lift is the silent-overspend shape: it must be counted and warned about.
func TestAggregate_FrontierOnDoubt(t *testing.T) {
	ev := func(codes ...string) telemetry.Event {
		return telemetry.Event{Harness: "claude-code", Outcome: "rewrite_emitted", Verdict: "UNKNOWN", Tier: "FRONTIER", Complexity: "COMPLEX", ReasonCodes: codes}
	}
	cases := []struct {
		name     string
		events   []telemetry.Event
		frontier int
		onDoubt  int
		wantWarn bool
	}{
		{"unconfident, no lift", []telemetry.Event{ev("CLASSIFIER_UNCONFIDENT", "SEMANTIC_BOOST")}, 1, 1, true},
		{"confident regex", []telemetry.Event{ev("CLASSIFIER_CONFIDENT")}, 1, 0, false},
		{"risk floor lift", []telemetry.Event{ev("CLASSIFIER_UNCONFIDENT", "RISK_FLOOR")}, 1, 0, false},
		{"nb upshift lift", []telemetry.Event{ev("CLASSIFIER_UNCONFIDENT", "NB_UPSHIFT_APPLIED")}, 1, 0, false},
		{"tier floor lift", []telemetry.Event{ev("CLASSIFIER_UNCONFIDENT", "TIER_FLOOR")}, 1, 0, false},
		{"mixed", []telemetry.Event{ev("CLASSIFIER_CONFIDENT"), ev("CLASSIFIER_UNCONFIDENT")}, 2, 1, true},
		{"held by a guardrail is not counted", []telemetry.Event{{Harness: "claude-code", Outcome: "allow", Verdict: "UPSHIFT", Tier: "frontier", Complexity: "COMPLEX", Corrections: []string{"R8_UNCONFIDENT_UPSHIFT"}, ReasonCodes: []string{"CLASSIFIER_UNCONFIDENT", "HELD_UNCONFIDENT_UPSHIFT"}}}, 1, 0, false},
		{"unchanged spawn is not counted", []telemetry.Event{{Harness: "claude-code", Outcome: "allow", Verdict: "OK", Tier: "frontier", Complexity: "COMPLEX", ReasonCodes: []string{"CLASSIFIER_UNCONFIDENT"}}}, 1, 0, false},
		{"mid tier is not counted", []telemetry.Event{{Harness: "claude-code", Outcome: "allow", Tier: "MID", Complexity: "MEDIUM", ReasonCodes: []string{"CLASSIFIER_UNCONFIDENT"}}}, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := telemetry.Aggregate(tc.events)
			if s.FrontierDecisions != tc.frontier || s.FrontierOnDoubt != tc.onDoubt {
				t.Fatalf("frontier=%d onDoubt=%d, want %d/%d", s.FrontierDecisions, s.FrontierOnDoubt, tc.frontier, tc.onDoubt)
			}
			var out bytes.Buffer
			telemetry.PrintStats(tc.events, telemetry.StatsOptions{Detail: true}, &out)
			if got := strings.Contains(out.String(), "WARNING"); got != tc.wantWarn {
				t.Fatalf("warning=%v want %v\n%s", got, tc.wantWarn, out.String())
			}
		})
	}
}

// The flag must agree with the classifier: set exactly when fusion raised the
// complexity, and never for a prompt the regex rules did not recognise.
func TestSemanticLift_MatchesTheClassifier(t *testing.T) {
	prompts := []string{
		"hook", "reply with a short summary", "explain what this regex matches",
		"Split the config module into load and validate files",
		"write a unit test for ParseDuration", "rename the userId variable",
		"Atualize a descrição do botão de login", "fix a typo in the password reset email template",
		"redesign the authentication architecture across all services",
	}
	for _, p := range prompts {
		base := core.Classify(p)
		got := core.ClassifyWithSemantic(p)
		if got.SemanticLift && got.Complexity <= base.Complexity {
			t.Errorf("%q: SemanticLift set but complexity did not rise (base=%s got=%s)", p, base.Complexity, got.Complexity)
		}
		if base.NoSignal && got.SemanticLift {
			t.Errorf("%q: a no-signal prompt must never be semantically lifted", p)
		}
		if got.SemanticLift && got.Complexity == core.Complex && base.Scores[core.Complex] == 0 {
			t.Errorf("%q: semantic produced COMPLEX without regex evidence", p)
		}
	}
}
