// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// Every Claude Code decision records the subagent type as a label, so stats can
// say which kinds of spawn had no known baseline. A custom agent's own name must
// never reach the event log, which stats --export is built from.
func TestHookE2E_ClaudeCodeRecordsAgentTypeLabel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
	frontierID := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	setSessionAllowlist(t, "claude-code", []string{
		cmdCat.ModelFor("claude-code", core.TierSmall).ID,
		cmdCat.ModelFor("claude-code", core.TierMid).ID,
		frontierID,
	})
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"type":"assistant","message":{"model":"`+frontierID+`","content":[]}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ input, want string }{
		{"Explore", "Explore"},
		{"Plan", "Plan"},
		{"general-purpose", "general-purpose"},
		{"", "unspecified"},
		{"voomp-payments-fraud-auditor", "custom"},
	} {
		if err := os.WriteFile(eventLog, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		toolInput := map[string]any{"prompt": "rename the variable userId to userIdentifier"}
		if tc.input != "" {
			toolInput["subagent_type"] = tc.input
		}
		stdin, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "tool_name": "Agent", "transcript_path": transcript, "tool_input": toolInput})
		captureStdout(func() {
			runHookAdapter(
				"claude-code",
				bytes.NewReader(stdin),
				func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
				func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, cmdCat) },
				printAllow,
			)
		})
		raw, err := os.ReadFile(eventLog)
		if err != nil {
			t.Fatal(err)
		}
		var ev map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(raw), &ev); err != nil {
			t.Fatalf("%q: one event expected, got %q", tc.input, raw)
		}
		if ev["agent_type"] != tc.want {
			t.Errorf("subagent_type %q: agent_type = %v, want %q", tc.input, ev["agent_type"], tc.want)
		}
		if tc.want == "custom" && strings.Contains(string(raw), tc.input) {
			t.Errorf("the custom agent's name reached the event log: %s", raw)
		}
	}
}

// Harnesses that report no subagent type record none, and stay as before.
func TestHookE2E_CursorEventHasNoAgentType(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
	captureStdout(func() {
		runHookAdapter(
			"cursor",
			strings.NewReader(`{"hook_event_name":"preToolUse","tool_name":"Task","tool_input":{"task":"rename the userId variable to userIdentifier","model":"inherit"}}`),
			func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
			func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
			printCursorAllow,
		)
	})
	raw, err := os.ReadFile(eventLog)
	if err != nil || len(bytes.TrimSpace(raw)) == 0 {
		t.Fatalf("the cursor hook should have recorded one event: %v %q", err, raw)
	}
	if strings.Contains(string(raw), "agent_type") {
		t.Errorf("cursor event should carry no agent_type: %s", raw)
	}
}
