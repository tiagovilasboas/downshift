// Copyright (c) 2026 Tiago Vilas Boas.
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
	"github.com/tiagovilasboas/downshift/internal/adapters/opencode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
)

// harnessEntry is everything the CLI needs to know about one harness. Adding
// a harness means adding one entry to harnessRegistry (plus its adapter
// package and caps var); main.go and portFor read from here.
type harnessEntry struct {
	// Hook runs the PreToolUse hook for this harness and returns the exit
	// code. Nil means the harness has no hook subcommand. Every
	// hook is runHookAdapter with that harness's hookWire: rewriteWire for a
	// JSON rewrite response, policyWire for an exit-code-only contract.
	Hook func(in io.Reader, catalog core.Resolver) int
	// Port is the optional post-spawn observation port (honor / usage).
	// Zero value means unobserved.
	Port *hookport.Port
	// Caps are the harness facts.
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
				rewriteWire(func() { fmt.Println(`{"decision":"allow"}`) }),
				func(e antigravity.Event) string { return e.TaskText() },
			)
		},
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
		Caps: core.ClaudeCodeCaps,
	},
	"cursor": {
		Hook: func(in io.Reader, catalog core.Resolver) int {
			return runHookAdapter(
				"cursor",
				in,
				// The payload is either a preToolUse Event or a beforeShellExecution
				// ShellExec; the shell advise path answers the latter.
				cursor.ParsePayload,
				func(e any) (any, string, core.Decision) {
					if ev, ok := e.(cursor.Event); ok {
						return cursor.Handle(ev, catalog)
					}
					return cursor.Output{Permission: "allow"}, "", core.Decision{}
				},
				cursorWire{printCursorAllow},
				func(e any) string {
					if ev, ok := e.(cursor.Event); ok {
						return ev.TaskText()
					}
					return ""
				},
			)
		},
		Port: portPtr(cursor.Port()),
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
		Caps: core.CodexCaps,
	},
	// kirocrew has no rewrite channel (exit 0 allows, exit 2 blocks with
	// stderr relayed to the LLM), so it runs the shared runner in policy mode:
	// same payload read, panic recovery, control group, pause, telemetry and
	// project seam; only the wire differs.
	"kirocrew": {
		Hook: func(in io.Reader, catalog core.Resolver) int {
			return runHookAdapter(
				"kirocrew",
				in,
				func(b []byte) (kirocrew.Event, error) { var e kirocrew.Event; return e, json.Unmarshal(b, &e) },
				func(e kirocrew.Event) (any, string, core.Decision) { return kirocrew.Handle(e, catalog) },
				policyWire{},
				func(e kirocrew.Event) string { return e.TaskText() },
			)
		},
		Port: portPtr(kirocrew.Port()),
		Caps: core.KiroCrewCaps,
	},
	// grok has no hook: a single model with configurable reasoning
	// (SingleModelReasoning).
	"grok": {Caps: core.GrokCaps},

	// opencode's plugin (phase 2) forwards subagent_spawn_intent to the hook
	// and applies the route answer itself; the hook reads the v1 wire, drops
	// unknown fields and answers one allow-listed JSON object. Its taskText is
	// the classification input: it stays in memory and only derived features
	// reach the event log.
	"opencode": {
		Hook: func(in io.Reader, catalog core.Resolver) int {
			return runHookAdapter(
				"opencode",
				in,
				opencode.ParseRequest,
				func(e opencode.Request) (any, string, core.Decision) { return opencode.Handle(e, catalog) },
				openCodeWire{},
				func(e opencode.Request) string { return e.TaskText },
			)
		},
		Port: portPtr(opencode.Port()),
		Caps: core.OpenCodeCaps,
	},
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
