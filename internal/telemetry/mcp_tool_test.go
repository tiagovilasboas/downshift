// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"testing"
)

func TestMCPToolEventHasNoText(t *testing.T) {
	ev := MCPTool("delegate", "cap", ToolCounts{Tasks: 2, Write: 1, GatewayRouted: 1})
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"correlation_id": true, "timestamp": true, "source": true, "agent": true,
		"outcome": true, "error_code": true, "tool": true, "complexity": true, "requested_model": true,
		"final_model": true, "requested_reasoning_effort": true, "final_reasoning_effort": true,
		"harness": true, "verdict": true, "tier": true, "policy_version": true, "binary_version": true}
	for k := range m {
		if !allowed[k] {
			t.Errorf("unexpected key %q in %s", k, raw)
		}
	}
	if m["agent"] != "delegate" || m["error_code"] != "cap" || m["outcome"] != "mcp_tool" {
		t.Errorf("fields: %s", raw)
	}
	tool, _ := m["tool"].(map[string]any)
	for k, v := range tool {
		if _, ok := v.(float64); !ok {
			t.Errorf("tool.%s is not a number: %v", k, v)
		}
	}
}

func TestAggregateSkipsMCPTool(t *testing.T) {
	ev := MCPTool("plan", "", ToolCounts{Steps: 3, SpawnSteps: 3})
	s := Aggregate([]Event{ev})
	if s.Total != 0 {
		t.Errorf("Total = %d, want 0", s.Total)
	}
	if !IsObservationOutcome(OutcomeMCPTool) {
		t.Error("mcp_tool must be an observation outcome")
	}
}
