// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// Through the Claude Code hook: a plugin agent spawned with no model leaves the
// parent's model for its declared tier; a call that names a model keeps it.
func TestHookE2E_ClaudeCodeAgentDefaultTier(t *testing.T) {
	frontierID := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	for _, tc := range []struct {
		name      string
		model     string
		wantCode  bool
		wantFinal string
	}{
		{"no model: the declared small tier applies", "", true, "DOWNSHIFT"},
		{"a named model wins", "opus", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.SetHome(t, t.TempDir())
			config := t.TempDir()
			t.Setenv("CLAUDE_CONFIG_DIR", config)
			t.Setenv("ANTHROPIC_MODEL", "")
			t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			setSessionAllowlist(t, "claude-code", []string{
				cmdCat.ModelFor("claude-code", core.TierSmall).ID,
				cmdCat.ModelFor("claude-code", core.TierMid).ID,
				frontierID,
			})
			// The plugin's agent definition, as Claude Code installs it: no model line.
			install := t.TempDir()
			if err := os.MkdirAll(filepath.Join(install, "agents"), 0o700); err != nil {
				t.Fatal(err)
			}
			def := "---\nname: mechanical-worker\ndefault_tier: small\n---\nbody"
			if err := os.WriteFile(filepath.Join(install, "agents", "mechanical-worker.md"), []byte(def), 0o600); err != nil {
				t.Fatal(err)
			}
			installed, _ := json.Marshal(map[string]any{"plugins": map[string]any{
				"downshift@downshift": []map[string]string{{"installPath": install}},
			}})
			if err := os.MkdirAll(filepath.Join(config, "plugins"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(config, "plugins", "installed_plugins.json"), installed, 0o600); err != nil {
				t.Fatal(err)
			}
			transcript := filepath.Join(t.TempDir(), "session.jsonl")
			if err := os.WriteFile(transcript, []byte(`{"type":"assistant","message":{"model":"`+frontierID+`","content":[]}}`+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			toolInput := map[string]any{"subagent_type": "downshift:mechanical-worker", "prompt": "plan the next sprint"}
			if tc.model != "" {
				toolInput["model"] = tc.model
			}
			ev := runClaudeCodeHook(t, eventLog, map[string]any{
				"hook_event_name": "PreToolUse", "tool_name": "Agent", "transcript_path": transcript,
				"cwd": t.TempDir(), "tool_input": toolInput,
			})
			if got := hasCode(ev, "AGENT_DEFAULT_TIER"); got != tc.wantCode {
				t.Errorf("AGENT_DEFAULT_TIER = %v, want %v: %v", got, tc.wantCode, ev["reason_codes"])
			}
			if tc.wantFinal != "" && (ev["verdict"] != tc.wantFinal || ev["outcome"] != "rewrite_emitted") {
				t.Errorf("verdict/outcome = %v/%v, want %s/rewrite_emitted: %v", ev["verdict"], ev["outcome"], tc.wantFinal, ev)
			}
		})
	}
}
