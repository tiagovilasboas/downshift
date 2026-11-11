<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Codex CLI: did the wired hooks fire in a live session?

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Date: 2026-11-11. Harness: Codex CLI 0.163.0, `codex exec`, read-only sandbox,
session model reported as `gpt-6.1-terra`, Downshift 0.16.0. One live session
(short hash `01a1cbb1`), plus the one child it spawned.

Closes nothing. It records a negative result for GAP-CODEX-HOOK and the Codex
part of GAP-PAYLOADS: no hook payload was captured.

## Configuration state before the run

- `[features] hooks = true` and `[features.multi_agent_v2] enabled = true` are set.
  `codex features list` reports `hooks` as stable and on.
- The hooks file carries three PreToolUse groups (matchers `(^Agent$|.*spawn_agent$)`,
  a shell-name regex owned by another tool, and `^Bash$`) and two PostToolUse groups
  (`.*spawn_agent$` form and the shell-name regex). The Downshift commands are
  `downshift codex` (twice) and `downshift end codex`.
- Trust is recorded per hook as `hooks.state.<file>:<event>:<group>:<index>` with a
  `trusted_hash`. Entries exist for PreToolUse groups 0 and 1 and PostToolUse
  groups 0 and 1. There is **no entry for PreToolUse group 2** (the `^Bash$` one).
  Whether the existing hashes still match the current hook definitions could not be
  checked from outside Codex.
- `downshift doctor --json` before and after: `hooks wired` ok (configuration only),
  `last hook event` warn "the hooks are wired but codex has never run one".

## The run

Prompt (the only one): run `git status --short`, then spawn one subagent that
replies "ok", wait for it, reply "done". Scratch directory, fresh `git init`.

## Observed

- The session completed. Codex emitted a `spawn_agent` function call (namespace
  `collaboration`, arguments `task_name` and `message`) and a `wait_agent` call; the
  child ran as its own rollout and replied.
- The `message` argument in the rollout is an opaque encrypted blob, not task text.
  The adapter's opaque-task path (`opaque_task_test.go`) is therefore the live case,
  not an edge case.
- The shell command did not use a `Bash` tool. Codex ran it through a custom tool
  named `exec` that calls `tools.exec_command`. The command text had been prefixed
  with the compactor wrapper by the model itself (project instructions), not by a hook.
- `events.jsonl` rows for `codex` before and after: 0 and 0. `exec-events.jsonl`:
  no new `codex` row. The last `codex` row dates from 2026-11-10 and came from a
  direct `downshift compactor` call, not a hook.
- The Codex rollout files contain no hook-related record, and the Codex log
  database has no hook execution line for this session.

## Not observed

- No PreToolUse or PostToolUse payload reached any Downshift binary, so no field
  (`cwd`, `session_id`, `tool_name`, `tool_input`, `model`, `transcript_path`,
  `tool_use_id`) could be seen on the wire.
- No payload dump option exists in the binary, so no raw payload was recorded. The
  conformance fixtures remain the only source for the field names.
- Not established: whether the hooks were skipped because trust hashes are stale,
  because `codex exec` does not run hooks, or because the `spawn_agent` call in
  namespace `collaboration` is not matched by the PreToolUse matchers.

## What the adapter reads (source, unverified against a live payload)

`internal/adapters/codex/codex.go` `Event` (line 28 onward): `hook_event_name`, `cwd`,
`correlation_id`, `tool_use_id`, `session_id`, `tool_name`, `tool_input`, `model`,
optional allowlists and quota marks, and `transcript_path` (line 51). Model
resolution falls back from the spawn input, to `model`, to the transcript.

## Next step

Trust the hooks from inside an interactive Codex session (`/hooks`), start a new
session, and repeat the run. If still silent, capture the payload with a temporary
hook command that tees stdin to a scratch file.
