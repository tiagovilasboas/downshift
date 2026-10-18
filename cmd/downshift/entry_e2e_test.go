// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// A Claude Code payload with entry=mod and parent_model records both on the
// decision event through the shared runner. Empty entry becomes "hook".
func TestHookE2E_EntryAndParentModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	frontier := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	small := cmdCat.ModelFor("claude-code", core.TierSmall).ID
	if frontier == "" || small == "" {
		t.Fatal("claude-code catalog missing tier models")
	}
	setSessionAllowlist(t, "claude-code", []string{
		cmdCat.ModelFor("claude-code", core.TierSmall).ID,
		cmdCat.ModelFor("claude-code", core.TierMid).ID,
		frontier,
	})

	modPayload, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Agent",
		"tool_use_id":     "toolu_mod_1",
		"entry":           "mod",
		"parent_model":    frontier,
		"tool_input": map[string]any{
			"subagent_type": "general-purpose",
			"prompt":        "rename the userId variable to userIdentifier",
			"description":   "rename",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	hookPayload, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Agent",
		"tool_use_id":     "toolu_mod_1", // same spawn id: dual route
		"model":           frontier,
		"tool_input": map[string]any{
			"subagent_type": "general-purpose",
			"prompt":        "rename the userId variable to userIdentifier",
			"description":   "rename",
			"model":         frontier,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	run := func(raw []byte) {
		captureStdout(func() {
			rc := runHookAdapter(
				"claude-code",
				bytes.NewReader(raw),
				func(b []byte) (claudecode.Event, error) {
					var e claudecode.Event
					return e, json.Unmarshal(b, &e)
				},
				func(e claudecode.Event) (any, string, core.Decision) {
					return claudecode.Handle(e, cmdCat)
				},
				printAllow,
			)
			if rc != 0 {
				t.Errorf("runHookAdapter rc = %d", rc)
			}
		})
	}
	run(modPayload)
	run(hookPayload)

	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	var sawMod, sawHook bool
	for _, ev := range events {
		switch ev.Entry {
		case telemetry.EntryMod:
			sawMod = true
			if ev.FromModel != frontier {
				t.Errorf("mod entry FromModel = %q, want parent_model %q", ev.FromModel, frontier)
			}
		case telemetry.EntryHook:
			sawHook = true
		default:
			t.Errorf("unexpected entry %q", ev.Entry)
		}
		if ev.ToolUseHash == "" {
			t.Error("tool_use_hash must be set")
		}
	}
	if !sawMod || !sawHook {
		t.Fatalf("want both entries, sawMod=%v sawHook=%v", sawMod, sawHook)
	}
	if n := telemetry.DuplicateToolUseCount(events); n != 1 {
		t.Fatalf("DuplicateToolUseCount = %d, want 1", n)
	}
	raw, _ := os.ReadFile(eventLog)
	if bytes.Contains(raw, []byte("rename the userId")) {
		t.Fatal("event log must never persist task text")
	}
}
