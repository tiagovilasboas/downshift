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
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/childend"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/sensor"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// SessionIdentifier returns the raw session id supplied by the harness, if
// any. runHookAdapter hashes it via telemetry.HashSessionID before durable
// logging, so future PostToolUse records can link by session.
//
// Some live Bash PreToolUse payloads carry no session_id. Claude Code names
// each transcript file after the session uuid (<uuid>.jsonl), so when
// session_id is absent the id is derived from transcript_path, and only when
// that file name has a uuid shape. session_id always wins when present.
func (ev Event) SessionIdentifier() string {
	if ev.SessionID != "" {
		return ev.SessionID
	}
	return sessionFromTranscript(ev.TranscriptPath)
}

// sessionFromTranscript returns the uuid file name of a transcript path, or ""
// when the path is empty or the base name is not a uuid.
func sessionFromTranscript(path string) string {
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	id := strings.TrimSuffix(base, filepath.Ext(base))
	if !isUUID(id) {
		return ""
	}
	return id
}

// isUUID reports whether s is a canonical 8-4-4-4-12 hexadecimal uuid.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case i == 8 || i == 13 || i == 18 || i == 23:
			if c != '-' {
				return false
			}
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

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
	ce.ServedModel = normalizeServedModel(ce.ServedModel, r)
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

// normalizeServedModel maps a served model read from a child transcript to its
// catalog id (a dated id such as claude-haiku-5-5-20251001 to claude-haiku-5-5)
// so the child_ended observation names the same id the hook wrote. It returns
// the raw value unchanged when there is no resolver or the catalog does not
// know the model, and "" unchanged; the shared recorder still bounds it.
func normalizeServedModel(raw string, r core.Resolver) string {
	if raw == "" || r == nil {
		return raw
	}
	if m, ok := r.LookupByID(harnessID, raw); ok {
		return m.ID
	}
	return raw
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
		outputBytes = toolOutputBytes(resp)
		if status, ok := resp["status"].(string); ok && (status == "error" || status == "failed") {
			isError = true
		}
		if code, ok := exitCodeField(resp); ok {
			exitCode = code
		}
	}
	_ = st.RecordToolResult(sessionHash, harnessID, toolName, outputBytes, exitCode, isError)
}

// toolOutputBytes is the text the model reads from one tool result, in the
// shapes Claude Code sends: Bash {stdout, stderr}, a plain content string, an
// array of content blocks ({type:"text", text}), or a generic output string.
// Only the length and the observe-mode compressor ever see it; nothing here is
// stored. Unknown shapes give nil, which records zero bytes as before.
func toolOutputBytes(resp map[string]any) []byte {
	var b []byte
	if out, ok := resp["stdout"].(string); ok {
		b = append(b, out...)
	}
	if errOut, ok := resp["stderr"].(string); ok && errOut != "" {
		if len(b) > 0 && b[len(b)-1] != '\n' {
			b = append(b, '\n')
		}
		b = append(b, errOut...)
	}
	if b != nil {
		return b
	}
	switch c := resp["content"].(type) {
	case string:
		return []byte(c)
	case []any:
		for _, block := range c {
			if m, ok := block.(map[string]any); ok {
				if text, ok := m["text"].(string); ok {
					b = append(b, text...)
				}
			}
		}
		return b
	}
	if out, ok := resp["output"].(string); ok {
		return []byte(out)
	}
	return nil
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
