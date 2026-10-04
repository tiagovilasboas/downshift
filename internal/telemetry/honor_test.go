// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package telemetry_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
)

func TestCountInferredHonored_CodexFollowUp(t *testing.T) {
	events := []telemetry.Event{
		{
			Outcome:   "rewrite_emitted",
			Harness:   "codex",
			SessionID: "abc",
			FromModel: "gpt-6-luna",
			ToModel:   "gpt-5.6-terra",
			Verdict:   "UPSHIFT",
		},
		{
			Outcome:   "rewrite_emitted",
			Harness:   "codex",
			SessionID: "abc",
			FromModel: "gpt-5.6-terra",
			ToModel:   "gpt-5.6-terra",
			Verdict:   "OK",
		},
	}
	shifted, honored := telemetry.CountInferredHonored(events)
	if shifted != 1 || honored != 1 {
		t.Fatalf("shifted=%d honored=%d, want 1 1", shifted, honored)
	}
}

func TestCountInferredHonored_NoSessionSkipped(t *testing.T) {
	events := []telemetry.Event{
		{
			Outcome:   "rewrite_emitted",
			Harness:   "claude-code",
			FromModel: "claude-opus-4-8",
			ToModel:   "claude-haiku-4-5",
		},
	}
	shifted, honored := telemetry.CountInferredHonored(events)
	if shifted != 0 || honored != 0 {
		t.Fatalf("shifted=%d honored=%d, want 0 0", shifted, honored)
	}
}

func TestCountInferredHonored_ExplicitFlag(t *testing.T) {
	yes := true
	events := []telemetry.Event{
		{
			Outcome:        "rewrite_emitted",
			Harness:        "codex",
			SessionID:      "abc",
			FromModel:      "gpt-6-sol",
			ToModel:        "gpt-6-luna",
			RewriteHonored: &yes,
		},
	}
	shifted, honored := telemetry.CountInferredHonored(events)
	if shifted != 1 || honored != 1 {
		t.Fatalf("shifted=%d honored=%d, want 1 1", shifted, honored)
	}
}

// A held decision, a KiroCrew block and an allow event wrote no model, so
// they are not shifts even when a later spawn asks for the recommended id.
func TestCountInferredHonored_OnlyAppliedRewritesAreShifts(t *testing.T) {
	later := telemetry.Event{Outcome: "allow", Harness: "codex", SessionID: "s", FromModel: "gpt-5.6-terra", ToModel: "gpt-5.6-terra", Verdict: "OK"}
	for name, ev := range map[string]telemetry.Event{
		"held":    {Outcome: "rewrite_emitted", Corrections: []string{"R1_UNCONFIDENT_DOWNSHIFT"}, Harness: "codex", SessionID: "s", FromModel: "gpt-5.6-sol", ToModel: "gpt-5.6-terra"},
		"blocked": {Outcome: "blocked", Harness: "codex", SessionID: "s", FromModel: "gpt-5.6-sol", ToModel: "gpt-5.6-terra"},
		"allow":   {Outcome: "allow", Harness: "codex", SessionID: "s", FromModel: "gpt-5.6-sol", ToModel: "gpt-5.6-terra"},
	} {
		shifted, honored := telemetry.CountInferredHonored([]telemetry.Event{ev, later})
		if shifted != 0 || honored != 0 {
			t.Errorf("%s: shifted=%d honored=%d, want 0 0", name, shifted, honored)
		}
	}
}

// The follow-up spawn that confirms a rewrite may itself be an allow event.
func TestCountInferredHonored_AllowFollowUpCounts(t *testing.T) {
	events := []telemetry.Event{
		{Outcome: "rewrite_emitted", Harness: "codex", SessionID: "s", FromModel: "gpt-6-luna", ToModel: "gpt-5.6-terra", Verdict: "UPSHIFT"},
		{Outcome: "allow", Harness: "codex", SessionID: "s", FromModel: "gpt-5.6-terra", ToModel: "gpt-5.6-terra", Verdict: "OK"},
	}
	if shifted, honored := telemetry.CountInferredHonored(events); shifted != 1 || honored != 1 {
		t.Fatalf("shifted=%d honored=%d, want 1 1", shifted, honored)
	}
}
