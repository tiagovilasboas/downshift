// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// A Codex spawn_agent call without "model" runs on the parent's current model.
// The hook reads it from the rollout's last turn_context, so requested_model
// is that model instead of unknown. An explicit model arg still wins.
func TestHookE2E_CodexInheritedSessionModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	smallID := cmdCat.ModelFor("codex", core.TierSmall).ID
	frontierID := cmdCat.ModelFor("codex", core.TierFrontier).ID

	rollout := filepath.Join(t.TempDir(), "rollout.jsonl")
	rows := `{"type":"session_meta","payload":{"id":"x"}}
{"type":"turn_context","payload":{"model":"` + smallID + `"}}
{"type":"response_item","payload":{"type":"message"}}
{"type":"turn_context","payload":{"model":"` + frontierID + `"}}
`
	if err := os.WriteFile(rollout, []byte(rows), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(toolInput map[string]any) any {
		t.Helper()
		if err := os.WriteFile(eventLog, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		stdin, _ := json.Marshal(map[string]any{
			"hook_event_name": "PreToolUse", "tool_name": "spawn_agent", "session_id": "s",
			"transcript_path": rollout, "tool_input": toolInput,
		})
		captureStdout(func() {
			runHookAdapter(
				bytes.NewReader(stdin),
				func(b []byte) (codex.Event, error) { var e codex.Event; return e, json.Unmarshal(b, &e) },
				func(e codex.Event) (any, string, core.Decision) { return codex.Handle(e, cmdCat) },
				printAllow,
			)
		})
		raw, _ := os.ReadFile(eventLog)
		var ev map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(raw), &ev); err != nil {
			t.Fatalf("one decision event expected, got %q: %v", raw, err)
		}
		return ev["requested_model"]
	}

	if got := run(map[string]any{"message": "rename the variable userId", "agent_type": "worker"}); got != frontierID {
		t.Errorf("inherited: requested_model = %v, want %s", got, frontierID)
	}
	if got := run(map[string]any{"message": "rename the variable userId", "model": smallID}); got != smallID {
		t.Errorf("explicit: requested_model = %v, want %s", got, smallID)
	}
}
