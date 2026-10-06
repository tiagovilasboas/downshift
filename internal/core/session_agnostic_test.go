// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestPlanForSession_UnknownIDsAreSelectedFromSessionOrder(t *testing.T) {
	for _, tc := range []struct {
		harness string
		caps    core.HarnessCapabilities
	}{
		{"claude-code", core.ClaudeCodeCaps},
		{"cursor", core.CursorCaps},
		{"codex", core.CodexCaps},
		{"antigravity", core.AntigravityCaps},
		{"kirocrew", core.KiroCrewCaps},
	} {
		t.Run(tc.harness, func(t *testing.T) {
			ids := []string{"session-low-x91", "session-mid-q27", "session-high-k44"}
			d := core.Decision{
				Harness:      tc.harness,
				RequestedID:  ids[2],
				Verdict:      core.VerdictDownshift,
				Confident:    true,
				Tier:         core.TierSmall,
				Model:        core.Model{ID: "recommendation-not-in-session", Harness: tc.harness},
				CurrentModel: core.Model{ID: ids[2], Harness: tc.harness},
			}
			plan := d.PlanForSession(tc.caps, nil, core.KnownSession(ids))
			if plan.Model.ID != ids[0] {
				t.Fatalf("target = %q, want first session member %q", plan.Model.ID, ids[0])
			}
			if !core.CanWriteSessionID(tc.harness, plan.Model.ID, core.KnownSession(ids), nil) {
				t.Fatalf("session member %q was rejected without catalog metadata", plan.Model.ID)
			}
		})
	}
}

func TestPlanForSession_MidTierUsesSessionMidpointWithoutCatalog(t *testing.T) {
	ids := []string{"opaque-a", "opaque-b", "opaque-c", "opaque-d", "opaque-e"}
	d := core.Decision{
		Harness:      "claude-code",
		RequestedID:  ids[len(ids)-1],
		Verdict:      core.VerdictDownshift,
		Confident:    true,
		Tier:         core.TierMid,
		Model:        core.Model{ID: "unlisted", Harness: "claude-code"},
		CurrentModel: core.Model{ID: ids[len(ids)-1], Harness: "claude-code"},
	}
	plan := d.PlanForSession(core.ClaudeCodeCaps, nil, core.KnownSession(ids))
	if plan.Model.ID != ids[2] {
		t.Fatalf("target = %q, want session midpoint %q", plan.Model.ID, ids[2])
	}
}
