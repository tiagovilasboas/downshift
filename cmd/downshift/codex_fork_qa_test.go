// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Exercise the real shared runner so an unmodified full-history spawn cannot
// be reported as an emitted rewrite or as a child running the recommendation.
func TestQA_CodexForkCapabilityPreservesInputAndTelemetry(t *testing.T) {
	for _, tc := range []struct {
		name string
		fork string
		full bool
	}{
		{"explicit full", "all", true},
		{"current default full", "", true},
		{"bounded history", "2", true},
		{"independent", "none", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			frontier := cmdCat.ModelFor("codex", core.TierFrontier).ID
			small := cmdCat.ModelFor("codex", core.TierSmall).ID
			setSessionAllowlist(t, "codex", []string{small, frontier})
			input := map[string]any{
				"message":   "rename the userId variable to userIdentifier",
				"task_name": "worker", "agent_type": "default",
				"custom": map[string]any{"keep": []any{"nested", true}},
			}
			if tc.fork != "" {
				input["fork_turns"] = tc.fork
			}
			payload, err := json.Marshal(map[string]any{
				"hook_event_name": "PreToolUse", "tool_name": "spawn_agent",
				"model": frontier, "tool_input": input,
			})
			if err != nil {
				t.Fatal(err)
			}
			out := captureStdout(func() {
				if rc := harnessRegistry["codex"].Hook(bytes.NewReader(payload), cmdCat); rc != 0 {
					t.Errorf("hook exit=%d, want 0", rc)
				}
			})
			var wire struct {
				Specific *struct {
					Updated map[string]any `json:"updatedInput"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal([]byte(out), &wire); err != nil {
				t.Fatalf("invalid hook output %q: %v", out, err)
			}
			events, err := telemetry.ReadEventsFrom(eventLog)
			if err != nil || len(events) != 1 {
				t.Fatalf("want one observation, got %d: %v", len(events), err)
			}
			event := events[0]
			if event.FromModel != frontier || event.RewriteHonored != nil || event.HasTokenUsage() {
				t.Fatalf("incorrect baseline or fabricated runtime evidence: %+v", event)
			}
			if tc.full {
				if strings.TrimSpace(out) != "{}" || wire.Specific != nil {
					t.Fatalf("full history must leave tool call and permissions untouched: %s", out)
				}
				if event.Outcome != telemetry.OutcomeAllow || event.ToModel != frontier || event.FinalEffort != event.RequestedEffort {
					t.Fatalf("unchanged call recorded as a rewrite: %+v", event)
				}
			} else {
				if wire.Specific == nil || wire.Specific.Updated["model"] != small {
					t.Fatalf("independent spawn lost routing: %s", out)
				}
				for key, value := range input {
					if !reflect.DeepEqual(wire.Specific.Updated[key], value) {
						t.Errorf("input field %s was changed", key)
					}
				}
				if event.Outcome != telemetry.OutcomeRewriteEmitted || event.ToModel != small {
					t.Fatalf("rewrite provenance missing: %+v", event)
				}
			}
			data, err := os.ReadFile(eventLog)
			if err != nil || bytes.Contains(data, []byte(input["message"].(string))) {
				t.Fatalf("event log unreadable or contains task text: %v", err)
			}
		})
	}
}
