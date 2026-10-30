<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Claude Desktop proof before the release gate (2026-10-30)

Goal: before the D5 release gate, prove in Claude Desktop that model routing, compaction and MCP work, surface by
surface, and say plainly where they cannot. Spike and surface map: `downshift-labs/reverse/claude-desktop/SPIKE.md`.

## Roadmap

1. Prove the Code tab (CD-1 to CD-3): the surface that should behave like the CLI.
2. In parallel, prepare the Chat MCP entry (CD-4) and map Cowork (CD-5), both read-only until the owner approves.
3. Turn the results into code where a gap is found (CD-6): client attribution and surface limits.
4. Record the evidence and update the docs (CD-7). Then go to D5.

## Tasks

| Card | What | Points | Depends on | Parallel |
|---|---|---|---|---|
| CD-0 | Spike: surfaces, embedded version, plugin loading, open questions | 1 | | done |
| CD-1 | Code tab, routing: one spawn asks for an Opus model with `[tier=small]` and a trivial task; expect a `rewrite` decision to the small model and a child on that model, correlated to the Desktop session | 2 | CD-0 | serial with CD-2, CD-3 (same app) |
| CD-2 | Code tab, compaction: one `go test` on a small package; expect the Bash rewrite to `downshift compactor --` and an exec event | 2 | CD-0 | serial |
| CD-3 | Code tab, MCP: the `downshift` MCP tools are listed and `providers` answers | 2 | CD-0 | serial |
| CD-4 | Chat, MCP: dry-run diff of the `claude_desktop_config.json` entry, check the stdio server with the MCP Inspector; apply and restart only with owner approval | 2 | CD-0 | yes |
| CD-5 | Cowork: read-only REA of what reaches the VM (hooks, plugin, binary, MCP) | 2 | CD-0 | yes |
| CD-6 | Record the client (`CLAUDE_CODE_ENTRYPOINT`) on events; add Chat and Cowork rows to `internal/limits` so `doctor` says what does not work | 3 | CD-1 to CD-5 | no |
| CD-7 | Evidence doc and README/capability matrix | 1 | CD-6 | no |

Total: 15 points, no card over 3.

State (2026-10-30, evidence in `docs/evidence/claude-desktop-code-tab-2026-10-30.md`): CD-1 routing proven (rewrite
honored on a real Desktop child); CD-3 MCP proven; CD-2 mostly proven (the Bash rewrite runs in real Desktop sessions;
the embedded 2.1.309 build run headless compacted 2,373 to 218 bytes); CD-4 and CD-5 done in the labs findings; CD-6 in PRs #643 (merged), #644 and #645.
New gap: exec events carry no session id or client, so compaction cannot be attributed to a client by hash.

## Pass criteria

- A surface is "works" only with an event log line, the matching session id and the observed result (child model,
  compacted output, MCP answer). A decision with `allow` is not a rewrite proof.
- A surface that cannot carry a feature is declared `none` with the reason, not left silent.

## Out of scope

- Restarting Claude Desktop or editing its config without owner approval.
- The D5 gate items (AppSec, remote CI, migration note).
