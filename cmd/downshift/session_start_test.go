// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// The session-start hook must answer with one JSON object Claude Code reads as
// additional context, and that context must name the tools and the hint.
func TestSessionStartAnswersWithContextAndTheToolsToCall(t *testing.T) {
	var out bytes.Buffer
	if code := runClaudeCodeSessionStart(&out); code != 0 {
		t.Fatalf("exit = %d", code)
	}
	var got struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if got.HookSpecificOutput.HookEventName != "SessionStart" {
		t.Errorf("event = %q", got.HookSpecificOutput.HookEventName)
	}
	ctx := got.HookSpecificOutput.AdditionalContext
	for _, want := range []string{"providers", "apply", "[tier=small]", "read-only", "Every time you are about to spawn"} {
		if !strings.Contains(ctx, want) {
			t.Errorf("context lacks %q: %s", want, ctx)
		}
	}
}
