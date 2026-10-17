# downshift-router (experimental mod)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

An opt-in Claude Code **mod** (plugin with a hooks module, Claude Code 2.1.287+) that routes a
subagent's model from the in-process `agent.spawn` event instead of the settings `PreToolUse` hook.
It is a prototype that answers one question, "can Downshift become a Claude Code mod?", with code;
the design and the evidence are in [docs/design/mod-router.md](../../docs/design/mod-router.md).

**Off by default.** The supported path is still `downshift setup` / the root plugin's
`hooks/hooks.json`. Do not run both on the same session unless you are measuring the overlap: each
spawn is then routed twice (observed 2026-10-17, see the design doc).

## What it does

On every `agent.spawn` (a subagent or teammate about to start, its model not yet resolved) the mod:

1. Returns `next(e)` unchanged unless enabled, and for a fork (inherits, `model` is ignored), a resume
   (only a refusal is possible) or a workflow agent (its prompt may be a placeholder).
2. Builds the PreToolUse-shaped payload the `downshift claude-code` hook already reads on stdin, from
   the fields the event really carries: `prompt`, `description`, `subagentType`, `model` (as the call
   gave it), `name`, `background`, `cwd`, `tool_use_id`, `permissionMode`, plus `session_id` and the
   session `cwd` from `$.session`. The parent's effective model travels as `parent_model`.
3. Runs `downshift claude-code` through `$.process.run` with the JSON on stdin (8 s cap; the binary
   answers in about 30 ms), parses `hookSpecificOutput.updatedInput.model`, and returns
   `next({ ...e, model })` when it differs from the requested one.
4. Fails open on anything else: binary missing, non-zero exit, timeout, non-JSON output, no model
   written. One dim `$.ui.log` line names the reason (to the debug log when nothing changed, to the
   transcript when a model was written). It never returns `{ deny }`.

On every `turn.complete` for a subagent loop (`e.agentId` set) the mod, when enabled:

1. Adds `e.usage` into a per-agent total (multi-turn subagents sum every turn).
2. When `$.agent.list()` shows that agent `completed` / `failed` / `killed` (or gone), runs
   `downshift end claude-code` with a SubagentStop-shaped payload that carries the totals inline, so
   the shared ChildEnd recorder prices the child without reading a transcript.
3. If `$.agent.list()` is unavailable, leaves pricing to the settings `SubagentStop` hook (fail open).
4. Dedup in the recorder still prices each agent once, so overlapping settings + mod paths do not
   double-bill.

On every `turn.step` for a subagent loop (`e.agentId` set), when enabled, the mod returns
`next({ ...e, effort })` when the spawn-time `downshift claude-code` decision carried
`reasoning_effort` in `updatedInput` (the settings hook ignores that field). Effort is stored
per `agentId` from the spawn result and cleared when usage is flushed at loop end.

All routing and pricing logic stays in the Go core; this module is an adapter in the sense of
[docs/contrib/layering.md](../../docs/contrib/layering.md): parse the event, call the shared binary,
apply the wire result.

## Enable it

Either of these turns it on (the option wins when set):

- `userConfig` field `enabled` (boolean, default `false`): `/plugin configure downshift-router`, or
  `claude plugin install ... --config enabled=true`, or `pluginConfigs["downshift-router"].options`
  in `~/.claude/settings.json`.
- Environment variable `DOWNSHIFT_MOD_ROUTER=1` (read with `$.env.get`; handy for `claude -p`, where
  `--plugin-dir` plugins have no stored options).

`binary` (string, default `downshift`) names the binary when it is not on the PATH Claude Code runs
with.

`DOWNSHIFT_MOD_ROUTER_LOG=<file>` appends one JSON line per spawn (`tool_use_id`, `subagent_type`,
`requested_model`, `parent_model`, `written_model`, `elapsed_ms`, `note`). It exists because nothing
draws under `claude -p`; it writes no prompt text.

Development run from this repository:

```bash
DOWNSHIFT_MOD_ROUTER=1 claude --plugin-dir ./mods/downshift-router
```

## Check it

```bash
claude plugin validate mods/downshift-router   # manifest, hooks, calls and env names the module makes
claude plugin test mods/downshift-router       # mock-engine tests; the binary is stubbed
```

The tests cover: disabled (binary never runs), enabled by option and by env, a rewrite applied, binary
error, non-zero exit, non-JSON output, allow without `updatedInput`, same model written back, fork /
resume / workflow pass-through, the trace file, and `turn.complete` usage flush (accumulate, wait for
terminal status, `end claude-code` payload), and `turn.step` effort from the spawn decision).

## What is missing compared with the settings hook

- `transcript_path` is not in the event. The event does carry `parentModel`; the mod sends it as
  `parent_model`, and the Claude Code adapter reads it under its inheriting-type rule (before the
  transcript), so a mod spawn gets a known baseline without waiting on disk.
- `session_models`, `model_metadata` and the quota marks are absent, as they are from Claude Code's
  own PreToolUse payload; the session list is inferred the same way in both paths.
- The payload always carries `entry: "mod"` so the decision event, `stats` and `doctor` can tell
  the two entry points apart; `doctor` warns when one `tool_use_hash` carries two decisions.

## Limits

Claude Code only (mods load nowhere else); the hook runs under `claude -p` but draws nothing; mods are
not sandboxed; a hook's own time limit is 10 s (time inside `$.process.run` does not count, the run
itself is capped at 8 s here). `agent.spawn` does not fire again when a subagent is resumed with
`SendMessage`.
