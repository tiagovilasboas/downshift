// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"
)

// heldSplitLog is a fixture session of six decisions. The expected split is
// worked out by hand in TestHeldSplit_FixtureLog.
func heldSplitLog() []Event {
	const sess = "heldsplit"
	base := func(id, outcome, verdict string) Event {
		return Event{CorrelationID: id, Harness: "claude-code", SessionID: sess, Outcome: outcome,
			Verdict: verdict, FromModel: "claude-opus-4-8", ToModel: "claude-haiku-4-5"}
	}
	switched := base("switched", OutcomeRewriteEmitted, "DOWNSHIFT")
	r1 := base("held-r1", OutcomeAllow, "DOWNSHIFT")
	r1.Corrections = []string{"R1_UNCONFIDENT_DOWNSHIFT"}
	r8 := base("held-r8-upshift", OutcomeAllow, "UPSHIFT")
	r8.Corrections = []string{"R8_UNCONFIDENT_UPSHIFT"}
	heldReason := base("held-reason", OutcomeAllow, "DOWNSHIFT")
	heldReason.ReasonCodes = []string{"HELD_COMPLEX_DOWNSHIFT"}
	credit := base("upshift-credit", OutcomeAllow, "UPSHIFT")
	credit.CreditHeld = true
	observed := base("upshift-observed", OutcomeAllow, "UPSHIFT")
	return []Event{switched, r1, r8, heldReason, credit, observed}
}

// TestHeldSplit_FixtureLog: held stays the sum of the held spawns. The guardrail
// holds (R1, R8, HELD_*) go to held_guardrail; the upshifts that did not change
// the spawn (credit-held, observed) go to upshift_not_applied. Panel and tree agree.
func TestHeldSplit_FixtureLog(t *testing.T) {
	events := heldSplitLog()
	const sess = "heldsplit"
	p := BuildPanel(events, sess)
	if p.Switched != 1 || p.Held != 4 || p.HeldGuardrail != 3 || p.UpshiftNotApplied != 2 {
		t.Fatalf("panel switched %d held %d held_guardrail %d upshift_not_applied %d, want 1 4 3 2",
			p.Switched, p.Held, p.HeldGuardrail, p.UpshiftNotApplied)
	}
	tree := BuildSessionTree(events, sess, TreeOptions{})
	if tree.Held != 4 || tree.HeldGuardrail != 3 || tree.UpshiftNotApplied != 2 {
		t.Fatalf("tree held %d held_guardrail %d upshift_not_applied %d, want 4 3 2",
			tree.Held, tree.HeldGuardrail, tree.UpshiftNotApplied)
	}
	if out := tree.Render(); !strings.Contains(out, "3 held") || !strings.Contains(out, "2 upshift not applied") {
		t.Fatalf("header lacks both counts:\n%s", out)
	}
}
