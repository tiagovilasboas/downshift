// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// parent_model is a fact from the mod's agent.spawn event. For an inheriting
// type with no per-call model it is the baseline before the transcript, and it
// is never marked as a settings guess.
func TestHandle_ParentModelBeatsEmptyTranscript(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "opus") // would be the settings guess without parent_model
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	frontier := catID(core.TierFrontier)
	ev := withCatalogSession(claudecode.Event{
		ToolName:       "Agent",
		TranscriptPath: transcript,
		Cwd:            t.TempDir(),
		ParentModel:    frontier,
		ToolInput:      json.RawMessage(`{"subagent_type":"general-purpose","prompt":"rename the userId variable to userIdentifier"}`),
	})
	out, note, d := claudecode.Handle(ev, cat)
	if d.Baseline.FromSettings || d.Baseline.Guessed {
		t.Fatalf("parent_model must not be a settings guess, fromSettings=%v guessed=%v", d.Baseline.FromSettings, d.Baseline.Guessed)
	}
	if d.Baseline.RequestedID != frontier {
		t.Fatalf("baseline = %q, want parent_model %q", d.Baseline.RequestedID, frontier)
	}
	if d.Verdict != core.VerdictDownshift || d.Tier != core.TierSmall {
		t.Fatalf("want small downshift from parent frontier, got verdict=%v tier=%v note=%q", d.Verdict, d.Tier, note)
	}
	if m := decodeUpdated(t, out); m == nil {
		t.Fatalf("rewrite must be written (note %q)", note)
	}
}

// An explicit tool_input model still wins over parent_model.
func TestHandle_ExplicitModelBeatsParentModel(t *testing.T) {
	mid := catID(core.TierMid)
	frontier := catID(core.TierFrontier)
	ev := withCatalogSession(claudecode.Event{
		ToolName:    "Agent",
		ParentModel: frontier,
		ToolInput:   json.RawMessage(`{"subagent_type":"general-purpose","model":"` + mid + `","prompt":"rename the userId variable to userIdentifier"}`),
	})
	_, _, d := claudecode.Handle(ev, cat)
	if d.Baseline.RequestedID != mid {
		t.Fatalf("baseline = %q, want explicit mid %q", d.Baseline.RequestedID, mid)
	}
}

// Non-inheriting agent types keep an unknown baseline even when parent_model
// is set: the parent would be a guess for a pinned custom agent.
func TestHandle_ParentModelIgnoredForPinnedAgentType(t *testing.T) {
	frontier := catID(core.TierFrontier)
	ev := withCatalogSession(claudecode.Event{
		ToolName:    "Agent",
		ParentModel: frontier,
		ToolInput:   json.RawMessage(`{"subagent_type":"statusline-setup","prompt":"rename the userId variable to userIdentifier"}`),
	})
	_, _, d := claudecode.Handle(ev, cat)
	if d.Baseline.RequestedID != "" {
		t.Fatalf("pinned agent type must keep unknown baseline, got %q", d.Baseline.RequestedID)
	}
}

// parent_model wins over a later transcript line: the mod already knows the
// parent's effective model, so the hook does not wait on disk.
func TestHandle_ParentModelBeatsTranscript(t *testing.T) {
	frontier := catID(core.TierFrontier)
	mid := catID(core.TierMid)
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	line := `{"type":"assistant","message":{"model":"` + mid + `"}}` + "\n"
	if err := os.WriteFile(transcript, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	ev := withCatalogSession(claudecode.Event{
		ToolName:       "Agent",
		TranscriptPath: transcript,
		ParentModel:    frontier,
		ToolInput:      json.RawMessage(`{"subagent_type":"general-purpose","prompt":"rename the userId variable to userIdentifier"}`),
	})
	_, _, d := claudecode.Handle(ev, cat)
	if d.Baseline.RequestedID != frontier {
		t.Fatalf("baseline = %q, want parent_model %q over transcript mid", d.Baseline.RequestedID, frontier)
	}
}
