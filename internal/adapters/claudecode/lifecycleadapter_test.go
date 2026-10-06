// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package claudecode

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/lifecycleobserver"
)

// Fixture payloads match the documented Claude Code SubagentStart/SubagentStop
// hook schema (Claude Code hooks reference, Sep 2026).

func TestParseLifecycleEvent_SubagentStart(t *testing.T) {
	raw := []byte(`{
		"hook_event_name": "SubagentStart",
		"session_id":      "sess-abc123",
		"turn_id":         "turn-001",
		"agent_id":        "agent-xyz",
		"agent_type":      "claude-code"
	}`)
	ev, ok := ParseLifecycleEvent(raw)
	if !ok {
		t.Fatal("expected ok=true for valid SubagentStart")
	}
	if ev.HookEventName != "SubagentStart" {
		t.Errorf("HookEventName: got %q, want %q", ev.HookEventName, "SubagentStart")
	}
	if ev.SessionID != "sess-abc123" {
		t.Errorf("SessionID: got %q", ev.SessionID)
	}
	if ev.TurnID != "turn-001" {
		t.Errorf("TurnID: got %q", ev.TurnID)
	}
	if ev.AgentID != "agent-xyz" {
		t.Errorf("AgentID: got %q", ev.AgentID)
	}
	if ev.AgentType != "claude-code" {
		t.Errorf("AgentType: got %q", ev.AgentType)
	}
}

func TestParseLifecycleEvent_SubagentStop(t *testing.T) {
	raw := []byte(`{
		"hook_event_name": "SubagentStop",
		"session_id":      "sess-abc123",
		"turn_id":         "turn-001",
		"agent_id":        "agent-xyz",
		"agent_type":      "claude-code"
	}`)
	ev, ok := ParseLifecycleEvent(raw)
	if !ok {
		t.Fatal("expected ok=true for valid SubagentStop")
	}
	if ev.HookEventName != "SubagentStop" {
		t.Errorf("HookEventName: got %q", ev.HookEventName)
	}
}

// Extra fields present in a real PreToolUse payload must be silently ignored.
func TestParseLifecycleEvent_IgnoresExtraFields(t *testing.T) {
	raw := []byte(`{
		"hook_event_name": "SubagentStart",
		"session_id":      "sess-abc123",
		"turn_id":         "turn-001",
		"agent_id":        "agent-xyz",
		"agent_type":      "claude-code",
		"prompt":          "should be ignored",
		"model":           "claude-opus-5-5",
		"tool_input":      {"task": "ignored"},
		"transcript":      ["ignored"]
	}`)
	ev, ok := ParseLifecycleEvent(raw)
	if !ok {
		t.Fatal("expected ok=true; extra fields must be ignored")
	}
	// Verify the returned Event carries no prompt, model, transcript residue.
	// lifecycleobserver.Event has no such fields — the type itself enforces this.
	_ = ev
}

func TestParseLifecycleEvent_RejectsPreToolUse(t *testing.T) {
	raw := []byte(`{
		"hook_event_name": "PreToolUse",
		"session_id":      "sess-abc123",
		"turn_id":         "turn-001",
		"tool_name":       "Task",
		"tool_input":      {}
	}`)
	_, ok := ParseLifecycleEvent(raw)
	if ok {
		t.Error("expected ok=false for PreToolUse (not a lifecycle event this adapter handles)")
	}
}

func TestParseLifecycleEvent_RejectsMissingSessionID(t *testing.T) {
	raw := []byte(`{
		"hook_event_name": "SubagentStart",
		"turn_id":         "turn-001",
		"agent_id":        "agent-xyz",
		"agent_type":      "claude-code"
	}`)
	_, ok := ParseLifecycleEvent(raw)
	if ok {
		t.Error("expected ok=false when session_id is absent")
	}
}

func TestParseLifecycleEvent_RejectsMissingAgentID(t *testing.T) {
	raw := []byte(`{
		"hook_event_name": "SubagentStart",
		"session_id":      "sess-abc123",
		"turn_id":         "turn-001",
		"agent_type":      "claude-code"
	}`)
	_, ok := ParseLifecycleEvent(raw)
	if ok {
		t.Error("expected ok=false when agent_id is absent")
	}
}

func TestParseLifecycleEvent_RejectsMalformedJSON(t *testing.T) {
	_, ok := ParseLifecycleEvent([]byte(`not json`))
	if ok {
		t.Error("expected ok=false for malformed JSON")
	}
}

func TestParseLifecycleEvent_RejectsUnknownEventName(t *testing.T) {
	raw := []byte(`{
		"hook_event_name": "SessionStart",
		"session_id":      "sess-abc123",
		"turn_id":         "turn-001"
	}`)
	_, ok := ParseLifecycleEvent(raw)
	if ok {
		t.Error("expected ok=false for unknown hook_event_name")
	}
}

// Integration: parsed events feed directly into the observer without type conversion.
func TestParseLifecycleEvent_FeedsObserver(t *testing.T) {
	startRaw := []byte(`{
		"hook_event_name": "SubagentStart",
		"session_id": "sess-feed", "turn_id": "t1",
		"agent_id": "ag1", "agent_type": "claude-code"
	}`)
	stopRaw := []byte(`{
		"hook_event_name": "SubagentStop",
		"session_id": "sess-feed", "turn_id": "t1",
		"agent_id": "ag1", "agent_type": "claude-code"
	}`)
	startEv, ok1 := ParseLifecycleEvent(startRaw)
	stopEv, ok2 := ParseLifecycleEvent(stopRaw)
	if !ok1 || !ok2 {
		t.Fatal("failed to parse start/stop events")
	}
	// PreToolUse is NOT parsed by this adapter; build it directly as an observer event.
	preEv := lifecycleobserver.Event{
		HookEventName: "PreToolUse",
		SessionID:     "sess-feed",
		TurnID:        "t1",
		ToolName:      "Task",
	}

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	obs := lifecycleobserver.New(lifecycleobserver.Config{
		Enabled:    true,
		HMACKey:    key,
		ObserverID: "test-observer",
	})
	observations := obs.Observe([]lifecycleobserver.Event{preEv, startEv, stopEv})
	if len(observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(observations))
	}
	o := observations[0]
	if o.EvidenceState != "observed" {
		t.Errorf("EvidenceState: got %q, want %q", o.EvidenceState, "observed")
	}
	// Claude Code SubagentStart/Stop have no opaque routing correlation ID,
	// so association_state must be matched (session+turn+agent are deterministic).
	// NOTE: The observer currently uses "unavailable" when PreToolUse has no
	// matching start. With all three events, it should reach "completed".
	if o.Lifecycle != "completed" {
		t.Errorf("Lifecycle: got %q, want %q", o.Lifecycle, "completed")
	}
	// evidence_state must never claim executor_acknowledged.
	if o.EvidenceState == "executor_acknowledged" {
		t.Error("EvidenceState must never be executor_acknowledged")
	}
}
