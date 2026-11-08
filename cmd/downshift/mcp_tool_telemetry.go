// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/tiagovilasboas/downshift-ai/internal/mcp/tools"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// recordMCPToolCall writes one adoption record for a plan or delegate call:
// the tool name, the refusal code ("" when accepted) and whole-number counts.
// The MCP edge hands it to the tools as a callback, so the tools package never
// imports telemetry. Nothing but those fields is stored.
func recordMCPToolCall(tool, code string, c telemetry.ToolCounts) {
	telemetry.Record(telemetry.MCPTool(tool, code, c))
}

// recordMCPCall adapts the tools' callback to the telemetry record.
func recordMCPCall(tool, code string, c tools.CallCounts) {
	recordMCPToolCall(tool, code, telemetry.ToolCounts{
		Steps: c.Steps, DelegateSteps: c.DelegateSteps, SpawnSteps: c.SpawnSteps,
		Tasks: c.Tasks, Write: c.Write, GatewayRouted: c.GatewayRouted,
	})
}
