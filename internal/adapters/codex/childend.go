// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/hookutil"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// ParseChildEnd is Codex's side of the shared child-end contract. It reads the
// PostToolUse payload of a spawn_agent call (the same matcher as the
// PreToolUse hook): the tool_response status and child model, and the token
// usage from the rollout transcript at transcript_path.
//
// Unverified: no real Codex PostToolUse payload for spawn_agent was captured
// (docs/reference/harness-matrix.md). The fields are tool_use_id, session_id, model,
// transcript_path (documented common fields) and tool_response.status/model
// (assumed names). A spawn_agent call that reports no status, or one outside
// the shared vocabulary, is a launch (telemetry.ChildLaunched), not an end:
// the call returned, and nothing says the child finished. Then no usage is read: the transcript at transcript_path belongs
// to the parent thread and says nothing about a child that has not finished.
// Parsing only: records, pricing and the child-end line are the shared
// recorder's job (internal/childend).
func ParseChildEnd(raw []byte) (hookport.ChildEnd, bool) {
	var p struct {
		HookEventName  string          `json:"hook_event_name"`
		SessionID      string          `json:"session_id"`
		ToolName       string          `json:"tool_name"`
		ToolUseID      string          `json:"tool_use_id"`
		TranscriptPath string          `json:"transcript_path"`
		ToolResponse   json.RawMessage `json:"tool_response"`
	}
	if json.Unmarshal(raw, &p) != nil {
		return hookport.ChildEnd{}, false
	}
	if p.HookEventName != "" && p.HookEventName != "PostToolUse" {
		return hookport.ChildEnd{}, false
	}
	if p.ToolName != "Agent" && !strings.HasSuffix(p.ToolName, "spawn_agent") {
		return hookport.ChildEnd{}, false
	}
	var resp map[string]any
	_ = json.Unmarshal(p.ToolResponse, &resp) // a non-object response reads as empty
	ce := hookport.ChildEnd{
		Harness:       harnessID,
		AgentKey:      p.ToolUseID,
		ToolUseID:     p.ToolUseID,
		SessionID:     p.SessionID,
		ResolvedModel: hookutil.StringField(resp, "model"),
		Status:        telemetry.ChildStatusFromText(hookutil.StringField(resp, "status"), telemetry.ChildLaunched),
		Source:        hookport.SourcePayload,
		IsChild:       true,
	}
	ce.Model = ce.ResolvedModel
	if ce.Status != telemetry.ChildLaunched {
		if u, ok := rolloutUsage(p.TranscriptPath); ok {
			ce.Usage = &u
		}
	}
	return ce, true
}

// rolloutUsage reads the token counts of the last token_count event in the
// bounded tail of a Codex rollout (JSONL). Only numbers are kept. Codex counts
// input_tokens including cached_input_tokens, while the shared TokenUsage keeps
// them apart, so the cached part is subtracted from the input.
func rolloutUsage(path string) (telemetry.TokenUsage, bool) {
	s := hookutil.LastInTranscriptTail(path, tokenCountLine)
	if s == "" {
		return telemetry.TokenUsage{}, false
	}
	var in, cached, out int64
	if _, err := fmt.Sscanf(s, "%d %d %d", &in, &cached, &out); err != nil {
		return telemetry.TokenUsage{}, false
	}
	if cached > in {
		cached = in
	}
	u := telemetry.TokenUsage{InputTokens: in - cached, OutputTokens: out, CachedTokens: cached}
	return u, u != (telemetry.TokenUsage{})
}

// tokenCountLine returns "input cached output" for an event_msg/token_count
// line that carries info.total_token_usage, else "".
func tokenCountLine(line []byte) string {
	if !bytes.Contains(line, []byte(`"token_count"`)) {
		return ""
	}
	var row struct {
		Type    string `json:"type"`
		Payload struct {
			Type string `json:"type"`
			Info *struct {
				Total *struct {
					Input  int64 `json:"input_tokens"`
					Cached int64 `json:"cached_input_tokens"`
					Output int64 `json:"output_tokens"`
				} `json:"total_token_usage"`
			} `json:"info"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &row) != nil || row.Type != "event_msg" || row.Payload.Type != "token_count" ||
		row.Payload.Info == nil || row.Payload.Info.Total == nil {
		return ""
	}
	t := row.Payload.Info.Total
	if t.Input < 0 || t.Cached < 0 || t.Output < 0 || t.Input+t.Output == 0 {
		return ""
	}
	return fmt.Sprintf("%d %d %d", t.Input, t.Cached, t.Output)
}
