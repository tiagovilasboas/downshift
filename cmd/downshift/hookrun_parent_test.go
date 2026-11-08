// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

// TestHookRunnerRecordsParentModel: the shared runner stamps the parent session
// model on the decision whether or not the call names its own model, and leaves
// the fields absent when the parent is unknown. Routing is unaffected.
func TestHookRunnerRecordsParentModel(t *testing.T) {
	frontier := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	cases := []struct {
		name       string
		withParent bool
		toolModel  string
		wantParent string
	}{
		{"named model, opus parent", true, "sonnet", frontier},
		{"no model, opus parent", true, "", frontier},
		{"named model, no parent", false, "sonnet", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testenv.SetHome(t, t.TempDir())
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv("ANTHROPIC_MODEL", "")
			t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
			t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			payload := map[string]any{
				"hook_event_name": "PreToolUse", "tool_name": "Agent", "cwd": t.TempDir(),
				"tool_input": map[string]any{"subagent_type": "general-purpose", "prompt": "plain task"},
			}
			if tc.toolModel != "" {
				payload["tool_input"].(map[string]any)["model"] = tc.toolModel
			}
			if tc.withParent {
				transcript := filepath.Join(t.TempDir(), "session.jsonl")
				line := `{"type":"assistant","message":{"model":"` + frontier + `","content":[]}}` + "\n"
				if err := os.WriteFile(transcript, []byte(line), 0o600); err != nil {
					t.Fatal(err)
				}
				payload["transcript_path"] = transcript
			}
			ev := runClaudeCodeHook(t, eventLog, payload)
			got, has := ev["parent_model"].(string)
			if tc.wantParent == "" {
				if has || ev["parent_model_source"] != nil {
					t.Fatalf("parent fields present, want absent: %v", ev)
				}
				return
			}
			if got != tc.wantParent || ev["parent_model_source"] != "transcript" {
				t.Errorf("parent_model = %v (%v), want %s (transcript)", got, ev["parent_model_source"], tc.wantParent)
			}
			if tc.toolModel != "" && ev["requested_model"] == got {
				t.Errorf("requested_model must stay the call model, got %v", ev["requested_model"])
			}
		})
	}
}
