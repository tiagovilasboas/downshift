// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/codeintel"
)

// userPromptSubmitInput matches the UserPromptSubmit hook schema across supported harnesses.
type userPromptSubmitInput struct {
	HookEventName      string `json:"hook_event_name"`
	HookEventNameCamel string `json:"hookEventName"`
	Cwd                string `json:"cwd"`
	SessionID          string `json:"session_id"`
	SessionIDCamel     string `json:"sessionId"`
	Prompt             string `json:"prompt"`
	Task               string `json:"task"`
	UserMessage        string `json:"user_message"`
}

func (u *userPromptSubmitInput) EventName() string {
	if u.HookEventName != "" {
		return u.HookEventName
	}
	return u.HookEventNameCamel
}

func (u *userPromptSubmitInput) PromptText() string {
	if u.Prompt != "" {
		return u.Prompt
	}
	if u.Task != "" {
		return u.Task
	}
	return u.UserMessage
}

// userPromptSubmitOutput matches the Claude Code UserPromptSubmit response schema
// and optionally includes top-level additionalContext for multi-harness compatibility.
type userPromptSubmitOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
	AdditionalContext  string             `json:"additionalContext,omitempty"`
}

type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

// runDCIHook executes DCI hook handling for the specified harness.
// It enforces fail-open semantics: errors, timeouts, or abstentions always exit 0.
func runDCIHook(harness string, r io.Reader, w io.Writer, stderr io.Writer) int {
	defer func() {
		// Recover from unexpected panics to guarantee fail-open
		_ = recover()
	}()

	switch harness {
	case "claude-code", "cursor", "opencode", "codex", "generic":
	default:
		return 0 // Unknown harness: fail-open
	}

	if r == nil {
		return 0
	}

	data, err := io.ReadAll(io.LimitReader(r, 1<<20))
	if err != nil || len(data) == 0 {
		return 0 // Empty or unreadable stdin: fail-open
	}

	var input userPromptSubmitInput
	if err := json.Unmarshal(data, &input); err != nil {
		return 0 // Invalid JSON: fail-open
	}

	prompt := input.PromptText()
	if prompt == "" {
		return 0
	}

	repoRoot := input.Cwd
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}

	cfg, err := codeintel.LoadConfig(repoRoot)
	if err != nil || cfg.Mode == codeintel.ModeOff {
		return 0 // Disabled or error loading config: fail-open
	}

	// Budget enforcement: hard 2500ms deadline (fail-open)
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	service := codeintel.NewService(repoRoot, cfg)
	pack, telemetry := service.ProcessPrompt(ctx, prompt)
	_ = codeintel.RecordTelemetry(telemetry)

	if pack == nil || pack.OutputBytes == 0 || !telemetry.InjectionApplied {
		return 0 // Abstained, observe mode, or empty context: exit 0
	}

	out := userPromptSubmitOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:     "UserPromptSubmit",
			AdditionalContext: pack.FormattedText,
		},
	}
	if harness != "claude-code" {
		out.AdditionalContext = pack.FormattedText
	}

	enc := json.NewEncoder(w)
	_ = enc.Encode(out)
	return 0
}
