// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// mcpHelp covers the MCP server the orchestrator can ask.
var mcpHelp = map[string]string{
	"mcp": `downshift mcp — The MCP server the orchestrator can ask

Usage:
  downshift mcp serve

serve runs an MCP server on standard input and output. A harness starts it once it is registered (downshift setup
does that for Claude Code and Codex, unless --no-mcp); you do not run it by hand. It answers both protocol eras: the initialize handshake
of 2025-11-25 and earlier, and the per-request metadata with server/discover of 2026-07-28.

Tools: providers (the models this harness offers now), playbook (the working rules for workers), status (quota advice)
and apply (what would be applied for a choice). Every result carries a strain block with the quota headroom. The server
never picks a model for you and takes no task text. Standard output carries protocol messages only; logs go to
standard error. Ends when the client closes its input.

Flags:
  none

Examples:
  downshift mcp serve
  claude mcp add -s user downshift -- "$(command -v downshift)" mcp serve

Next: downshift setup, to register the server in your harnesses
`,
}
