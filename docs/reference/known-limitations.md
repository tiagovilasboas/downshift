<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Known limitations

What Downshift does not do yet, in one place. The beta label stays until the [exit criteria](beta-exit.md) close. Per-harness evidence is in the [harness matrix](harness-matrix.md).

## Harnesses

- **Claude Code** is the only harness exercised end to end every release.
- **Cursor:** the hook is wired, but its neutral `{}` answer and the `agent_message` advice have not been verified live. The shell rewrite is limited to one read-only command.
- **KiroCrew** has only an allow/block `preToolUse` hook, so it cannot rewrite a command or a model. It gets advice, with no limit on what it advises, and no automatic routing.
- **Codex, Antigravity, OpenCode:** partial routing and compaction; see the matrix for each row.
- **Claude Desktop** Chat and Cowork host the MCP server only: no hooks, no rewrite.

## Compaction

- A shell **alias or function** hides the command from the allowlist, so it runs unwrapped.
- A command already written as `downshift compact -- <command>` passes through as typed, including a mutating one.
- `npm test` and `npm run <script>` are not compacted when the script starts a watcher: the script body is not visible. Known follow, watch and server forms (`docker logs -f`, `jest --watch`, `vite preview`) run as typed.
- Quoted shapes (a quoted word, a trailing `2>/dev/null` or `|| true`) are wrapped for every harness that declares a POSIX shell; they are not gated per operating system.

## Raw output kept by CCR

- Redaction covers common credential shapes but misses the values `aws sts` and `aws ssm` print. Turn CCR off (`"ccr": false` and `"ccr_failures": false`, see the [CCR guide](../guides/ccr.md)) before running those, or do not run them through `downshift compact`.

## Setup

- `downshift setup --undo` restores the whole backup, so a change you made to the same file after setup is lost. Review the diff first.
- Setup leaves a linked file alone and holds the Claude Desktop entry while Claude is running.

## Measurement

- Savings are tokens, never money. How a plan turns tokens into its 5-hour and weekly limits is not published, so no limit saving is claimed.
- Quota routing reads a statusline snapshot that only an interactive Claude Code terminal writes; elsewhere it can be stale.

Found something not listed? [Open an issue](https://github.com/tiagovilasboas/downshift/issues).
