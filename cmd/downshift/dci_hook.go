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

// userPromptSubmitInput matches the Claude Code UserPromptSubmit hook schema.
type userPromptSubmitInput struct {
	HookEventName      string `json:"hook_event_name"`
	HookEventNameCamel string `json:"hookEventName"`
	Cwd                string `json:"cwd"`
	SessionID          string `json:"session_id"`
	SessionIDCamel     string `json:"sessionId"`
	Prompt             string `json:"prompt"`
}

func (u *userPromptSubmitInput) EventName() string {
	if u.HookEventName != "" {
		return u.HookEventName
	}
	return u.HookEventNameCamel
}

// userPromptSubmitOutput matches the Claude Code UserPromptSubmit response schema.
type userPromptSubmitOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
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

	if harness != "claude-code" {
		return 0 // Only claude-code currently supported, fail-open
	}

	data, err := io.ReadAll(r)
	if err != nil || len(data) == 0 {
		return 0 // Empty or unreadable stdin: fail-open
	}

	var input userPromptSubmitInput
	if err := json.Unmarshal(data, &input); err != nil {
		return 0 // Invalid JSON: fail-open
	}

	prompt := input.Prompt
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

	if pack == nil || pack.OutputBytes == 0 || !telemetry.InjectionApplied {
		return 0 // Abstained, observe mode, or empty context: exit 0
	}

	out := userPromptSubmitOutput{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:     "UserPromptSubmit",
			AdditionalContext: pack.FormattedText,
		},
	}

	enc := json.NewEncoder(w)
	_ = enc.Encode(out)
	return 0
}
