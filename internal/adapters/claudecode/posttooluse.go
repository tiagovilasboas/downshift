// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package claudecode — PostToolUse payload parsing (v1).
//
// A Claude Code PostToolUse hook fires when the Task/Agent tool returns. For an
// async subagent that is the LAUNCH, not the completion (captured 2026-10-06:
// tool_response is {isAsync, status, agentId, resolvedModel, ...}, duration_ms
// a few ms), so the payload carries no token usage and the "usage" path stays
// idle until a harness reports tokens. What it does carry is
// tool_response.resolvedModel, the model the harness chose for the child.
// Token usage is still parsed defensively (see telemetry.TokenUsageFromPayload).
//
// This file only parses. Linking to the decision, pricing, dedup and the
// child-end line are shared by every harness in internal/childend; the adapter
// hands it a hookport.ChildEnd (see childend.go in this package).
package claudecode

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift/internal/childend"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/sensor"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// SessionIdentifier returns the raw session id supplied by the harness, if
// any. runHookAdapter hashes it via telemetry.HashSessionID before durable
// logging, so future PostToolUse records can link by session.
func (ev Event) SessionIdentifier() string { return ev.SessionID }

// stringFieldAny reads a string value from a decoded payload map.
func stringFieldAny(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// HandlePostToolUse processes one Claude Code PostToolUse stdin payload: it
// parses it into the shared hookport.ChildEnd contract, lets childend.Record
// write the resolved, child_ended and usage records (linked to the matching
// decision), and returns the neutral PostToolUse response ({}; this hook cannot
// steer, so there is nothing to rewrite), carrying the child-end line when the
// call was a child that reported tokens.
//
// Fail-open: any parse, lookup, or write failure returns the neutral
// response with linked=false. The spawn already completed, so nothing is
// ever blocked; the boolean only reports whether a decision was linked.
func HandlePostToolUse(raw []byte, binaryVersion string, r core.Resolver) (Output, string, bool) {
	neutral := Output{}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return neutral, "", false
	}
	// Context Sensor observation: track tool execution volume safely without prompt text
	recordSensorToolObservation(payload, telemetry.HashSessionID(stringFieldAny(payload, "session_id", "sessionId", "sessionID")))

	ce, ok := parsePostToolUse(payload)
	if !ok {
		return neutral, "", false
	}
	res := childend.Record(ce, binaryVersion, r)
	if res.Line != "" && telemetry.ChildLineEnabled(core.ClaudeCodeCaps.CanShowChildLine) {
		neutral.SystemMessage = res.Line
	}
	return neutral, res.Note, res.Linked
}

// agentID reads the child's agent id from tool_response.
func agentID(payload map[string]any) string {
	resp, _ := payload["tool_response"].(map[string]any)
	return stringFieldAny(resp, "agentId", "agent_id")
}

// resolvedModel reads the model the harness chose for the spawn. Claude Code
// reports it as tool_response.resolvedModel when it launches a subagent; this
// is the only post-spawn model signal the hook receives (the launch payload
// carries no token usage for async subagents).
func resolvedModel(payload map[string]any) string {
	resp, _ := payload["tool_response"].(map[string]any)
	return stringFieldAny(resp, "resolvedModel", "resolved_model")
}

func recordSensorToolObservation(payload map[string]any, sessionHash string) {
	if sessionHash == "" || !nativeContextObserveOn() {
		return
	}
	st, err := sensor.DefaultStore()
	if err != nil {
		return
	}
	toolName := stringFieldAny(payload, "tool_name", "toolName")
	if toolName == "" {
		toolName = "tool"
	}

	var outputBytes []byte
	isError := false
	exitCode := -1
	if resp, ok := payload["tool_response"].(map[string]any); ok {
		if content, ok := resp["content"].(string); ok {
			outputBytes = []byte(content)
		}
		if status, ok := resp["status"].(string); ok && (status == "error" || status == "failed") {
			isError = true
		}
		if code, ok := exitCodeField(resp); ok {
			exitCode = code
		}
	}
	_ = st.RecordToolResult(sessionHash, harnessID, toolName, outputBytes, exitCode, isError)
}

// nativeContextObserveOn is the persisted native-provider flag. Missing
// config uses the default (enabled, provider native). A read error fails
// closed. The stored record is observe-mode only; this hook does not
// replace the tool result the model sees.
func nativeContextObserveOn() bool {
	cfg, err := contextopt.LoadConfig()
	if err != nil || !cfg.Enabled {
		return false
	}
	return cfg.Provider == "" || cfg.Provider == "native"
}

func exitCodeField(resp map[string]any) (int, bool) {
	for _, key := range []string{"exit_code", "exitCode"} {
		n, ok := resp[key].(float64)
		if !ok || n < 0 || n > 255 || n != float64(int(n)) {
			continue
		}
		return int(n), true
	}
	return -1, false
}
