// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/kirocrew"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestParseChildEnd(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantOK  bool
		status  telemetry.ChildStatus
		model   string
	}{
		{
			name:    "spawn_run completed",
			payload: `{"hook_event_name":"PostToolUse","tool_name":"spawn_run","tool_use_id":"abc123","tool_input":{"model":"claude-haiku-4.5","task":"test"},"tool_response":{"status":"completed"}}`,
			wantOK:  true,
			status:  telemetry.ChildCompleted,
			model:   "claude-haiku-4.5",
		},
		{
			name:    "spawn_run with outcome field",
			payload: `{"hook_event_name":"postToolUse","tool_name":"spawn_run","tool_use_id":"def456","tool_input":{"model":"claude-sonnet-4.6"},"tool_response":{"outcome":"success"}}`,
			wantOK:  true,
			status:  telemetry.ChildCompleted,
			model:   "claude-sonnet-4.6",
		},
		{
			name:    "spawn_sub_agents error",
			payload: `{"hook_event_name":"PostToolUse","tool_name":"spawn_sub_agents","tool_use_id":"ghi789","tool_input":{"model":"claude-haiku-4.5"},"tool_response":{"status":"error"}}`,
			wantOK:  true,
			status:  telemetry.ChildError,
			model:   "claude-haiku-4.5",
		},
		{
			name:    "no status means launched",
			payload: `{"hook_event_name":"PostToolUse","tool_name":"spawn_run","tool_use_id":"jkl012","tool_input":{"model":"claude-haiku-4.5"},"tool_response":{}}`,
			wantOK:  true,
			status:  telemetry.ChildLaunched,
			model:   "claude-haiku-4.5",
		},
		{
			name:    "wrong event name rejected",
			payload: `{"hook_event_name":"PreToolUse","tool_name":"spawn_run","tool_use_id":"mno345","tool_input":{"model":"claude-haiku-4.5"}}`,
			wantOK:  false,
		},
		{
			name:    "non-spawn tool rejected",
			payload: `{"hook_event_name":"PostToolUse","tool_name":"read_file","tool_use_id":"pqr678","tool_input":{}}`,
			wantOK:  false,
		},
		{
			name:    "empty event name accepted (implicit postToolUse)",
			payload: `{"tool_name":"spawn_run","tool_use_id":"stu901","tool_input":{"model":"claude-opus-4.5"},"tool_response":{"status":"completed"}}`,
			wantOK:  true,
			status:  telemetry.ChildCompleted,
			model:   "claude-opus-4.5",
		},
		{
			name:    "with usage data",
			payload: `{"hook_event_name":"PostToolUse","tool_name":"spawn_run","tool_use_id":"vwx234","tool_input":{"model":"claude-haiku-4.5"},"tool_response":{"status":"completed","usage":{"input_tokens":1000,"output_tokens":500}}}`,
			wantOK:  true,
			status:  telemetry.ChildCompleted,
			model:   "claude-haiku-4.5",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ce, ok := kirocrew.ParseChildEnd([]byte(c.payload))
			if ok != c.wantOK {
				t.Fatalf("ParseChildEnd() ok = %v, want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if ce.Status != c.status {
				t.Errorf("Status = %q, want %q", ce.Status, c.status)
			}
			if ce.Model != c.model {
				t.Errorf("Model = %q, want %q", ce.Model, c.model)
			}
			if ce.Harness != "kirocrew" {
				t.Errorf("Harness = %q, want kirocrew", ce.Harness)
			}
		})
	}
}

func TestParseChildEndUsage(t *testing.T) {
	payload := `{"hook_event_name":"PostToolUse","tool_name":"spawn_run","tool_use_id":"usage-test","tool_input":{"model":"claude-haiku-4.5"},"tool_response":{"status":"completed","usage":{"input_tokens":1500,"output_tokens":800,"cached_tokens":200}}}`
	ce, ok := kirocrew.ParseChildEnd([]byte(payload))
	if !ok {
		t.Fatal("ParseChildEnd returned false")
	}
	if ce.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if ce.Usage.InputTokens != 1500 {
		t.Errorf("InputTokens = %d, want 1500", ce.Usage.InputTokens)
	}
	if ce.Usage.OutputTokens != 800 {
		t.Errorf("OutputTokens = %d, want 800", ce.Usage.OutputTokens)
	}
	if ce.Usage.CachedTokens != 200 {
		t.Errorf("CachedTokens = %d, want 200", ce.Usage.CachedTokens)
	}
}
