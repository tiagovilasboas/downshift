// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
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

// runDCIHook runs the DCI hook for the harness through the shared
// prompt-context runner (hookprompt.go). The harness gate comes from
// HarnessCapabilities.CanInjectPromptContext.
func runDCIHook(harness string, r io.Reader, w io.Writer, _ io.Writer) int {
	return runPromptContext(harness, r, w, dciContext)
}

// dciContext is the DCI domain step: the formatted context pack for a prompt,
// or "" to abstain (disabled, observe mode, nothing relevant). Fail-open.
func dciContext(prompt, cwd string) string {
	repoRoot := cwd
	if repoRoot == "" {
		repoRoot, _ = os.Getwd()
	}
	cfg, err := codeintel.LoadConfig(repoRoot)
	if err != nil || cfg.Mode == codeintel.ModeOff {
		return ""
	}
	// Budget enforcement: hard 2500ms deadline (fail-open)
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()

	service := codeintel.NewService(repoRoot, cfg)
	pack, telemetry := service.ProcessPrompt(ctx, prompt)
	_ = codeintel.RecordTelemetry(telemetry)

	if pack == nil || pack.OutputBytes == 0 || !telemetry.InjectionApplied {
		return ""
	}
	return pack.FormattedText
}
