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
	"time"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// runClaudeCodeSpawn is runClaudeCodeHook that also returns what the hook wrote
// to stdout, so a test can tell a rewrite from a held spawn.
func runClaudeCodeSpawn(t *testing.T, eventLog string, payload map[string]any) (map[string]any, string) {
	t.Helper()
	if err := os.WriteFile(eventLog, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stdin, _ := json.Marshal(payload)
	out := captureStdout(func() {
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
		t.Fatalf("exactly one event expected, got %q", raw)
	}
	return ev, out
}

// A subagent that spawns another subagent (nested spawn). Claude Code puts a
// top-level agent_id (and agent_type) on the hook input of a call made inside a
// subagent, but session_id and transcript_path stay the main session's. The
// transcript therefore names the main model, not the model subagent A runs on,
// and a spawn that inherits (names no model) inherits A's model, which the hook
// cannot see. The production decider runs (the orchestrator's [tier=small] hint),
// so the guard that holds an unknown baseline is the one under test. Owner rules: doubt keeps the requested model, and no target may
// sit above it. So a nested inheriting spawn is held with an unknown baseline,
// and the main model is never recorded as the nested spawn's parent.
//
// Live check, Claude Code 2.1.319: a subagent has no Agent tool, even when its
// definition lists one, so this payload could not be captured; it follows the
// documented hook input. See docs/reference/harness-matrix.md.
func TestHookE2E_ClaudeCodeNestedSpawn(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("ANTHROPIC_MODEL", "")
	t.Setenv("CLAUDE_CODE_SUBAGENT_MODEL", "")
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	// A call id the main transcript does not hold waits for it; keep that short.
	total, step := hookutil.TranscriptWaitTotal, hookutil.TranscriptWaitStep
	hookutil.TranscriptWaitTotal, hookutil.TranscriptWaitStep = 30*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { hookutil.TranscriptWaitTotal, hookutil.TranscriptWaitStep = total, step })

	smallID := cmdCat.ModelFor("claude-code", core.TierSmall).ID
	midID := cmdCat.ModelFor("claude-code", core.TierMid).ID
	frontierID := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	if smallID == "" || midID == "" || frontierID == "" {
		t.Fatal("claude-code catalog missing tier models")
	}
	setSessionAllowlist(t, "claude-code", []string{smallID, midID, frontierID})

	// The main session ran on the frontier model. Subagent A's own lines live in
	// A's transcript, which is not the one the hook is given.
	transcript := filepath.Join(t.TempDir(), "main.jsonl")
	line := `{"type":"assistant","message":{"model":"` + frontierID + `","content":[]}}` + "\n"
	if err := os.WriteFile(transcript, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	spawn := func(top map[string]any, model string) map[string]any {
		ti := map[string]any{"subagent_type": "general-purpose", "prompt": "[tier=small] rename the variable userId to userIdentifier"}
		if model != "" {
			ti["model"] = model
		}
		p := map[string]any{
			"hook_event_name": "PreToolUse", "tool_name": "Agent", "session_id": "sess-main",
			"transcript_path": transcript, "cwd": t.TempDir(), "tool_use_id": "toolu_spawn", "tool_input": ti,
		}
		for k, v := range top {
			p[k] = v
		}
		return p
	}
	nested := map[string]any{"agent_id": "agent-a", "agent_type": "general-purpose"}

	cases := []struct {
		name       string
		payload    map[string]any
		verdict    string
		requested  string // requested_model in the event
		rewritten  bool   // the hook emitted an updatedInput
		parent     string // parent_model, "" means the field must be absent
		parentFrom string
	}{
		{
			name:    "main session spawn inherits the session model",
			payload: spawn(nil, ""), verdict: "DOWNSHIFT", requested: frontierID, rewritten: true,
			parent: frontierID, parentFrom: "transcript",
		},
		{
			name:    "empty agent_id is the main session",
			payload: spawn(map[string]any{"agent_id": ""}, ""), verdict: "DOWNSHIFT", requested: frontierID, rewritten: true,
			parent: frontierID, parentFrom: "transcript",
		},
		{
			name:    "nested spawn inherits a model the hook cannot see",
			payload: spawn(nested, ""), verdict: "UNKNOWN", requested: "unknown", rewritten: false,
		},
		{
			name:    "nested spawn that names its model is routed on that model",
			payload: spawn(nested, frontierID), verdict: "DOWNSHIFT", requested: frontierID, rewritten: true,
		},
		{
			name:    "nested spawn with a parent model the mod reports keeps that fact",
			payload: spawn(map[string]any{"agent_id": "agent-a", "entry": "mod", "parent_model": midID}, ""),
			verdict: "DOWNSHIFT", requested: midID, rewritten: true, parent: midID, parentFrom: "mod",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, out := runClaudeCodeSpawn(t, eventLog, tc.payload)
			if ev["verdict"] != tc.verdict {
				t.Errorf("verdict = %v, want %s: %v", ev["verdict"], tc.verdict, ev)
			}
			if ev["requested_model"] != tc.requested {
				t.Errorf("requested_model = %v, want %s", ev["requested_model"], tc.requested)
			}
			if got := strings.Contains(out, "updatedInput"); got != tc.rewritten {
				t.Errorf("rewrite emitted = %v, want %v (stdout %q)", got, tc.rewritten, out)
			}
			if got, _ := ev["parent_model"].(string); got != tc.parent {
				t.Errorf("parent_model = %q, want %q", got, tc.parent)
			}
			if got, _ := ev["parent_model_source"].(string); got != tc.parentFrom {
				t.Errorf("parent_model_source = %q, want %q", got, tc.parentFrom)
			}
			// The target is never above the model the spawn asked for.
			if tc.rewritten && tc.requested != "unknown" {
				if rank(ev["final_model"]) > rank(ev["requested_model"]) {
					t.Errorf("final_model %v is above requested_model %v", ev["final_model"], ev["requested_model"])
				}
			}
		})
	}
}

// rank orders the claude-code catalog tiers; an unknown id ranks below all.
func rank(id any) int {
	s, _ := id.(string)
	switch s {
	case cmdCat.ModelFor("claude-code", core.TierFrontier).ID:
		return 3
	case cmdCat.ModelFor("claude-code", core.TierMid).ID:
		return 2
	case cmdCat.ModelFor("claude-code", core.TierSmall).ID:
		return 1
	}
	return 0
}
