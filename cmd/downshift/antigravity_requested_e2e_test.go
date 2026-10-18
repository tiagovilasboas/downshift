// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/antigravity"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// The Antigravity payload carries each subagent's Model. The shared runner
// must log it as requested_model instead of "unknown", for the same child
// whose decision is recorded (the last subagent with a prompt).
func TestHookE2E_AntigravityRequestedModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	frontierID := cmdCat.ModelFor("antigravity", core.TierFrontier).ID
	smallID := cmdCat.ModelFor("antigravity", core.TierSmall).ID

	stdin, _ := json.Marshal(map[string]any{
		"toolCall": map[string]any{
			"name": "invoke_subagent",
			"args": map[string]any{"Subagents": []any{
				map[string]any{"Prompt": "rename the variable userId", "Model": smallID},
				map[string]any{"Prompt": "rename the variable orderId", "Model": frontierID},
			}},
		},
	})
	captureStdout(func() {
		runHookAdapter(
			"antigravity",
			bytes.NewReader(stdin),
			func(b []byte) (antigravity.Event, error) { var e antigravity.Event; return e, json.Unmarshal(b, &e) },
			func(e antigravity.Event) (any, string, core.Decision) { return antigravity.Handle(e, cmdCat) },
			printAllow,
		)
	})
	raw, err := os.ReadFile(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	var ev map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &ev); err != nil {
		t.Fatalf("one decision event expected, got %q: %v", raw, err)
	}
	if ev["requested_model"] != frontierID {
		t.Errorf("requested_model = %v, want %s (last subagent with a prompt)", ev["requested_model"], frontierID)
	}
}
