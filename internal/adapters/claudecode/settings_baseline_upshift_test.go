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

func TestHandle_SettingsBaselineUpshiftKeepsStrongest(t *testing.T) {
	t.Setenv("DOWNSHIFT_EXPLICIT_UPSHIFT", "")
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent.json"))
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "haiku")
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	sess := []string{"claude-haiku-4-5", "claude-sonnet-5-5", "claude-opus-5-5"}
	ev := claudecode.Event{
		ToolName:       "Agent",
		TranscriptPath: transcript,
		Cwd:            t.TempDir(),
		SessionModels:  &sess,
		ToolInput:      json.RawMessage(`{"subagent_type":"general-purpose","prompt":"implement the feature to export sales as CSV"}`),
	}
	out, _, d := claudecode.Handle(ev, cat)
	if !d.Baseline.FromSettings || !d.Baseline.Guessed || d.Verdict != core.VerdictUpshift {
		t.Fatalf("precondition: want an upshift on a settings baseline, got %+v", d)
	}
	if m := decodeUpdated(t, out); m == nil || m["model"] != "opus" {
		t.Fatalf("settings-baseline upshift wrote %v, want opus", m)
	}

	ev.Model = "claude-haiku-4-5"
	out, _, d = claudecode.Handle(ev, cat)
	if d.Baseline.Guessed {
		t.Fatalf("precondition: payload model is not a guess")
	}
	if m := decodeUpdated(t, out); m == nil || m["model"] != "sonnet" {
		t.Fatalf("observed-baseline upshift wrote %v, want sonnet", m)
	}
}
