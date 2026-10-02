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
