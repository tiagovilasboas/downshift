<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Worker lifecycle through an MCP client (2026-10-29)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

What this page answers: do `delegate`, `collect` and `cancel` work end to end when a client we did not write drives
them over stdio, for the read route and the write route? No model was called: the worker program is a fake `claude`.
One machine, one run.

## Method

- Client: `mcpdo` from `@modelcontextprotocol/inspector` 2.13.0 on Node 22.23.1. It keeps one named connection open,
  so the worker queue, which lives in the server process, survives from one call to the next. The inspector's
  stateless `--cli` mode starts a new server per call and cannot do this.
- Server: `downshift mcp serve` built from commit `ca808704` (the delegate title change), started with
  `DOWNSHIFT_WORKERS=bypass`, so the read and the write route were both on.
- Fake worker: a shell script named `claude`, first on the server's `PATH`. It reads the prompt, sleeps 60 seconds
  when the prompt contains `SLOW`, writes `shim-output.txt` only when launched with `bypassPermissions`, and prints a
  success result. This stands in for the real CLI; it does not prove the real CLI behaves the same.
- Isolation: `HOME`, the state directory, the inspector catalog and a private daemon were all under a scratch folder,
  and the target was a disposable git repository. Nothing touched the owner's `~/.downshift` or `~/.mcp-inspector`.

## Result

| Step | Observed |
|---|---|
| `tools/list` | Seven tools: `apply`, `cancel`, `collect`, `delegate`, `playbook`, `providers`, `status` |
| `delegate` read, one task | Job `w001` queued |
| `collect` with `wait` | One delivery, outcome `done`, text from the worker |
| `collect` with the last `ack` | No deliveries: the acknowledged one is gone |
| `delegate` write, one task | Job `w002` queued |
| `collect` with `wait` | `done`; the text names the worktree and the branch `downshift/w002` and says Downshift does not merge |
| The repository afterwards | Branch `downshift/w002` exists; `shim-output.txt` is in the worktree and not in the main checkout |
| `delegate` slow task, then `cancel` | Outcome `cancelled`; unacknowledged deliveries come back with `repeat` set |

## Findings

1. **Cancel left the worker's child running.** The fake `claude` stopped, but the `sleep 60` it had started kept
   running. With a real CLI, the test runs and builds a worker starts would outlive a cancel or a budget expiry. The
   worker's `Wait` also stayed blocked while that child held the output pipes. Fixed in the branch that adds this page:
   the worker now starts in its own process group and a stop reaches the whole group, with a regression test that
   failed before the change.
2. **The write route leaves `.downshift/` untracked in the user's checkout.** `git status` in the target repository
   showed `?? .downshift/` after the write job, because the worktrees live under it. A careless `git add -A` would pick
   it up. Open.
3. **The queue has no shutdown.** By reading the code, `Queue` has no method that stops running workers when the
   server exits. Not observed in this run. Open.

## What this supports

- The read and write routes, the acknowledgement of deliveries, the isolation of a write job in its own worktree and
  branch, and cancellation all work through a real MCP client, for a worker that behaves like the CLI contract.

## What it does not support

- The real `claude` or `codex` CLIs. That plan mode blocks a write (spec R27) is still unobserved.
- The budget expiry path, the limit of four running workers, large outputs, or more than one task per call.
- Anything about whether an orchestrator model chooses to call these tools.

## Notes on the inspector

- Version 2.13.0 requires Node 22.19 or newer. On Node 20, `npx` quietly falls back to the last compatible release,
  0.15.0, which is how the earlier run in [the inspector check](mcp-inspector-2026-10-29.md) ended up on 0.15.0.
- `mcpdo` writes its catalog under `~/.mcp-inspector` unless `MCP_CATALOG_PATH` is set, and keeps secrets in a plain
  file when the OS keychain is not available. Run it with its own `HOME`, as here.
