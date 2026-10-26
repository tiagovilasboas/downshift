// Copyright (c) 2026 Tiago Vilas Boas.
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

// A baseline read from settings is a guess: the session may have been started
// with --model haiku. A downshift from it to the mid tier could then be an
// upshift nobody sees, so it is held; only the small tier is written.
func TestHandle_SettingsBaselineHoldsMidTierDownshift(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "opus")
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ev := claudecode.Event{
		ToolName:       "Agent",
		TranscriptPath: transcript,
		Cwd:            t.TempDir(),
		ToolInput:      json.RawMessage(`{"subagent_type":"general-purpose","prompt":"implement the feature to export sales as CSV"}`),
	}
	out, note, d := claudecode.Handle(withCatalogSession(ev), cat)
	if !d.Baseline.FromSettings || !d.Confident {
		t.Fatalf("precondition: want confident decision on a settings baseline, got fromSettings=%v confident=%v", d.Baseline.FromSettings, d.Confident)
	}
	if !d.Baseline.GuessHeld {
		t.Errorf("held mid-tier downshift must set GuessHeld for its reason code")
	}
	if m := decodeUpdated(t, out); m != nil {
		t.Fatalf("mid-tier downshift on a guessed baseline, model rewritten to %v (verdict %v, note %q)", m["model"], d.Verdict, note)
	}
}

func TestHandle_SettingsBaselineStillWritesSmallTier(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "opus")
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	ev := claudecode.Event{
		ToolName:       "Agent",
		TranscriptPath: transcript,
		Cwd:            t.TempDir(),
		ToolInput:      json.RawMessage(`{"subagent_type":"general-purpose","prompt":"rename the userId variable to userIdentifier"}`),
	}
	out, note, d := claudecode.Handle(withCatalogSession(ev), cat)
	if !d.Baseline.FromSettings || d.Verdict != core.VerdictDownshift || d.Tier != core.TierSmall {
		t.Fatalf("precondition: want a small-tier downshift on a settings baseline, got fromSettings=%v verdict=%v tier=%v", d.Baseline.FromSettings, d.Verdict, d.Tier)
	}
	if m := decodeUpdated(t, out); m == nil {
		t.Fatalf("small-tier downshift on a settings baseline must still be written (note %q)", note)
	}
}
