// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// Claude Code cuts server instructions at 4096 characters and loads them at
// session start, so the rule for when to call must come first and fit well
// inside the cut.
func TestMcpInstructionsLeadWithTheTriggerAndStayShort(t *testing.T) {
	if n := len(mcpInstructions); n > 1500 {
		t.Errorf("instructions are %d characters, want at most 1500", n)
	}
	lead := mcpInstructions
	if len(lead) > 160 {
		lead = lead[:160]
	}
	if !strings.Contains(lead, "Before you spawn a subagent") || !strings.Contains(lead, "providers") {
		t.Errorf("the first 160 characters should say when to call providers, got %q", lead)
	}
	for _, want := range []string{"[tier=small]", "apply", "playbook", "status", "never picks one for you"} {
		if !strings.Contains(mcpInstructions, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
}
