// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
)

// TestCodexHook_NamespacedSpawnAgentWithoutCurrentModel exercises the actual
// hook executable rather than calling the adapter directly. Codex has emitted
// namespaced tool names in practice; an absent source model must still be
// routed, not silently allowed unchanged.
func TestCodexHook_NamespacedSpawnAgentWithoutCurrentModel(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "downshift")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build hook binary: %v\n%s", err, output)
	}

	event := []byte(`{
		"hook_event_name":"PreToolUse",
		"session_id":"codex-session-smoke",
		"tool_name":"collaboration.spawn_agent",
		"tool_input":{"task_name":"review_go","message":"list the Go files and do a code review without changing anything"}
	}`)
	cmd := exec.Command(binary, "codex")
	cmd.Stdin = bytes.NewReader(event)
	home := t.TempDir()
	modelConfig := filepath.Join(t.TempDir(), "session-models.json")
	if err := os.WriteFile(modelConfig, []byte(`{"codex":["gpt-6-luna","gpt-6-sol","gpt-5.6-luna","gpt-5.6-terra","gpt-5.6-sol"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd.Env = []string{"HOME=" + home, "DOWNSHIFT_SESSION_MODELS=" + modelConfig}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("run codex hook: %v", err)
	}
	output := stdout.Bytes()

	var response struct {
		HookSpecificOutput struct {
			HookEventName      string          `json:"hookEventName"`
			PermissionDecision string          `json:"permissionDecision"`
			UpdatedInput       json.RawMessage `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		t.Fatalf("decode hook response: %v\n%s", err, output)
	}
	if response.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName = %q, want PreToolUse", response.HookSpecificOutput.HookEventName)
	}
	if response.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("permissionDecision = %q, want allow", response.HookSpecificOutput.PermissionDecision)
	}

	var updated map[string]any
	if err := json.Unmarshal(response.HookSpecificOutput.UpdatedInput, &updated); err != nil {
		t.Fatalf("decode updatedInput: %v", err)
	}
	if updated["model"] != "gpt-6-sol" {
		t.Errorf("model = %v, want gpt-6-sol", updated["model"])
	}
	if updated["reasoning_effort"] != "high" {
		t.Errorf("reasoning_effort = %v, want high", updated["reasoning_effort"])
	}
	if !bytes.Contains(stderr.Bytes(), []byte("gpt-6-sol")) {
		t.Errorf("stderr = %q, want routing diagnostic for gpt-6-sol", stderr.String())
	}
	eventLog, err := os.ReadFile(filepath.Join(home, ".harness-downshift", "events.jsonl"))
	if err != nil {
		t.Fatalf("read hook telemetry: %v", err)
	}
	if !bytes.Contains(eventLog, []byte(`"session_id":"`+telemetry.HashSessionID("codex-session-smoke")+`"`)) {
		t.Errorf("telemetry did not identify the Codex session: %s", eventLog)
	}
}
