// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// sessionStartContext is the line Claude Code puts into the session's context at
// start. It is a short, factual pointer to the orchestrator's own choice, the
// only channel that reaches the model before it decides how to delegate
// (docs/internal/plans/mcp-spike-2026-10-28.md).
const sessionStartContext = "Downshift is installed. Every time you are about to spawn a subagent with the Agent or Task tool, " +
	"call the downshift MCP tool providers first: it is the one step that shows the models this harness offers right now, so make it a habit before the spawn, not an afterthought. " +
	"Then call apply to preview your choice of model and effort. " +
	"Choose the model and effort yourself. For mechanical work, start the spawn's prompt with [tier=small]; that hint only moves down. " +
	"Workers from the MCP are read-only unless the owner turned on write mode."

// runClaudeCodeSessionStart answers the SessionStart hook with the context line.
// It reads nothing from its input and fails open: any write error is ignored,
// because a hook that prints nothing leaves the session as it was.
func runClaudeCodeSessionStart(out io.Writer) int {
	payload := map[string]any{
		"hookSpecificOutput": map[string]string{
			"hookEventName":     "SessionStart",
			"additionalContext": sessionStartContext,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return 0
	}
	_, _ = fmt.Fprintln(out, string(b))
	return 0
}
