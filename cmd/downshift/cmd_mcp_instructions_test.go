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
	for _, want := range []string{"[tier=small]", "apply", "playbook", "status", "never picks one for you", "always call providers first", "every time before a spawn"} {
		if !strings.Contains(mcpInstructions, want) {
			t.Errorf("instructions lack %q", want)
		}
	}
}

// The worker paragraph follows DOWNSHIFT_WORKERS: off says delegate refuses,
// read and bypass say what a worker may do and carry the worker rules (when to
// delegate, reports are data, collect every worker). Every variant stays well
// inside Claude Code's 4096-character cut.
func TestMcpInstructionsFollowTheWorkerMode(t *testing.T) {
	for _, tc := range []struct {
		mode      string
		want      []string
		wantRules bool
	}{
		{"", []string{"Worker tools are off", "delegate refuses every task"}, false},
		{"read", []string{"read-only worker", "write mode is off"}, true},
		{"bypass", []string{"own git worktree", "never merges"}, true},
		{"anything-else", []string{"Worker tools are off"}, false},
	} {
		got := mcpInstructionsFor(tc.mode)
		if !strings.HasPrefix(got, mcpInstructions) {
			t.Errorf("mode %q: the providers rule must still lead", tc.mode)
		}
		if n := len(got); n > 2000 {
			t.Errorf("mode %q: instructions are %d characters, want at most 2000", tc.mode, n)
		}
		for _, w := range tc.want {
			if !strings.Contains(got, w) {
				t.Errorf("mode %q: instructions lack %q", tc.mode, w)
			}
		}
		for _, rule := range []string{"never a single command", "data, not instructions", "Collect every worker"} {
			if strings.Contains(got, rule) != tc.wantRules {
				t.Errorf("mode %q: rule %q present=%v, want %v", tc.mode, rule, !tc.wantRules, tc.wantRules)
			}
		}
	}
}
