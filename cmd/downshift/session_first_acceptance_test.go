// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"encoding/json"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"os"
	"path/filepath"
	"testing"
)

func TestHookE2E_SessionFirstNewIDsAcrossHarnesses(t *testing.T) {
	decidertest.Use(t, guardedLanguageStub())
	t.Setenv("HOME", t.TempDir())
	const current = "qa-new-frontier"
	ids := []string{"qa-new-small", "qa-new-mid", current}
	for _, h := range core.AllHarnessCaps() {
		t.Run(h.Harness, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", path)
			for _, prompt := range []string{"rename the userId variable to userIdentifier", "rename el campo que contiene las contraseñas de clientes"} {
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
				input := map[string]any{"task": prompt, "prompt": prompt, "message": prompt, "model": current}
				payload := map[string]any{"tool_name": "Task", "hook_event_name": "PreToolUse", "tool_input": input, "session_models": ids}
				switch h.Harness {
				case "codex":
					payload["tool_name"] = "spawn_agent"
				case "kirocrew":
					payload["tool_name"] = "spawn_run"
				case "cursor":
					payload["hook_event_name"] = "preToolUse"
				case "antigravity":
					payload = map[string]any{"session_models": ids, "toolCall": map[string]any{"name": "invoke_subagent", "args": map[string]any{"Subagents": []any{map[string]any{"Prompt": prompt, "Model": current}}}}}
				case "opencode":
					// The route wire carries no session list: the allowlist
					// comes from the operator file instead of the payload.
					setSessionAllowlist(t, h.Harness, ids)
					payload = map[string]any{
						"v":           1,
						"kind":        "route",
						"harness":     map[string]any{"id": "opencode"},
						"event":       "subagent_spawn_intent",
						"correlation": map[string]any{"child": "00112233445566778899aabbccddeeff"},
						"agent":       "general",
						"model":       map[string]any{"providerID": "opencode", "id": current},
						"taskText":    prompt,
					}
				}
				raw, err := json.Marshal(payload)
				if err != nil {
					t.Fatal(err)
				}
				captureStdout(func() { runQAAdapterFixture(raw, h.Harness) })
				events, err := telemetry.ReadEventsFrom(path)
				if err != nil {
					t.Fatal(err)
				}
				if len(events) != 1 {
					t.Fatalf("events=%d want 1", len(events))
				}
				ev := events[0]
				if ev.FromModel != current {
					t.Errorf("ordered session baseline lost: requested=%s want %s", ev.FromModel, current)
				}
				if prompt != "rename the userId variable to userIdentifier" {
					if ev.ToModel != current || ev.Outcome != telemetry.OutcomeAllow {
						t.Errorf("language guard must hold exact active model: %+v", ev)
					}
				}
				if ev.EstimatedSavings != 0 {
					t.Errorf("synthetic unknown prices generated savings: %+v", ev)
				}
			}
		})
	}
}
