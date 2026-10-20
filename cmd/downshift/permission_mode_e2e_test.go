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
	"github.com/tiagovilasboas/downshift/internal/core"
)

// A Claude Code payload that names its permission mode yields a decision record
// with that mode name; a payload without it yields a record with no mode. The
// prompt and any path never reach the log.
func TestHookE2E_ClaudeCodeRecordsPermissionMode(t *testing.T) {
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

	for _, tc := range []struct {
		name, mode, want string
	}{
		{"plan mode", "plan", "plan"},
		{"auto mode", "auto", "auto"},
		{"absent", "", ""},
		{"unknown mode is labelled", "/Users/someone/private-mode", "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(eventLog, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			payload := map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       "Agent",
				"transcript_path": transcript,
				"tool_input":      map[string]any{"subagent_type": "Explore", "prompt": "rename the variable userId to userIdentifier"},
			}
			if tc.mode != "" {
				payload["permission_mode"] = tc.mode
			}
			stdin, _ := json.Marshal(payload)
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
				t.Fatalf("one event expected, got %q", raw)
			}
			if tc.want == "" {
				if _, ok := ev["permission_mode"]; ok {
					t.Fatalf("no mode sent, but the record has one: %s", raw)
				}
				return
			}
			if ev["permission_mode"] != tc.want {
				t.Errorf("permission_mode = %v, want %q", ev["permission_mode"], tc.want)
			}
			if strings.Contains(string(raw), "/Users/someone") || strings.Contains(string(raw), "userIdentifier") {
				t.Errorf("a raw mode or prompt text reached the event log: %s", raw)
			}
		})
	}
}
