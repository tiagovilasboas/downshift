// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestLinkTargetBy_ToolUseID(t *testing.T) {
	now := time.Now().UTC()
	at := func(d time.Duration) string { return now.Add(-d).Format(time.RFC3339Nano) }
	sess := telemetry.HashSessionID("s")
	callA, callB := telemetry.HashSessionID("toolu_A"), telemetry.HashSessionID("toolu_B")
	legacy := telemetry.Event{CorrelationID: "legacy", Timestamp: at(time.Second), Harness: harnessID, Outcome: telemetry.OutcomeRewriteEmitted, SessionID: sess}
	decA := telemetry.Event{CorrelationID: "A", Timestamp: at(3 * time.Second), Harness: harnessID, Outcome: telemetry.OutcomeRewriteEmitted, SessionID: sess, ToolUseHash: callA}
	decB := telemetry.Event{CorrelationID: "B", Timestamp: at(2 * time.Second), Harness: harnessID, Outcome: telemetry.OutcomeRewriteEmitted, SessionID: sess, ToolUseHash: callB}
	prior := []telemetry.Event{decA, decB, legacy}

	if got := linkTargetBy(prior, sess, callA, "", now, telemetry.OutcomeResolved); got == nil || got.CorrelationID != "A" {
		t.Fatalf("exact call: got %+v, want A", got)
	}
	// An unknown call id still falls back to the session heuristic, but never
	// to a decision recorded for another call.
	if got := linkTargetBy(prior, sess, telemetry.HashSessionID("toolu_C"), "", now, telemetry.OutcomeResolved); got == nil || got.CorrelationID != "legacy" {
		t.Fatalf("unknown call: got %+v, want legacy", got)
	}
	// A repeated PostToolUse for an already linked call links nothing.
	linked := append(prior, telemetry.Event{Outcome: telemetry.OutcomeResolved, LinkedDecision: "A"})
	if got := linkTargetBy(linked, sess, callA, "", now, telemetry.OutcomeResolved); got != nil {
		t.Fatalf("double fire: got %+v, want nil", got)
	}
}
