// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// A custom agent is no longer an unknown baseline just because its type is not
// a built-in: through the shared runner, one with no `model:` inherits the
// parent's model (the event records it and the downshift is measurable), and one
// that pins a model records that model as the baseline and is left unchanged.
func TestHookE2E_ClaudeCodeAgentDefinitionBaseline(t *testing.T) {
	frontierID := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	cases := []struct {
		name        string
		definition  string
		wantVerdict string
		wantOutcome string
		wantPinned  bool
	}{
		{"no model line inherits the parent", "---\nname: worker\n---\nbody", "DOWNSHIFT", "rewrite_emitted", false},
		{"a pinned model is kept", "---\nname: worker\nmodel: opus\n---\nbody", "DOWNSHIFT", "allow", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv("ANTHROPIC_MODEL", "")
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
			setSessionAllowlist(t, "claude-code", []string{
				cmdCat.ModelFor("claude-code", core.TierSmall).ID,
				cmdCat.ModelFor("claude-code", core.TierMid).ID,
				frontierID,
			})
			project := t.TempDir()
			agents := filepath.Join(project, ".claude", "agents")
			if err := os.MkdirAll(agents, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(agents, "worker.md"), []byte(tc.definition), 0o600); err != nil {
				t.Fatal(err)
			}
			transcript := filepath.Join(t.TempDir(), "session.jsonl")
			if err := os.WriteFile(transcript, []byte(`{"type":"assistant","message":{"model":"`+frontierID+`","content":[]}}`+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			ev := runClaudeCodeHook(t, eventLog, map[string]any{
				"hook_event_name": "PreToolUse", "tool_name": "Agent", "transcript_path": transcript, "cwd": project,
				"tool_input": map[string]any{"subagent_type": "worker", "prompt": "rename the variable userId to userIdentifier"},
			})
			if ev["requested_model"] != frontierID {
				t.Errorf("requested_model = %v, want the baseline %s (not unknown)", ev["requested_model"], frontierID)
			}
			if ev["verdict"] != tc.wantVerdict || ev["outcome"] != tc.wantOutcome {
				t.Errorf("verdict/outcome = %v/%v, want %s/%s: %v", ev["verdict"], ev["outcome"], tc.wantVerdict, tc.wantOutcome, ev)
			}
			if got := hasCode(ev, "AGENT_PINS_MODEL"); got != tc.wantPinned {
				t.Errorf("AGENT_PINS_MODEL = %v, want %v: %v", got, tc.wantPinned, ev["reason_codes"])
			}
			if ev["agent_type"] != "custom" {
				t.Errorf("agent_type = %v, want the privacy label custom (the agent's own name is never stored)", ev["agent_type"])
			}
		})
	}
}
