// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/cascade"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// TestClaudeCodeMVP_E2E verifies the complete Onda 1 MVP lifecycle for Claude Code:
// 1. Shell command rewriting to downshift compact -- <cmd>
// 2. Subagent model downshifting to economic tier with cache preservation
// 3. Sensor-guided Auto-Upgrade cascade on test failure
// 4. Token accounting linkage on SubagentStop
// 5. Git commit acceptance recording
func TestClaudeCodeMVP_E2E(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	eventLog := filepath.Join(tempHome, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")

	sessionID := "sess_mvp_e2e_001"
	frontierID := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	smallID := cmdCat.ModelFor("claude-code", core.TierSmall).ID
	if frontierID == "" || smallID == "" {
		t.Fatal("claude-code catalog missing tier models")
	}
	setSessionAllowlist(t, "claude-code", []string{smallID, frontierID})

	// --- 1. PreToolUse Shell Compactor ---
	t.Run("1_ShellCompactorRewrite", func(t *testing.T) {
		withExec(t, true, nil, true)
		cwd := userSettings(t, okRules, nil, nil)

		var rc int
		out := captureStdout(func() {
			rc = harnessRegistry["claude-code"].Hook(strings.NewReader(bashPayload(t, "go test ./...", cwd)), cmdCat)
		})
		if rc != 0 {
			t.Fatalf("runHookAdapter(Bash) rc = %d", rc)
		}

		if !strings.Contains(out, "compact") || !strings.Contains(out, "go test") {
			t.Errorf("expected command rewrite with compact, got output: %s", out)
		}
	})

	// --- 2. Subagent Model Downshifting ---
	taskPrompt := "[tier=small] rename the helper function formatTime to formatTimestamp"
	t.Run("2_SubagentRoutingDownshift", func(t *testing.T) {
		ev := runClaudeCodeHook(t, eventLog, map[string]any{
			"hook_event_name": "PreToolUse",
			"tool_name":       "Agent",
			"session_id":      sessionID,
			"model":           frontierID,
			"tool_input": map[string]any{
				"subagent_type": "general-purpose",
				"prompt":        taskPrompt,
				"model":         frontierID,
			},
		})

		if ev["final_model"] != smallID {
			t.Errorf("final_model = %v, want small tier model %s", ev["final_model"], smallID)
		}
		if ev["verdict"] != "DOWNSHIFT" {
			t.Errorf("verdict = %v, want DOWNSHIFT", ev["verdict"])
		}
	})

	// --- 3. Sensor-guided Auto-Upgrade Cascade ---
	t.Run("3_AutoUpgradeCascadeOnFailure", func(t *testing.T) {
		// Record sensor failure for this task
		cascade.RecordFailure(sessionID, taskPrompt, smallID, "go test failed with exit code 1")

		// Re-run the hook: should upgrade back to frontier model
		ev := runClaudeCodeHook(t, eventLog, map[string]any{
			"hook_event_name": "PreToolUse",
			"tool_name":       "Agent",
			"session_id":      sessionID,
			"model":           frontierID,
			"tool_input": map[string]any{
				"subagent_type": "general-purpose",
				"prompt":        taskPrompt,
				"model":         frontierID,
			},
		})

		if ev["verdict"] != "UPSHIFT" {
			t.Errorf("verdict = %v, want UPSHIFT after failure", ev["verdict"])
		}
		cascade.ClearSession(sessionID)
	})

	// --- 4. SubagentStop Token Accounting ---
	t.Run("4_SubagentStopAccounting", func(t *testing.T) {
		stopPayload, err := json.Marshal(map[string]any{
			"hook_event_name": "SubagentStop",
			"session_id":      sessionID,
			"agent_id":        "subagent_worker_1",
			"input_tokens":    1200,
			"output_tokens":   350,
			"cached_tokens":   8000,
		})
		if err != nil {
			t.Fatal(err)
		}

		captureStdout(func() {
			runUsage(bytes.NewReader(stopPayload), portFor("claude-code"), cmdCat)
		})

		events, err := telemetry.ReadEventsFrom(eventLog)
		if err != nil {
			t.Fatalf("cannot read event log: %v", err)
		}
		foundUsage := false
		for _, e := range events {
			if e.Outcome == "usage" {
				foundUsage = true
				break
			}
		}
		if !foundUsage {
			t.Errorf("no usage event recorded on SubagentStop in %d events", len(events))
		}
	})

	// --- 5. Git Commit Acceptance ---
	t.Run("5_GitCommitAcceptance", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		rc := runAccept([]string{"--session=" + sessionID, "--used"}, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("runAccept() = %d; stderr: %s", rc, stderr.String())
		}

		events, err := telemetry.ReadEventsFrom(eventLog)
		if err != nil {
			t.Fatalf("cannot read event log: %v", err)
		}
		foundAcceptance := false
		for _, e := range events {
			if e.Outcome == telemetry.OutcomeAcceptance && e.Acceptance == "used" {
				foundAcceptance = true
				break
			}
		}
		if !foundAcceptance {
			t.Errorf("no acceptance event found in events log")
		}
	})
}
