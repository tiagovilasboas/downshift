// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// Counterpart of TestHandle_HeldDownshiftKeepsRequestedEffort: an upshift held by
// R8 (unconfident) must leave the spawn unchanged, effort included.
func TestHandle_HeldUpshiftLeavesSpawnUnchanged(t *testing.T) {
	smallID := catID(core.TierSmall)
	ev := withCatalogSession(codex.Event{
		ToolName:  "spawn_agent",
		ToolInput: json.RawMessage(`{"message": "look into why the build is slow", "model": "` + smallID + `", "reasoning_effort": "low"}`),
	})
	out, note, d := codex.Handle(ev, cat)
	if d.Verdict != core.VerdictUpshift || d.Confident || d.SafeVerdict != core.VerdictOK {
		t.Fatalf("precondition: want R8-held upshift, got verdict=%v confident=%v safe=%v corrections=%v", d.Verdict, d.Confident, d.SafeVerdict, d.Corrections)
	}
	if m := decodeUpdated(t, out); m != nil {
		t.Fatalf("held upshift must leave the spawn unchanged, got %v (note %q, corrections %v)", m, note, d.Corrections)
	}
}
