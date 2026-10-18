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

// Two subagents spawned in parallel in one session. The first call's
// PostToolUse arrives after the second decision was logged. Linking by
// tool_use_id must attach the launch and the final cost to the first
// decision, not to the latest sibling.
func TestHookE2E_ParallelSpawnsLinkByToolUseID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	smallID := cmdCat.ModelFor("claude-code", core.TierSmall).ID
	midID := cmdCat.ModelFor("claude-code", core.TierMid).ID
	frontierID := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	setSessionAllowlist(t, "claude-code", []string{smallID, midID, frontierID})

	spawn := func(toolUseID, prompt string) {
		stdin, _ := json.Marshal(map[string]any{
			"hook_event_name": "PreToolUse", "tool_name": "Agent", "session_id": "sess-par",
			"tool_use_id": toolUseID,
			"tool_input":  map[string]any{"prompt": prompt, "model": frontierID},
		})
		captureStdout(func() {
			runHookAdapter(
				"claude-code",
				bytes.NewReader(stdin),
				func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
				func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, cmdCat) },
				printAllow,
			)
		})
	}
	spawn("toolu_A", "rename the variable userId to userIdentifier")
	spawn("toolu_B", "rename the variable orderId to orderIdentifier")

	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil || len(events) != 2 {
		t.Fatalf("want 2 decisions, got %d (%v)", len(events), err)
	}
	decA, decB := events[0], events[1]
	if decA.ToolUseHash == "" || decA.ToolUseHash == decB.ToolUseHash {
		t.Fatalf("decisions must carry distinct tool-use hashes: %q %q", decA.ToolUseHash, decB.ToolUseHash)
	}
	if !telemetry.IsAppliedShift(decA) || !telemetry.IsAppliedShift(decB) {
		t.Fatalf("both spawns should be applied rewrites: %+v %+v", decA, decB)
	}

	// A's launch reports late, after B's decision.
	launch, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse", "session_id": "sess-par", "tool_name": "Agent",
		"tool_use_id":   "toolu_A",
		"tool_response": map[string]any{"isAsync": true, "status": "async_launched", "agentId": "agent-A", "resolvedModel": smallID},
	})
	claudecode.HandlePostToolUse(launch, "test", cmdCat)

	transcript := filepath.Join(t.TempDir(), "agent-A.jsonl")
	row := `{"type":"assistant","message":{"id":"m1","model":"` + smallID + `","usage":{"input_tokens":10,"output_tokens":20}}}` + "\n"
	if err := os.WriteFile(transcript, []byte(row), 0o600); err != nil {
		t.Fatal(err)
	}
	stop, _ := json.Marshal(map[string]any{
		"hook_event_name": "SubagentStop", "session_id": "sess-par",
		"agent_id": "agent-A", "agent_transcript_path": transcript,
	})
	claudecode.HandleSubagentStop(stop, "test", cmdCat)

	events, _ = telemetry.ReadEventsFrom(eventLog)
	wantKey := telemetry.DecisionKey(decA)
	seen := map[string]bool{}
	for _, ev := range events[2:] {
		seen[ev.Outcome] = true
		if ev.LinkedDecision != wantKey {
			t.Errorf("%s linked to %q, want decision A %q (B is %q)", ev.Outcome, ev.LinkedDecision, wantKey, telemetry.DecisionKey(decB))
		}
	}
	for _, outcome := range []string{telemetry.OutcomeResolved, telemetry.OutcomeChildEnded, telemetry.OutcomeUsage} {
		if !seen[outcome] {
			t.Errorf("missing %s record; got %v", outcome, seen)
		}
	}
}
