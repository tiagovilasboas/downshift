// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
)

// A Claude Code Task call without "model" runs on the session model. The hook
// reads it from the parent transcript, so the decision has a verdict instead
// of UNKNOWN and telemetry records the model the child would have used.
func TestHookE2E_ClaudeCodeInheritedSessionModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	smallID := cmdCat.ModelFor("claude-code", core.TierSmall).ID
	midID := cmdCat.ModelFor("claude-code", core.TierMid).ID
	frontierID := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	setSessionAllowlist(t, "claude-code", []string{smallID, midID, frontierID})

	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	rows := `{"type":"user","message":{"content":"hi"}}
{"type":"assistant","message":{"model":"` + frontierID + `","content":[]}}
{"type":"assistant","message":{"model":"<synthetic>","content":[]}}
`
	if err := os.WriteFile(transcript, []byte(rows), 0o600); err != nil {
		t.Fatal(err)
	}

	run := func(toolInput map[string]any) map[string]any {
		t.Helper()
		if err := os.WriteFile(eventLog, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		stdin, _ := json.Marshal(map[string]any{
			"hook_event_name": "PreToolUse",
			"tool_name":       "Agent",
			"transcript_path": transcript,
			"tool_input":      toolInput,
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
		raw, err := os.ReadFile(eventLog)
		if err != nil {
			t.Fatal(err)
		}
		var ev map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(raw), &ev); err != nil {
			t.Fatalf("one decision event expected, got %q: %v", raw, err)
		}
		return ev
	}

	ev := run(map[string]any{"prompt": "rename the variable userId to userIdentifier", "subagent_type": "general-purpose"})
	if ev["requested_model"] != frontierID {
		t.Errorf("requested_model = %v, want inherited %s", ev["requested_model"], frontierID)
	}
	if ev["verdict"] == "UNKNOWN" {
		t.Errorf("verdict stayed UNKNOWN with a known session model: %v", ev)
	}

	// The built-in Explore and Plan run on the session model, so they inherit it
	// and are routed like general-purpose (they were left unknown until 2026-10-15).
	for _, typ := range []string{"Explore", "Plan"} {
		ev = run(map[string]any{"prompt": "rename the variable userId to userIdentifier", "subagent_type": typ})
		if ev["requested_model"] != frontierID {
			t.Errorf("%s requested_model = %v, want the inherited %s", typ, ev["requested_model"], frontierID)
		}
		if ev["verdict"] == "UNKNOWN" {
			t.Errorf("%s verdict stayed UNKNOWN with a known session model: %v", typ, ev)
		}
	}

	// A custom agent type may pin a model in its definition: no guess.
	ev = run(map[string]any{"prompt": "rename the variable userId to userIdentifier", "subagent_type": "my-custom-reviewer"})
	if ev["requested_model"] == frontierID {
		t.Errorf("a custom agent type must not inherit the parent model guess: %v", ev)
	}
}
