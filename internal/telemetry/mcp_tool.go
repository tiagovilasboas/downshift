// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import "time"

// OutcomeMCPTool marks one call of the MCP plan or delegate tool. It is an
// adoption observation, not a routing decision: Aggregate and RoutingTokens
// skip it, so it never moves decision, usage or savings counts.
const OutcomeMCPTool = "mcp_tool"

// ToolCounts are the whole-number facts of one tool call. Steps,
// DelegateSteps and SpawnSteps describe a plan; Tasks, Write and
// GatewayRouted describe a delegate call. Integers only: never task text, a
// directory or a file name.
type ToolCounts struct {
	Steps         int `json:"steps,omitempty"`
	DelegateSteps int `json:"delegate_steps,omitempty"`
	SpawnSteps    int `json:"spawn_steps,omitempty"`
	Tasks         int `json:"tasks,omitempty"`
	Write         int `json:"write,omitempty"`
	GatewayRouted int `json:"gateway_routed,omitempty"`
}

// MCPTool builds the record of one plan or delegate call. tool is "plan" or
// "delegate"; code is the refusal code (workers_off, write_off, cap, budget,
// strain, dir) or "" when the call was accepted.
func MCPTool(tool, code string, c ToolCounts) Event {
	return Event{
		CorrelationID: NewCorrelationID(),
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Source:        "mcp",
		Agent:         tool,
		Outcome:       OutcomeMCPTool,
		ErrorCode:     code,
		Tool:          &c,
	}
}
