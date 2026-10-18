// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

func TestRecordSessionUnknownSession(t *testing.T) {
	d := Decision{Harness: "claude-code"}
	d.RecordSession(UnknownSession(), "some-model")
	if !d.Session.Unknown {
		t.Fatal("an unknown session must be recorded as unknown")
	}
	if d.Session.QuotaSource != "" {
		t.Fatalf("no usage evidence must leave the source empty, got %q", d.Session.QuotaSource)
	}
}

func TestRecordSessionKnownSession(t *testing.T) {
	d := Decision{Harness: "claude-code"}
	d.RecordSession(KnownSession([]string{"a", "b"}), "a")
	if d.Session.Unknown {
		t.Fatal("a known session must not be recorded as unknown")
	}
	if d.Session.QuotaStatus == "" {
		t.Fatal("quota status must always be stamped, even when unknown")
	}
}

// PlanSession must equal the hand-written sequence every adapter used to carry:
// record, plan, copy the credit hold, re-stamp quota on a rewrite.
func TestPlanSessionMatchesManualSequence(t *testing.T) {
	session := KnownSession([]string{"small-m", "mid-m", "big-m"})

	manual := Route("rename the userId variable to accountId", "claude-code", "big-m")
	manual.Baseline.RequestedID = "big-m"
	manual.RecordSession(session, "big-m")
	wantPlan := manual.PlanForSession(ClaudeCodeCaps, nil, session)
	manual.Session.CreditHeld = wantPlan.CreditHeld
	if wantPlan.RewriteModel {
		manual.Session.QuotaStatus = string(session.QuotaStatus(manual.Harness, wantPlan.Model.ID))
	}

	got := Route("rename the userId variable to accountId", "claude-code", "big-m")
	got.Baseline.RequestedID = "big-m"
	gotPlan := got.PlanSession(ClaudeCodeCaps, nil, session, "big-m")

	if gotPlan.RewriteModel != wantPlan.RewriteModel || gotPlan.Model.ID != wantPlan.Model.ID {
		t.Fatalf("plan differs: got %+v want %+v", gotPlan, wantPlan)
	}
	if got.Session.QuotaStatus != manual.Session.QuotaStatus || got.Session.CreditHeld != manual.Session.CreditHeld || got.Session.Unknown != manual.Session.Unknown {
		t.Fatalf("stamped fields differ: got %+v want %+v", got, manual)
	}
}
