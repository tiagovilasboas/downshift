// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/tiagovilasboas/downshift/internal/adapters/antigravity"
	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/adapters/kirocrew"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
)

// harnessEntry is everything the CLI needs to know about one harness. Adding
// a harness means adding one entry to harnessRegistry (plus its adapter
// package and caps var); main.go, portFor and tryHook read from here.
type harnessEntry struct {
	// Hook runs the PreToolUse hook for this harness and returns the exit
	// code. Nil means the harness has no hook subcommand (try-only).
	Hook func(in io.Reader, catalog core.Resolver) int
	// Port is the optional post-spawn observation port (honor / usage).
	// Zero value means unobserved.
	Port *hookport.Port
	// Try runs the real adapter on a synthetic payload for `downshift try`.
	// Nil means try falls back to core.Route.
	Try func(prompt, current string, catalog core.Resolver) tryResult
	// Caps are the harness facts; try reads SingleModelReasoning from here.
	Caps core.HarnessCapabilities
}

var harnessRegistry = map[string]harnessEntry{
	"antigravity": {
		Hook: func(in io.Reader, catalog core.Resolver) int {
			return runHookAdapter(
				"antigravity",
				in,
				func(b []byte) (antigravity.Event, error) { var e antigravity.Event; return e, json.Unmarshal(b, &e) },
				func(e antigravity.Event) (any, string, core.Decision) { return antigravity.Handle(e, catalog) },
				func() { fmt.Println(`{"decision":"allow"}`) },
				func(e antigravity.Event) string { return e.TaskText() },
			)
		},
		Try:  tryAntigravity,
		Caps: core.AntigravityCaps,
	},
	"claude-code": {
		Hook: func(in io.Reader, catalog core.Resolver) int {
			return runHookAdapter(
				"claude-code",
				in,
				func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
				func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, catalog) },
				printAllow,
				func(e claudecode.Event) string { return e.TaskText() },
			)
		},
		Port: portPtr(claudecode.Port()),
		Try:  tryClaudeCode,
		Caps: core.ClaudeCodeCaps,
	},
	"cursor": {
		Hook: func(in io.Reader, catalog core.Resolver) int {
			return runHookAdapter(
				"cursor",
				in,
				func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
				func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, catalog) },
				printCursorAllow,
				func(e cursor.Event) string { return e.TaskText() },
			)
		},
		Port: portPtr(cursor.Port()),
		Try:  tryCursor,
		Caps: core.CursorCaps,
	},
	"codex": {
		Hook: func(in io.Reader, catalog core.Resolver) int {
			return runHookAdapter(
				"codex",
				in,
				func(b []byte) (codex.Event, error) { var e codex.Event; return e, json.Unmarshal(b, &e) },
				func(e codex.Event) (any, string, core.Decision) { return codex.Handle(e, catalog) },
				printAllow,
				func(e codex.Event) string { return e.TaskText() },
			)
		},
		Port: portPtr(codex.Port()),
		Try:  tryCodex,
		Caps: core.CodexCaps,
	},
	"kirocrew": {
		Hook: runKiroCrewHook,
		Try:  tryKiroCrew,
		Caps: core.KiroCrewCaps,
	},
	// grok has no hook: a single model with configurable reasoning, so try
	// reports an effort instead of a model (SingleModelReasoning).
	"grok": {Caps: core.GrokCaps},
}

func portPtr(p hookport.Port) *hookport.Port { return &p }

// harnessIDs lists registry ids in sorted order.
func harnessIDs() []string {
	out := make([]string, 0, len(harnessRegistry))
	for id := range harnessRegistry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// tryInput builds the synthetic tool input for try.
func tryInput(promptKey, prompt, current string) json.RawMessage {
	m := map[string]any{promptKey: prompt}
	if current != "" {
		m["model"] = current
	}
	raw, _ := json.Marshal(m)
	return raw
}

// tryWritten reads a string key from an updatedInput blob.
func tryWritten(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func tryClaudeCode(prompt, current string, catalog core.Resolver) tryResult {
	out, _, d := claudecode.Handle(claudecode.Event{ToolName: "Task", ToolInput: tryInput("prompt", prompt, current)}, catalog)
	r := tryResult{decision: d}
	if out.HookSpecificOutput != nil {
		if w := tryWritten(out.HookSpecificOutput.UpdatedInput, "model"); w != current {
			r.written = w
		}
	}
	return r
}

func tryCursor(prompt, current string, catalog core.Resolver) tryResult {
	out, _, d := cursor.Handle(cursor.Event{ToolName: "Task", ToolInput: tryInput("task", prompt, current)}, catalog)
	r := tryResult{decision: d}
	if w := tryWritten(out.UpdatedInput, "model"); w != current {
		r.written = w
	}
	return r
}

func tryCodex(prompt, current string, catalog core.Resolver) tryResult {
	out, _, d := codex.Handle(codex.Event{ToolName: "spawn_agent", ToolInput: tryInput("message", prompt, current)}, catalog)
	r := tryResult{decision: d}
	if out.HookSpecificOutput != nil {
		if w := tryWritten(out.HookSpecificOutput.UpdatedInput, "model"); w != current {
			r.written = w
		}
		r.effort = tryWritten(out.HookSpecificOutput.UpdatedInput, "reasoning_effort")
	}
	return r
}

func tryKiroCrew(prompt, current string, catalog core.Resolver) tryResult {
	out, _, d := kirocrew.Handle(kirocrew.Event{ToolName: "spawn_run", ToolInput: tryInput("task", prompt, current)}, catalog)
	r := tryResult{decision: d, blocked: out.Block}
	if out.Block {
		r.written = d.Model.ID
	}
	return r
}

func tryAntigravity(prompt, current string, catalog core.Resolver) tryResult {
	sub := map[string]any{"Prompt": prompt}
	if current != "" {
		sub["Model"] = current
	}
	args, _ := json.Marshal(map[string]any{"Subagents": []any{sub}})
	out, _, d := antigravity.Handle(antigravity.Event{ToolCall: antigravity.ToolCall{Name: "invoke_subagent", Args: args}}, catalog)
	r := tryResult{decision: d}
	if len(out.Overwrite) > 0 {
		var m struct {
			Subagents []map[string]any `json:"Subagents"`
		}
		if json.Unmarshal(out.Overwrite, &m) == nil && len(m.Subagents) > 0 {
			if w, _ := m.Subagents[0]["Model"].(string); w != current {
				r.written = w
			}
		}
	}
	return r
}
