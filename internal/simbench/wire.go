// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package simbench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Spawn is one synthetic child as the simulator plans it. Raw ids carry
// distinctive prefixes so the prompt-free check can prove none reached disk.
type Spawn struct {
	Harness     string
	Session     string
	ToolUse     string
	Agent       string
	Correlation string // lowercase hex, the routing id the runner keeps
	Task        string
	AgentType   string
	// Requested is the model the child runs on without a rewrite, the truth the
	// simulator prices the baseline with.
	Requested string
	// Explicit puts Requested in the spawn call; otherwise the harness learns
	// it by inheritance (the parent transcript).
	Explicit   bool
	Dir        string // per-harness fixture directory (transcripts, rollouts)
	ProjectDir string
}

// Raw-id prefixes; Check fails if any of them appears in what the hook wrote.
const (
	sessionPrefix = "simsess"
	toolPrefix    = "simtool"
	agentPrefix   = "simagent"
)

// wire is one harness's payload shapes: the only per-harness knowledge in
// this package, kept as data so the engine itself names no harness. The shapes
// are the ones the adapters' own tests use; unverified harness payloads stay
// unverified here too (docs/reference/harness-matrix.md).
type wire struct {
	// prepare writes the files the spawn payload points at.
	prepare func(s Spawn) error
	spawn   func(s Spawn) []byte
	// written is the model the hook wrote for the child, "" when unchanged.
	written func(stdout []byte) string
	// honor is the post-launch observation proving what ran; nil when the
	// harness reports it in the end payload or not at all.
	honor func(s Spawn, written, ran string) []byte
	// ends are the child-end payloads, in the order the harness sends them.
	ends func(s Spawn, ran string, u telemetry.TokenUsage) ([][]byte, error)
	// reported is the usage as the harness can report it (Codex has no cache
	// write split): the what-if prices exactly this.
	reported func(u telemetry.TokenUsage) telemetry.TokenUsage
	// reportsRan: a post-spawn payload names the model the child ran on, so a
	// rewrite gets an honored or refuted verdict.
	reportsRan bool
	// readsAgentDefinitions: the harness resolves a custom agent type's model
	// from its definition file (pinned types run on their own model).
	readsAgentDefinitions bool
}

var wires = map[string]wire{
	"claude-code": {
		prepare: func(s Spawn) error {
			// The parent's assistant message that issued this call names the
			// parent model; the hook reads it by tool-use id.
			line := mustJSON(map[string]any{"type": "assistant", "message": map[string]any{
				"model": s.Requested, "content": []any{map[string]any{"type": "tool_use", "id": s.ToolUse, "name": "Agent"}},
			}})
			return appendLine(transcriptPath(s), line)
		},
		spawn: func(s Spawn) []byte {
			in := map[string]any{"prompt": s.Task, "description": "simulated task", "subagent_type": s.AgentType}
			if s.Explicit {
				in["model"] = s.Requested
			}
			return mustJSON(map[string]any{
				"hook_event_name": "PreToolUse", "tool_name": "Agent", "session_id": s.Session,
				"tool_use_id": s.ToolUse, "correlation_id": s.Correlation, "cwd": s.ProjectDir,
				"transcript_path": transcriptPath(s), "tool_input": in,
			})
		},
		written: func(stdout []byte) string {
			var o struct {
				H *struct {
					UpdatedInput map[string]any `json:"updatedInput"`
				} `json:"hookSpecificOutput"`
			}
			if json.Unmarshal(stdout, &o) != nil || o.H == nil {
				return ""
			}
			m, _ := o.H.UpdatedInput["model"].(string)
			return m
		},
		ends: func(s Spawn, ran string, u telemetry.TokenUsage) ([][]byte, error) {
			post := mustJSON(map[string]any{
				"hook_event_name": "PostToolUse", "tool_name": "Agent", "session_id": s.Session, "tool_use_id": s.ToolUse,
				"tool_response": map[string]any{"agentId": s.Agent, "status": "completed", "resolvedModel": ran},
			})
			stop := mustJSON(map[string]any{
				"hook_event_name": "SubagentStop", "session_id": s.Session, "agent_id": s.Agent, "model": ran,
				"usage": map[string]any{
					"input_tokens": u.InputTokens, "output_tokens": u.OutputTokens,
					"cache_read_input_tokens":     u.CachedTokens - u.CacheWriteTokens,
					"cache_creation_input_tokens": u.CacheWriteTokens,
				},
			})
			return [][]byte{post, stop}, nil
		},
		reported:              identity,
		reportsRan:            true,
		readsAgentDefinitions: true,
	},
	"cursor": {
		prepare: noFiles,
		spawn: func(s Spawn) []byte {
			// Cursor names the parent model on every call (model_id); the task
			// input may repeat it.
			return mustJSON(map[string]any{
				"hook_event_name": "preToolUse", "tool_name": "Task", "model_id": s.Requested,
				"tool_use_id": s.ToolUse, "correlation_id": s.Correlation, "cwd": s.ProjectDir,
				"tool_input": map[string]any{"task": s.Task, "model": s.Requested},
			})
		},
		written: func(stdout []byte) string {
			var o struct {
				UpdatedInput map[string]any `json:"updated_input"`
			}
			if json.Unmarshal(stdout, &o) != nil {
				return ""
			}
			m, _ := o.UpdatedInput["model"].(string)
			return m
		},
		ends: func(s Spawn, _ string, _ telemetry.TokenUsage) ([][]byte, error) {
			// No model and no tokens: Cursor's stop payload reports a status only.
			return [][]byte{mustJSON(map[string]any{
				"hook_event_name": "subagentStop", "tool_use_id": s.ToolUse, "subagent_id": s.Agent, "status": "completed",
			})}, nil
		},
		reported: identity,
	},
	"codex": {
		prepare: noFiles,
		spawn: func(s Spawn) []byte {
			return mustJSON(map[string]any{
				"hook_event_name": "PreToolUse", "session_id": s.Session, "tool_use_id": s.ToolUse,
				"correlation_id": s.Correlation, "tool_name": "spawn_agent", "cwd": s.ProjectDir,
				"tool_input": map[string]any{"message": s.Task, "model": s.Requested},
			})
		},
		written: func(stdout []byte) string {
			var o struct {
				H *struct {
					UpdatedInput map[string]any `json:"updatedInput"`
				} `json:"hookSpecificOutput"`
			}
			if json.Unmarshal(stdout, &o) != nil || o.H == nil {
				return ""
			}
			m, _ := o.H.UpdatedInput["model"].(string)
			return m
		},
		ends: func(s Spawn, ran string, u telemetry.TokenUsage) ([][]byte, error) {
			// Codex counts cached input inside input_tokens.
			rollout := filepath.Join(s.Dir, "rollout-"+s.ToolUse+".jsonl")
			line := mustJSON(map[string]any{"type": "event_msg", "payload": map[string]any{"type": "token_count", "info": map[string]any{
				"total_token_usage": map[string]any{
					"input_tokens": u.InputTokens + u.CachedTokens, "cached_input_tokens": u.CachedTokens, "output_tokens": u.OutputTokens,
				},
			}}})
			if err := appendLine(rollout, line); err != nil {
				return nil, err
			}
			return [][]byte{mustJSON(map[string]any{
				"hook_event_name": "PostToolUse", "session_id": s.Session, "tool_name": "spawn_agent", "tool_use_id": s.ToolUse,
				"transcript_path": rollout, "tool_response": map[string]any{"status": "completed", "model": ran},
			})}, nil
		},
		reported: func(u telemetry.TokenUsage) telemetry.TokenUsage {
			u.CacheWriteTokens = 0 // the rollout has no cache-write split
			return u
		},
		reportsRan: true,
	},
	"opencode": {
		prepare: noFiles,
		spawn: func(s Spawn) []byte {
			return mustJSON(map[string]any{
				"v": 1, "kind": "route", "harness": map[string]any{"id": "opencode"}, "event": "subagent_spawn_intent",
				"correlation": map[string]any{"child": s.Correlation}, "agent": s.AgentType,
				"model": map[string]any{"providerID": "opencode", "id": s.Requested}, "taskText": s.Task,
			})
		},
		written: func(stdout []byte) string {
			var o struct {
				Action string `json:"action"`
				Model  *struct {
					ID string `json:"id"`
				} `json:"model"`
			}
			if json.Unmarshal(stdout, &o) != nil || o.Action != "route" || o.Model == nil {
				return ""
			}
			return o.Model.ID
		},
		honor: func(s Spawn, written, ran string) []byte {
			// The switch chain the plugin observes: attempt, selection, then the
			// model the child's first context ran on.
			c := map[string]any{"child": s.Correlation}
			return joinLines(
				mustJSON(map[string]any{"v": 1, "kind": "observation", "correlation": c, "event": "switch_attempt", "from": s.Requested, "to": written, "result": "ok"}),
				mustJSON(map[string]any{"v": 1, "kind": "observation", "correlation": c, "event": "model_selected", "model": written}),
				mustJSON(map[string]any{"v": 1, "kind": "observation", "correlation": c, "event": "child_context", "model": ran}),
			)
		},
		ends: func(s Spawn, ran string, u telemetry.TokenUsage) ([][]byte, error) {
			return [][]byte{mustJSON(map[string]any{
				"v": 1, "kind": "observation", "correlation": map[string]any{"child": s.Correlation}, "event": "child_done",
				"model": ran, "status": "completed",
				"usage": map[string]any{"input": u.InputTokens, "output": u.OutputTokens, "cache": map[string]any{
					"read": u.CachedTokens - u.CacheWriteTokens, "write": u.CacheWriteTokens,
				}},
			})}, nil
		},
		reported:   identity,
		reportsRan: true,
	},
}

// Harnesses lists the harnesses the simulator can drive, sorted.
func Harnesses() []string {
	out := make([]string, 0, len(wires))
	for h := range wires {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

func identity(u telemetry.TokenUsage) telemetry.TokenUsage { return u }

func noFiles(Spawn) error { return nil }

func transcriptPath(s Spawn) string {
	return filepath.Join(s.Dir, "parent-"+s.Session+".jsonl")
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("simbench: marshal fixture: %v", err))
	}
	return b
}

func joinLines(lines ...[]byte) []byte {
	var out []byte
	for _, l := range lines {
		out = append(append(out, l...), '\n')
	}
	return out
}

func appendLine(path string, line []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
