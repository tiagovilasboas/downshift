<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MCP server checked with the official inspector (2026-10-29)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

What this page answers: does `downshift mcp serve` speak MCP over stdio to a client we did not write? The client is
the MCP project's own inspector, run in its CLI mode. One machine, one run, read-only tools only.

## Method

- Binary: built from `main` after PR 594 (`go build ./cmd/downshift`), into an empty temporary folder.
- Client: `@modelcontextprotocol/inspector`, run with `npx` from an empty temporary folder, install scripts off
  (`npm_config_ignore_scripts=true`).
- Version: the registry lists 2.13.0 as the latest, but `npx` resolved **0.15.0** on this machine (the v1 line, which
  npm marks deprecated and fixes for security only). The cause is the Node version: 2.13.0 needs Node 22.19 or newer
  and the default here was Node 20. The run used 0.15.0 pinned. The v2 run, with the worker tools, is in
  [the worker lifecycle record](mcp-worker-lifecycle-2026-10-29.md).
- Commands: `--cli <binary> mcp serve --method tools/list`, then `--method tools/call --tool-name <name>`.

## Result

`tools/list` returned exactly seven tools, each with a title, annotations and an input schema:

| Tool | Title | Read-only | Idempotent | Destructive |
|---|---|---|---|---|
| `apply` | What would be applied for a choice | yes | yes | no |
| `cancel` | Stop workers | no | yes | no |
| `collect` | Read worker results | no | yes | no |
| `delegate` | Start read-only workers | no | no | no |
| `playbook` | Working rules for a worker | yes | yes | no |
| `providers` | Models this harness offers now | no | yes | no |
| `status` | Quota advice | yes | yes | no |

`tools/call` on `playbook`, `status` and `providers` (with `harness=claude-code`) returned no error. Each result has
`data` and a `strain` block; `providers` also has the `next` pointer with the `[tier=small]` hint. Headroom read
`unknown` because the fresh binary had no quota readings.

## What this supports

- The stdio transport and the tool schemas are accepted by a client we did not write.
- This explains the earlier shell check that returned nothing: that check closed stdin right after sending its
  requests, and the server drops responses in that case (open item in the todo). A client that keeps stdin open, as
  the inspector does, gets every answer.

## What it does not support

- Only three read-only tools were called. `delegate`, `collect` and `cancel` were listed, not called.
- Not the v2 inspector. Not a protocol-conformance suite: it shows the tools list and three calls work, not that every
  edge of the specification holds.
- Not the orchestrator's behaviour: it says nothing about whether a model calls these tools on its own.

## Findings

1. The `delegate` title says "Start read-only workers", but with `DOWNSHIFT_WORKERS=bypass` it starts write workers.
2. Every tool carries `destructiveHint=false`, including `cancel`, which kills running workers. Whether that is the
   right hint is open.
