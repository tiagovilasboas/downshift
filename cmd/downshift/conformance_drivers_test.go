// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	conformanceTrivialTask = "rename the userId variable to userIdentifier"
	conformanceMediumTask  = "implement the CSV export feature"
	conformanceComplexTask = "rearchitect the payment flow across services"
)

// spawnSpec is one subagent spawn in harness-free terms.
type spawnSpec struct {
	task     string
	model    string         // requested model id; "" means the spawn names none
	cwd      string         // working directory the payload reports
	siblings map[string]any // extra tool-input fields beside the task and model
}

// hookResult is the harness-free reading of one hook response.
type hookResult struct {
	rc      int
	changed bool           // the hook asked for a model other than the requested one (a rewrite, or a block naming one)
	model   string         // the model the hook left on the wire, or named in a block
	fields  map[string]any // the rewritten tool input; nil when no rewrite was emitted
}

// conformanceDriver is the per-harness data: payload shape in, wire shape out.
type conformanceDriver struct {
	build  func(spawnSpec) []byte
	decode func(t *testing.T, rc int, stdout, stderr, requested string) hookResult
	// chdir means the payload carries no working directory, so the runner
	// reads the process cwd (the project seam) instead; the suite chdirs to
	// the scenario's cwd before running the hook.
	chdir bool
}

// conformanceChildHex is the synthetic spawn correlation for the opencode
// driver: lowercase hex, within the wire's 16..64 character contract.
const conformanceChildHex = "a1b2c3d4e5f60718293a4b5c6d7e8f90"

// conformanceDrivers maps a registry id to its payload builder and response
// decoder. Every registry entry with a Hook must have one; the suite fails
// otherwise, so a new harness cannot be left out of the shared proof.
var conformanceDrivers = map[string]conformanceDriver{
	"claude-code": {
		build: func(s spawnSpec) []byte {
			return mustJSON(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       "Task",
				"cwd":             s.cwd,
				"tool_input":      conformanceToolInput("prompt", s),
			})
		},
		decode: decodeHookSpecificOutput,
	},
	"codex": {
		build: func(s spawnSpec) []byte {
			return mustJSON(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       "spawn_agent",
				"cwd":             s.cwd,
				"tool_input":      conformanceToolInput("message", s),
			})
		},
		decode: decodeHookSpecificOutput,
	},
	"cursor": {
		build: func(s spawnSpec) []byte {
			payload := map[string]any{
				"hook_event_name": "preToolUse",
				"tool_name":       "Task",
				"cwd":             s.cwd,
				"tool_input":      conformanceToolInput("task", s),
			}
			if s.model != "" {
				payload["model_id"] = s.model
			}
			return mustJSON(payload)
		},
		decode: func(t *testing.T, rc int, stdout, _, requested string) hookResult {
			t.Helper()
			var out struct {
				UpdatedInput map[string]any `json:"updated_input"`
			}
			decodeHookStdout(t, stdout, &out)
			return resultFromFields(rc, out.UpdatedInput, "model", requested)
		},
	},
	"antigravity": {
		build: func(s spawnSpec) []byte {
			sub := map[string]any{"Prompt": s.task}
			if s.model != "" {
				sub["Model"] = s.model
			}
			for k, v := range s.siblings {
				sub[k] = v
			}
			return mustJSON(map[string]any{
				"cwd": s.cwd,
				"toolCall": map[string]any{
					"name": "invoke_subagent",
					"args": map[string]any{"Subagents": []any{sub}},
				},
			})
		},
		decode: func(t *testing.T, rc int, stdout, _, requested string) hookResult {
			t.Helper()
			var out struct {
				Overwrite *struct {
					Subagents []map[string]any `json:"Subagents"`
				} `json:"overwrite"`
			}
			decodeHookStdout(t, stdout, &out)
			if out.Overwrite == nil || len(out.Overwrite.Subagents) == 0 {
				return hookResult{rc: rc}
			}
			return resultFromFields(rc, out.Overwrite.Subagents[len(out.Overwrite.Subagents)-1], "Model", requested)
		},
	},
	"kirocrew": {
		build: func(s spawnSpec) []byte {
			return mustJSON(map[string]any{
				"hook_event_name": "preToolUse",
				"tool_name":       "spawn_run",
				"cwd":             s.cwd,
				"tool_input":      conformanceToolInput("task", s),
			})
		},
		// No rewrite channel: exit 2 blocks and stderr names the model to
		// respawn with ("model=<id>"); exit 0 allows unchanged.
		decode: func(_ *testing.T, rc int, _, stderr, _ string) hookResult {
			res := hookResult{rc: rc}
			if rc != 2 {
				return res
			}
			res.changed = true
			if _, after, ok := strings.Cut(stderr, "model="); ok {
				res.model = strings.FieldsFunc(after, func(r rune) bool { return r == ' ' || r == '\n' || r == '(' })[0]
			}
			return res
		},
	},
	"opencode": {
		build: func(s spawnSpec) []byte {
			return mustJSON(map[string]any{
				"v":           1,
				"kind":        "route",
				"harness":     map[string]any{"id": "opencode", "version": "2.0.32"},
				"event":       "subagent_spawn_intent",
				"correlation": map[string]any{"child": conformanceChildHex, "parent": "f0e1d2c3b4a5968778695a4b3c2d1e0f"},
				"agent":       "general",
				"model":       map[string]any{"providerID": "opencode", "id": s.model, "variant": "default"},
				"taskText":    s.task,
				"tsUtc":       "2026-10-18T12:00:00Z",
			})
		},
		// The route answer names the model the plugin should spawn on; it
		// never echoes the spawn input, so fields stay nil. Action "route"
		// is the only change signal on this wire.
		decode: func(t *testing.T, rc int, stdout, _, requested string) hookResult {
			t.Helper()
			var out struct {
				V      int    `json:"v"`
				Action string `json:"action"`
				Model  *struct {
					ProviderID string `json:"providerID"`
					ID         string `json:"id"`
				} `json:"model"`
			}
			decodeHookStdout(t, stdout, &out)
			res := hookResult{rc: rc}
			if out.Action == "route" && out.Model != nil && out.Model.ID != "" {
				res.changed = out.Model.ID != requested
				res.model = out.Model.ID
			}
			return res
		},
		chdir: true,
	},
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// conformanceToolInput builds a flat tool input: the task under taskKey, the
// model under "model" when present, and every sibling field beside them.
func conformanceToolInput(taskKey string, s spawnSpec) map[string]any {
	m := map[string]any{taskKey: s.task}
	if s.model != "" {
		m["model"] = s.model
	}
	for k, v := range s.siblings {
		m[k] = v
	}
	return m
}

func decodeHookStdout(t *testing.T, stdout string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), into); err != nil {
		t.Fatalf("hook stdout is not one JSON object: %v\n%s", err, stdout)
	}
}

// resultFromFields reads a rewritten tool input. An input whose model equals
// the requested one is not a model change (an effort-only rewrite, for example).
func resultFromFields(rc int, fields map[string]any, modelKey, requested string) hookResult {
	if fields == nil {
		return hookResult{rc: rc}
	}
	model, _ := fields[modelKey].(string)
	return hookResult{rc: rc, changed: model != "" && model != requested, model: model, fields: fields}
}

// decodeHookSpecificOutput reads the Claude Code / Codex PreToolUse envelope.
func decodeHookSpecificOutput(t *testing.T, rc int, stdout, _, requested string) hookResult {
	t.Helper()
	var out struct {
		HookSpecificOutput *struct {
			PermissionDecision string         `json:"permissionDecision"`
			UpdatedInput       map[string]any `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	decodeHookStdout(t, stdout, &out)
	if out.HookSpecificOutput == nil {
		return hookResult{rc: rc}
	}
	// A decision without a rewrite would override the user's ask rules.
	if out.HookSpecificOutput.PermissionDecision != "" && out.HookSpecificOutput.UpdatedInput == nil {
		t.Errorf("unchanged spawn must emit no permissionDecision: %s", stdout)
	}
	return resultFromFields(rc, out.HookSpecificOutput.UpdatedInput, "model", requested)
}
