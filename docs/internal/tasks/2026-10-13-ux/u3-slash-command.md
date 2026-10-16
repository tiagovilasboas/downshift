<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# U3: `/downshift` inside the harness: session tree, why, pause

The developer should see what Downshift did without leaving the conversation.

CLI (shared, every harness): `downshift session [--tree|--why|--pause|--resume] [--session=<id>] [--json]`.
- `--tree` (default): the current session's subagents as a tree, one line each:
  `├─ Refactor billing   sonnet  ✓  $0.22   MEDIUM, confident`
  columns: label, final model, end status (`✓` completed, `↻` redone, `✗` error,
  `…` running), cost (real if observed, else `~` estimate, else `-`), short reason
  from the reason codes. Header: `session · N subagents · $X (without downshift: $Y)`.
  Redo = a later spawn in the same session within 10 min after an error/interrupt
  (reuse `telemetry.RespawnAfterFailure`). Task labels are NEVER stored: show the
  agent type or `subagent N`; if the harness gives a local transcript the label
  may be read from it at render time only (optional, off by default).
- `--why`: for the latest decision, every reason code in plain words
  (SEMANTIC_BOOST, RISK_FLOOR, HELD_*...), the guardrail that held it, verdict and
  tier. One screen.
- `--pause` / `--resume`: write/remove a per-session pause marker in the state dir
  (keyed by the hashed session id, expiring after 12 h). The shared hook runner
  checks it (one `stat`) and routes as the control group does (classify and log,
  never rewrite; outcome `baseline`, reason `PAUSED`). Never blocks, fail open.
- Session id: from `--session`, else `CLAUDE_SESSION_ID`-style env if the harness
  declares one in `HarnessCapabilities` (data, no harness name branch), else the
  most recent session in the log.
- Shared formatter in `internal/telemetry` (`SessionTree`, `Why`), reuse U1/U2
  helpers (`ShortModel`, tail reader). Prompt-free.

Claude Code surface: add a project command file `.claude/commands/downshift.md`
(and document installing it under `~/.claude/commands/` in `docs/guides/install.md`) that
runs `downshift session $ARGUMENTS` through the Bash tool and prints the output
verbatim, with the allowed-tools frontmatter limited to that command.

Tests: tree rendering (empty, one, nested redo, error, running, loss shown as
over), `--why` for held and lifted decisions, pause marker expiry and effect
through `runHookAdapter` (no rewrite, logged as baseline), session selection,
output < 50 ms on a 50 MB log. Docs: `docs/guides/quota.md` or a new `docs/guides/session-view.md`.
