// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"
	"time"
)

func TestReasonWords_EveryCodeHasWords(t *testing.T) {
	codes := []string{
		ReasonClassifierConfident, ReasonClassifierUnconfident, ReasonNoSignal, ReasonSemanticBoost, ReasonTaskFocus, ReasonUnsupportedLanguage,
		ReasonRiskCategory, ReasonRiskFloor, ReasonNBUpshiftApplied,
		ReasonNBDownshiftApplied, ReasonTierFloor, ReasonCreditHeld, ReasonGuessHeld, ReasonPaused, ReasonScoreRouted, ReasonBaselineFromSettings, ReasonAgentPinsModel, ReasonHeldUnmapped,
	}
	for _, c := range heldReason {
		codes = append(codes, c)
	}
	for _, c := range codes {
		if _, ok := reasonWords[c]; !ok {
			t.Errorf("reason code %s has no plain-words entry", c)
		}
	}
	if ReasonWords("SOMETHING_NEW") == "" {
		t.Error("an unknown code still gets a sentence")
	}
}

func TestWhy_HeldDecision(t *testing.T) {
	d := td("d1", sessA, 0, "claude-opus-5-5", ReasonClassifierUnconfident, "HELD_UNCONFIDENT_DOWNSHIFT")
	d.Outcome = OutcomeAllow
	d.Corrections = []string{"R1_UNCONFIDENT_DOWNSHIFT"}
	d.SafeVerdict = "OK"
	out := Why([]Event{d}, sessA)
	for _, want := range []string{
		"subagent 1", "asked for   opus", "ran on      opus", "DOWNSHIFT, held to OK (mid, MEDIUM)",
		"held: spawn left unchanged", "what held it", "HELD_UNCONFIDENT_DOWNSHIFT", "not confident enough to downshift",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if n := len(lines(out)); n > 20 {
		t.Errorf("must fit one screen, got %d lines", n)
	}
}

func TestWhy_LiftedDecisionNamesTheLift(t *testing.T) {
	d := td("d1", sessA, 0, "claude-opus-5-5", ReasonClassifierConfident, ReasonSemanticBoost, ReasonRiskFloor)
	d.Verdict, d.Tier, d.FromModel = "UPSHIFT", "frontier", "claude-sonnet-5-5"
	out := Why([]Event{d}, sessA)
	if !strings.Contains(out, "model rewritten to opus") || !strings.Contains(out, "SEMANTIC_BOOST") ||
		!strings.Contains(out, "no guardrail held this decision") {
		t.Fatalf("lifted decision:\n%s", out)
	}
}

func TestWhy_PausedAndLatestDecisionWins(t *testing.T) {
	first := td("d1", sessA, 0, "claude-haiku-5-5", ReasonClassifierConfident)
	paused := td("d2", sessA, time.Minute, "claude-opus-5-5", ReasonClassifierConfident, ReasonPaused)
	paused.Outcome = OutcomeBaseline
	res := Event{CorrelationID: "r2", Timestamp: at(2 * time.Minute), SessionID: sessA, Outcome: OutcomeResolved, LinkedDecision: "d2", ToModel: "claude-opus-5-5"}
	out := Why([]Event{first, paused, res}, sessA)
	if !strings.Contains(out, "subagent 2") || !strings.Contains(out, "paused: spawn left unchanged") || !strings.Contains(out, "PAUSED") {
		t.Fatalf("paused:\n%s", out)
	}
}

func TestWhy_OldEventsAndNoDecisions(t *testing.T) {
	old := Event{CorrelationID: "d1", Timestamp: at(0), SessionID: sessA, Harness: "claude-code", Outcome: OutcomeAllow, Verdict: "OK", Tier: "mid", FromModel: "sonnet", ToModel: "sonnet"}
	if out := Why([]Event{old}, sessA); !strings.Contains(out, "predates reason codes") {
		t.Fatalf("old event:\n%s", out)
	}
	if out := Why(nil, sessA); !strings.Contains(out, "no subagent decision") {
		t.Fatalf("empty:\n%s", out)
	}
}
