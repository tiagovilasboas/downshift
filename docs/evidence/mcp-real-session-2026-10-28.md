<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MCP server: a real Claude Code session calls the tools (2026-10-28)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Level: **observed once, one machine, one session.** It shows the server works end to end with a real client; it says
nothing about savings, and nothing about how often a model would use the tools unprompted.

## What was run

- Client: Claude Code 2.1.310, model `claude-haiku-5-5`, non-interactive (`claude -p`), at most 8 turns, no session
  persistence. The server was passed for this one run only (`--mcp-config` with `--strict-mcp-config`); no user
  configuration was written.
- Server: `downshift mcp serve`, built from the working tree (the tool port, the SDK transport and the real adapters).
- Prompt: ask the model to call `providers` for the harness `claude-code`, then `apply` with a small tier hint, then
  `status`, and to report one fact from each result.

## What happened

| Step | Result |
|---|---|
| Connection | the client listed the server as `connected` and exposed `mcp__downshift__apply`, `playbook`, `providers` and `status` |
| `providers` | three detected models for `claude-code` with their tiers (small, mid, frontier), efforts, and `source: file` (the user's `session-models.json`), none flagged untiered |
| `apply` (tier small, no model) | verdict `kept`, `applied: false`, reason "no requested model: nothing to apply" |
| `status` | server name, version and the playbook count (3), plus a `strain` block for every harness; headroom `unknown` because no quota snapshot was present |
| End | `success` after 5 turns; tokens: 8 input, 63,018 cache-write, 186,078 cache-read, 551 output |

## What it does and does not show

- It shows a current Claude Code reaches the server, lists its tools and gets well-formed answers, over the stdio
  transport, with the tool names the model sees.
- It does not show the protocol era Claude Code used (the client did not report it), the behaviour with the client's
  other runtime, or any effect on a spawn.
- The `apply` answer is the honest one for that input: a tier hint moves a requested model down, and none was requested.
