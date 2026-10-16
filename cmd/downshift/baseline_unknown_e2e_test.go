// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/antigravity"
	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// Characterization of the recorded baseline per non-Claude harness. A
// resolvable requested model is logged as requested_model; an unresolvable
// one ("inherit", nothing sent) is logged as unknown, with the raw value kept
// only when one was sent. A regression that turns a resolvable baseline into
// unknown fails here instead of passing silently.
func TestHookE2E_BaselineUnknownPerHarness(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	rollout := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(rollout, []byte(`{"type":"turn_context","payload":{"model":"gpt-6-luna"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runCursor := func(stdin []byte) {
		runHookAdapter(
			"cursor",
			bytes.NewReader(stdin),
			func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
			func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
			printCursorAllow,
		)
	}
	runAntigravity := func(stdin []byte) {
		runHookAdapter(
			"antigravity",
			bytes.NewReader(stdin),
			func(b []byte) (antigravity.Event, error) { var e antigravity.Event; return e, json.Unmarshal(b, &e) },
			func(e antigravity.Event) (any, string, core.Decision) { return antigravity.Handle(e, cmdCat) },
			printAllow,
		)
	}
	runCodex := func(stdin []byte) {
		runHookAdapter(
			"codex",
			bytes.NewReader(stdin),
			func(b []byte) (codex.Event, error) { var e codex.Event; return e, json.Unmarshal(b, &e) },
			func(e codex.Event) (any, string, core.Decision) { return codex.Handle(e, cmdCat) },
			printAllow,
		)
	}
	antigravityPayload := func(subagent map[string]any) map[string]any {
		return map[string]any{"toolCall": map[string]any{
			"name": "invoke_subagent",
			"args": map[string]any{"Subagents": []any{subagent}},
		}}
	}
	codexPayload := func(transcript string) map[string]any {
		return map[string]any{
			"hook_event_name": "PreToolUse", "tool_name": "spawn_agent", "session_id": "s",
			"transcript_path": transcript,
			"tool_input":      map[string]any{"message": "rename the variable userId"},
		}
	}

	cases := []struct {
		name          string
		run           func([]byte)
		payload       map[string]any
		wantRequested string
		wantRaw       string // "" means the field must be absent
	}{
		{
			name: "cursor inherit without parent model is unknown",
			run:  runCursor,
			payload: map[string]any{
				"hook_event_name": "preToolUse", "tool_name": "Task",
				"tool_input": map[string]any{"task": "rename the userId variable", "model": "inherit"},
			},
			wantRequested: "unknown",
		},
		{
			name: "cursor inherit uses model_id baseline",
			run:  runCursor,
			payload: map[string]any{
				"hook_event_name": "preToolUse", "tool_name": "Task", "model_id": "claude-opus-5-thinking-high",
				"tool_input": map[string]any{"task": "rename the userId variable", "model": "inherit"},
			},
			wantRequested: "claude-opus-5-thinking-high",
		},
		{
			name:          "antigravity recognised Model",
			run:           runAntigravity,
			payload:       antigravityPayload(map[string]any{"Prompt": "rename the variable userId", "Model": "pro"}),
			wantRequested: "pro",
		},
		{
			name:          "antigravity inherit is unknown with raw kept",
			run:           runAntigravity,
			payload:       antigravityPayload(map[string]any{"Prompt": "rename the variable userId", "Model": "inherit"}),
			wantRequested: "unknown", wantRaw: "inherit",
		},
		{
			name:          "antigravity absent Model is unknown without raw",
			run:           runAntigravity,
			payload:       antigravityPayload(map[string]any{"Prompt": "rename the variable userId"}),
			wantRequested: "unknown",
		},
		{
			name:          "codex inherited from transcript",
			run:           runCodex,
			payload:       codexPayload(rollout),
			wantRequested: "gpt-6-luna",
		},
		{
			name:          "codex missing transcript is unknown without raw",
			run:           runCodex,
			payload:       codexPayload(filepath.Join(t.TempDir(), "absent.jsonl")),
			wantRequested: "unknown",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(eventLog, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			stdin, _ := json.Marshal(tc.payload)
			captureStdout(func() { tc.run(stdin) })
			raw, err := os.ReadFile(eventLog)
			if err != nil {
				t.Fatal(err)
			}
			var ev map[string]any
			if err := json.Unmarshal(bytes.TrimSpace(raw), &ev); err != nil {
				t.Fatalf("one decision event expected, got %q: %v", raw, err)
			}
			if ev["requested_model"] != tc.wantRequested {
				t.Errorf("requested_model = %v, want %q", ev["requested_model"], tc.wantRequested)
			}
			got, present := ev["requested_model_raw"]
			switch {
			case tc.wantRaw == "" && present:
				t.Errorf("requested_model_raw = %v, want absent", got)
			case tc.wantRaw != "" && got != tc.wantRaw:
				t.Errorf("requested_model_raw = %v, want %q", got, tc.wantRaw)
			}
		})
	}
}
