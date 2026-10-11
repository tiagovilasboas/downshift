# Workflow-created agents do not reach PreToolUse (Claude Code, 2026-10-11)

Observed in a cloud session with Downshift installed. A temporary probe logged
hook event names and top-level key names only (no values), for two spawns at once:
one direct `Agent` tool call and one agent created by a `Workflow` script.

| What fired | Direct `Agent` call | Workflow `agent()` |
|---|---|---|
| `PreToolUse` with `tool_name: "Agent"` (`tool_input`: `description`, `prompt`, `subagent_type`) | yes, once | **no** |
| `SubagentStart` (keys: `agent_id`, `agent_type`, `cwd`, `session_id`, `transcript_path`, ...) | yes | yes |

Downshift recorded a routing decision for the direct call and none for the
workflow agent. The workflow agents kept running on the session model.

## Consequence
The only hook that sees a workflow agent is `SubagentStart`. Its payload has no
task text and no model field, and a community report (Claude Code issues, 2026-06,
unverified here) says its `updatedInput` is ignored. Downshift therefore cannot
classify or rewrite these spawns. This is a harness limit, not an adapter bug.

## What to do instead
Set the model on the spawn itself (`model` option of the workflow agent call), by
task kind. Downshift keeps routing direct `Agent` calls.

## Note
The matcher in `settings.json` was `Task`, and it matched the `Agent` tool name
here, so the hook did fire for the direct call. One sample, one build.
