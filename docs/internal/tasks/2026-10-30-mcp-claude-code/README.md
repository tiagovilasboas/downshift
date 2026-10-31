<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MCP fully working in Claude Code (2026-10-30)

Goal: every tool of the Downshift MCP server works in a real Claude Code session, the registration is checked by
`doctor`, and every limit is declared. Scope: Claude Code only (CLI and the Desktop Code tab). Codex and the other
harnesses are out of scope here.

## Starting point (observed 2026-10-30, binary `v0.2.0-75-gb2bc0d20`)

| Fact | Source |
|---|---|
| `downshift` is registered at user scope as `downshift mcp serve` (stdio, `DOWNSHIFT_MANAGED=setup`) and Claude Code reports it connected | `claude mcp list`, `claude mcp get downshift` |
| The server answers `initialize` and `tools/list` with 7 tools: `providers`, `playbook`, `status`, `apply`, `delegate`, `collect`, `cancel` | stdio exchange, 2026-10-30 |
| Calls in Claude Code transcripts since 2026-10-25: `providers` 345, `status` 2, `apply` 2; `playbook`, `delegate`, `collect` and `cancel` none | `~/.claude/projects/**/*.jsonl` |
| `downshift doctor` has no MCP check: it never says whether the server is registered, which binary it points at, or whether it answers | `cmd/downshift/doctor*.go` |
| The registered binary had been built from an older commit than `main` until it was reinstalled | `downshift version` |
| Worker tools: one real read-only Claude worker completed `delegate` and `collect` (2026-10-29); a write refused under plan mode (spec R27) is not shown; the bypass write route has not run against the real CLI | CHANGELOG `[Unreleased]` |

## Definition of done

- Each of the 7 tools is called in a real Claude Code session and returns the expected result, with the session id
  and the event or tool result recorded as evidence.
- `doctor` reports, for Claude Code, whether the server is registered, whether the registered command is the binary
  on PATH, and whether it answers `tools/list`.
- The worker routes are proven or declared: read route round trip, a write refused under the read route (R27),
  `cancel` stopping a running job, and the bypass route in its own worktree.
- `internal/limits` carries a `mcp` row for `claude-code` that matches the evidence.

## Tasks

| Card | What | Points | Depends on | Parallel |
|---|---|---|---|---|
| M0 | Baseline above | 1 | | done |
| M1 | `doctor` MCP check for Claude Code: registered, command matches the binary on PATH, `tools/list` answers within a timeout | 3 | M0 | yes |
| M2 | Conformance: run the official MCP Inspector CLI against `downshift mcp serve`, record tools, schemas and annotations | 2 | M0 | yes |
| M3 | Real session, read-only tools: `playbook`, `status`, `apply`, `providers` each called in one Claude Code session | 2 | M0 | yes, uses tokens |
| M4 | Real session, workers: `delegate` + `collect` (read route), `cancel` on a running job, a write attempt refused under the read route (R27) | 3 | M3 | no |
| M5 | Bypass write route against the real CLI: job runs in `.downshift/worktrees/`, report names worktree and branch, nothing merged | 3 | M4 | no, owner opt-in |
| M6 | Unprompted use: share of Agent spawns preceded by a `providers` call in the same session, from transcripts | 2 | M0 | yes |
| M7 | Evidence doc, `limits` row `mcp` for claude-code, README/capability matrix | 1 | M1 to M6 | no |

Total: 17 points, no card over 3.

## Out of scope

- MCP in Codex, Cursor, Antigravity, KiroCrew and the Claude Desktop Chat (covered by findings F2 and limits).
- New MCP tools.
