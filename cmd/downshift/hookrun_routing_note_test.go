// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// promptSentinel marks the task text. It must never reach the event log.
const promptSentinel = "ZZSENTINEL7391"

// TestHookRunnerRecordsRoutingNote drives the shared runner with the production
// decider and a project policy, and checks the event the runner writes: the
// routing note holds the verdict, reason, declared rule or profile id and the
// tiers, and nothing of the prompt.
func TestHookRunnerRecordsRoutingNote(t *testing.T) {
	frontier := cmdCat.ModelFor("claude-code", core.TierFrontier).ID
	cases := []struct {
		name       string
		policy     string
		agentType  string
		definition string // agents/<agentType>.md; "" writes none
		want       map[string]any
		absent     []string
	}{
		{
			name: "kept: no policy, nothing moves", agentType: "general-purpose",
			want:   map[string]any{"verdict": "kept"},
			absent: []string{"rule_id", "profile"},
		},
		{
			name:      "profile: an agent type names a declared profile",
			policy:    `{"version":1,"routing":{"profiles":[{"name":"cheap","tier":"small","harness":"claude-code"}]}}`,
			agentType: "cheap", definition: "---\nname: cheap\n---\nbody",
			want: map[string]any{"verdict": "profile", "profile": "cheap", "tier_from": "frontier", "tier_to": "small"},
		},
		{
			name:       "rule: a declared rule moves the spawn",
			policy:     `{"version":1,"routing":{"rules":[{"id":"cheap-lane","tier":"small"}]}}`,
			agentType:  "worker",
			definition: "---\nname: worker\n---\nbody",
			want:       map[string]any{"verdict": "rule", "rule_id": "cheap-lane", "tier_from": "frontier", "tier_to": "small"},
		},
		{
			name:       "refused_config: a rule names a profile of another harness",
			policy:     `{"version":1,"routing":{"profiles":[{"name":"elsewhere","tier":"small","harness":"codex"}],"rules":[{"id":"to-codex","profile":"elsewhere"}]}}`,
			agentType:  "worker",
			definition: "---\nname: worker\n---\nbody",
			want:       map[string]any{"verdict": "refused_config", "reason": "unknown_profile"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			t.Setenv("ANTHROPIC_MODEL", "")
			t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
			t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
			t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			setSessionAllowlist(t, "claude-code", []string{
				cmdCat.ModelFor("claude-code", core.TierSmall).ID,
				cmdCat.ModelFor("claude-code", core.TierMid).ID,
				frontier,
			})
			project := t.TempDir()
			if tc.policy != "" {
				writeFile(t, filepath.Join(project, ".downshift", "policy.json"), tc.policy)
			}
			if tc.definition != "" {
				agents := filepath.Join(project, ".claude", "agents")
				if err := os.MkdirAll(agents, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(agents, tc.agentType+".md"), []byte(tc.definition), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			transcript := filepath.Join(t.TempDir(), "session.jsonl")
			line := `{"type":"assistant","message":{"model":"` + frontier + `","content":[]}}` + "\n"
			if err := os.WriteFile(transcript, []byte(line), 0o600); err != nil {
				t.Fatal(err)
			}
			ev := runClaudeCodeHook(t, eventLog, map[string]any{
				"hook_event_name": "PreToolUse", "tool_name": "Agent", "transcript_path": transcript, "cwd": project,
				"tool_input": map[string]any{"subagent_type": tc.agentType, "prompt": "plain task " + promptSentinel},
			})
			note, ok := ev["routing_note"].(map[string]any)
			if !ok {
				t.Fatalf("event has no routing_note: %v", ev)
			}
			for k, v := range tc.want {
				if note[k] != v {
					t.Errorf("routing_note.%s = %v, want %v (note %v)", k, note[k], v, note)
				}
			}
			for _, k := range tc.absent {
				if _, has := note[k]; has {
					t.Errorf("routing_note.%s present, want absent: %v", k, note)
				}
			}
			raw, err := os.ReadFile(eventLog)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), promptSentinel) {
				t.Errorf("the prompt text reached the event line: %s", raw)
			}
		})
	}
}
