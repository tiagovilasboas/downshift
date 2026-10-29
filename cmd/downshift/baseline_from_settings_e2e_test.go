// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

// runClaudeCodeHook sends one PreToolUse payload through the real runner and
// returns the single event it records.
func runClaudeCodeHook(t *testing.T, eventLog string, payload map[string]any) map[string]any {
	t.Helper()
	if err := os.WriteFile(eventLog, nil, 0o600); err != nil {
		t.Fatal(err)
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
	return ev
}

func hasCode(ev map[string]any, code string) bool {
	codes, _ := ev["reason_codes"].([]any)
	return slices.ContainsFunc(codes, func(c any) bool { return c == code })
}

// A first assistant message with several Agent calls leaves the transcript
// empty at the hook. The configured model then supplies the baseline: the spawn
// whose baseline was unknown is routed as a downshift, and the event says the baseline was inferred.
func TestHookE2E_ClaudeCodeBaselineFromSettings(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "")
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
	setSessionAllowlist(t, "claude-code", []string{
		cmdCat.ModelFor("claude-code", core.TierSmall).ID,
		cmdCat.ModelFor("claude-code", core.TierMid).ID,
		cmdCat.ModelFor("claude-code", core.TierFrontier).ID,
	})
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(transcript, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	payload := map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Agent", "transcript_path": transcript, "cwd": project,
		"tool_input": map[string]any{"subagent_type": "general-purpose", "prompt": "rename the variable userId to userIdentifier"},
	}

	before := runClaudeCodeHook(t, eventLog, payload)
	if before["verdict"] != "UNKNOWN" || before["requested_model"] != "unknown" || hasCode(before, "BASELINE_FROM_SETTINGS") {
		t.Fatalf("without a configured model the baseline must be unknown: %v", before)
	}

	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte(`{"model":"sonnet"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	after := runClaudeCodeHook(t, eventLog, payload)
	if after["verdict"] != "DOWNSHIFT" {
		t.Errorf("verdict = %v, want DOWNSHIFT: %v", after["verdict"], after)
	}
	if !hasCode(after, "BASELINE_FROM_SETTINGS") {
		t.Errorf("reason_codes = %v, want BASELINE_FROM_SETTINGS", after["reason_codes"])
	}
	if after["requested_model"] != cmdCat.ModelFor("claude-code", core.TierMid).ID {
		t.Errorf("requested_model = %v, want the mid catalog id", after["requested_model"])
	}
	t.Logf("event: %v", after)
}

// A transcript that has the model keeps the old path: no settings read, no code.
func TestHookE2E_ClaudeCodeTranscriptModelSkipsSettingsFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "")
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
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte(`{"model":"haiku"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ev := runClaudeCodeHook(t, eventLog, map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Agent", "transcript_path": transcript, "cwd": project,
		"tool_input": map[string]any{"subagent_type": "general-purpose", "prompt": "rename the variable userId to userIdentifier"},
	})
	if hasCode(ev, "BASELINE_FROM_SETTINGS") {
		t.Errorf("the fallback ran although the transcript has the model: %v", ev)
	}
	if ev["requested_model"] != frontierID {
		t.Errorf("requested_model = %v, want the transcript model %s", ev["requested_model"], frontierID)
	}
}
