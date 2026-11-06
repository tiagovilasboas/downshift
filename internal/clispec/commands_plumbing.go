// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package clispec

// plumbingCommands are called by a harness or its hooks, not typed by a person.
var plumbingCommands = []Command{
	{Name: "claude-code", Summary: "Claude Code PreToolUse hook: rewrites a subagent's model"},
	{Name: "cursor", Summary: "Cursor preToolUse hook: rewrites a subagent's model"},
	{Name: "codex", Summary: "Codex PreToolUse hook: rewrites a subagent's model"},
	{Name: "antigravity", Summary: "Antigravity PreToolUse hook: rewrites a subagent's model"},
	{Name: "kirocrew", Summary: "KiroCrew preToolUse hook: policy mode, exit 0 allow or 2 block"},
	{Name: "opencode", Summary: "OpenCode subagent_spawn_intent route hook (wire v1)"},
	{Name: "honor", Summary: "Post-spawn observation: records whether the routing was honored"},
	{Name: "usage", Summary: "Post-spawn usage observation: records tokens"},
	{Name: "end", Summary: "Child-end observation: marks a subagent as finished"},
	{Name: "probe", Summary: "Record a payload's field names (never prompts or paths) in probe.jsonl"},
	{Name: "cursor-touch", Summary: "Cursor session hooks: remembers the composer model", Hidden: true},
	{Name: "claude-code-post-tool-use", Summary: "Claude Code post-spawn observation", Hidden: true, Legacy: true},
	{Name: "claude-code-subagent-stop", Summary: "Claude Code subagent-stop observation", Hidden: true, Legacy: true},
	{Name: "claude-code-session-start", Summary: "Claude Code session-start hook", Hidden: true, Legacy: true},
}
