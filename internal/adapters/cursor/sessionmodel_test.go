// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package cursor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
)

func writeCursorTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	var b []byte
	for _, l := range lines {
		b = append(b, l...)
		b = append(b, '\n')
	}
	p := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRequestedModel_TranscriptTail(t *testing.T) {
	path := writeCursorTranscript(t,
		`{"type":"user","message":{"content":"hi"}}`,
		`{"type":"assistant","message":{"model":"claude-opus-5-thinking-high","content":[{"type":"text","text":"ok"}]}}`,
	)
	ev := cursor.Event{
		ToolName:       "Task",
		ToolInput:      json.RawMessage(`{"prompt":"rename userId","subagent_type":"explore"}`),
		TranscriptPath: path,
		ToolUseID:      "toolu_spawn",
	}
	if got := ev.RequestedModel(); got != "claude-opus-5-thinking-high" {
		t.Fatalf("RequestedModel() = %q", got)
	}
}

func TestRequestedModel_TranscriptCallWinsOverTail(t *testing.T) {
	path := writeCursorTranscript(t,
		`{"type":"assistant","message":{"model":"claude-opus-5-thinking-high","content":[{"type":"tool_use","id":"toolu_spawn","name":"Task"}]}}`,
		`{"type":"assistant","message":{"model":"composer-2.5-fast","content":[{"type":"text"}]}}`,
	)
	ev := cursor.Event{
		ToolName:       "Task",
		ToolInput:      json.RawMessage(`{"prompt":"rename userId"}`),
		TranscriptPath: path,
		ToolUseID:      "toolu_spawn",
	}
	if got := ev.RequestedModel(); got != "claude-opus-5-thinking-high" {
		t.Fatalf("RequestedModel() = %q, want model at spawn call", got)
	}
}

func TestHandle_TranscriptBaselineTrivialDownshift(t *testing.T) {
	path := writeCursorTranscript(t,
		`{"type":"assistant","message":{"model":"`+catID(core.TierFrontier)+`","content":[{"type":"text"}]}}`,
	)
	ev := withCatalogSession(cursor.Event{
		ToolName:       "Task",
		ToolInput:      json.RawMessage(`{"prompt":"rename the userId variable to userIdentifier"}`),
		TranscriptPath: path,
	})
	out, _, d := cursor.Handle(ev, cat)
	if d.Baseline.RequestedID != catID(core.TierFrontier) {
		t.Fatalf("baseline = %q", d.Baseline.RequestedID)
	}
	if d.Verdict != core.VerdictDownshift || d.Tier != core.TierSmall {
		t.Fatalf("want small downshift, got %v tier %v", d.Verdict, d.Tier)
	}
	if m := decodeUpdated(t, out); m == nil {
		t.Fatal("expected rewrite on trivial task with transcript baseline")
	}
}
