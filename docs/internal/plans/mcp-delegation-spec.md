<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MCP delegation server (`downshift mcp`)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **Draft, 2026-10-28.** Reopens owner decision O7 for one part: the MCP server is Downshift's own, run by the
same binary, not a separate product. Decision O8 (the LLM chooses the model) is the premise. Clean-room: the behaviour
of the Alethe MCP path (kept outside this repository, no code copied) informs the shape; the choices below are ours.

## 0. What it is

`downshift mcp` is a stdio MCP server started by the orchestrator's harness (Claude Code, Codex). The orchestrator
calls its tools to delegate a worker with an agent CLI, choosing the **provider, model and effort** itself. The server
applies the choice, enforces the project policy the user declared, runs the worker in the requested mode, and returns
the result with a routing note. It never chooses a model on its own.

## 1. Requirements

- R1. One binary, zero external dependencies: JSON-RPC 2.0 over stdio, written in Go (no MCP SDK), so the install
  stays a single file.
- R2. Subcommand `downshift mcp`; no daemon, no listener, no network socket (O7 non-goal still holds: no app, no proxy).
- R3. Tools: `delegate` (provider, model, effort, mode `read|write`, prompt, cwd), `status` (job id), `result`
  (job id), `cancel` (job id), `providers` (installed agent CLIs and their models from the catalog).
- R4. The model and effort in `delegate` are applied verbatim. The server passes them through the routing decider only
  to record the note and to honour `policy.json` (O8). No built-in table, no automatic upgrade, no risk veto.
- R5. Mode `read` runs the provider with its read-only permission mode; mode `write` runs in an isolated git worktree
  (stage 4 P5). The server never emits a permission-bypass flag.
- R6. The prompt is held in memory for the life of the job and never written to disk (O4). Output goes only where the
  caller names; an interrupted job is lost (O4).
- R7. Results carry enums, tiers, models, tokens and times, and the routing note. No task text in any note (contract).
- R8. Budget: the same call, wall-time and token caps as the plan runner (stage 4 section 4).
- R9. Errors are returned as tool errors, never as a crash; a malformed request is refused with a reason.

## 2. Fronts

| Front | Points | Scope | Depends on | Gate |
|---|--:|---|---|---|
| M1 JSON-RPC stdio core | 3 | `internal/mcp/rpc/*`: framing, initialize, tools/list, tools/call, errors (R1, R9) | none | unit tests on framed messages; `make check` |
| M2 Tool surface | 3 | `internal/mcp/tools/*`: the five tools (R3), schemas, no prompt on disk (R6) | M1, P8 (plan runner jobs) | R3, R6, R7 tests |
| M3 Command | 1 | `cmd/downshift/cmd_mcp.go`, `case "mcp"` in `main.go`, help spec | M2 | e2e: initialize + tools/list over pipes |
| M4 Setup | 1 | `setup` registers the server in the harness's MCP config (opt-in, dry-run, undo by marker) | M3, S1 | setup tests; undo leaves other servers |
| M5 Playbook | 1 | `downshift-orchestrator` skill names the MCP tools (after M2) | M2, R2 | `TestSetupInstallsTheOrchestratorPlaybook` |

Dependency direction: `internal/mcp` depends on `internal/plan` through small interfaces; nothing in `internal/routing`
or `internal/core` imports `internal/mcp`. Each front is one PR of at most 5 points, `make check` green.

## 3. Open decisions

- G-MCP1. Harness registration path for Claude Code (`.mcp.json` or the settings `mcpServers` key). Verify on the
  installed version before M4.
- G-MCP2. Whether `delegate` returns after the job starts or waits (default: returns a job id; `result` blocks with a
  timeout). Owner to confirm.
