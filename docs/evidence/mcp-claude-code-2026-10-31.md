<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# The Downshift MCP server in Claude Code (2026-10-30 to 2026-10-31)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Level: **observed on one machine** (macOS, Claude Code 2.1.314 CLI and the 2.1.309 build in Claude Desktop),
binaries `v0.2.0-75-gb2bc0d20` to `v0.2.0-78-g22899752`. Plan and cards:
`docs/internal/tasks/2026-10-30-mcp-claude-code/README.md`. Raw notes live in the private labs repository.

## Result per card

| Card | What was shown | How |
|---|---|---|
| M1 | `doctor` reports `mcp registered`, `mcp command` (registered command is this binary) and `mcp answers` (7 tools over stdio), all green on `v0.2.0-78-g22899752` | `downshift doctor`, after #651 |
| M2 | The official MCP Inspector CLI (2.13.0) lists 7 valid tools with annotations (`apply`, `playbook`, `status` read-only); `resources/list` and `prompts/list` answer empty; `delegate` and `cancel` with invalid arguments fail closed with a clear message | `npx @modelcontextprotocol/inspector --cli` |
| M3 | `providers`, `playbook`, `status` and `apply` each answered in one `claude -p` session; `apply` refused a `prompt` field by design; over stdio, `apply` with `tier: small` moved `claude-opus-5-5` to `claude-haiku-5-5` (`hint_down`) | session `0f61c4fc...`, transcript tool results |
| M4 | Read workers: `delegate` + `collect` round trip (worker replied `ok`); a write request under the read route was refused by plan mode and the file never appeared (spec R27); `cancel` stopped a queued job and, over stdio, a running one (`active: 1` then `outcome: cancelled`, no process left) | sessions `dfd322bb...`, `a119f445...`, `pgrep` |
| M5 | Bypass write route (`DOWNSHIFT_WORKERS=bypass`, owner-authorized) in a throwaway repository: the worker wrote `HELLO.md` in `.downshift/worktrees/w001` on branch `downshift/w001`, untracked; `main` kept only its first commit and a clean `git status`; nothing was merged | `git worktree list`, `git log main` |
| M6 | Unprompted use: 61 of 302 Agent spawns since 2026-10-25 were preceded by a `providers` call in the same session (20.2%), all in Claude Desktop sessions | transcripts, assistant records only |

## Fixed on the way

- `delegate` without `dir` failed every job ("a worker needs a directory") while the schema left `dir` optional. It
  now defaults to the root the server already resolves and refuses when that root is broad (#652).
- The `collect` description now states its 45-second cap (#652).
- `doctor` had no MCP check (#651).

## What this does not show

- The orchestrator calling the tools often on its own: 20.2% of spawns consult `providers` first. Raising that is
  instruction work (session hint, skill), tracked separately.
- A second machine, or Claude Code versions other than the two above.
- MCP in any harness other than Claude Code.

## Correction

The roadmap baseline first counted 345 `providers` calls. Claude Code transcripts mirror each tool call in an
`api-request-blob` record, so the count was inflated; counting assistant records only gives 7 calls in 6 sessions.
