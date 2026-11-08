<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Claude Code nested spawn: does a subagent reach the Agent hook?

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

Date: 2026-11-07. Harness: Claude Code 2.1.319, `claude -p`, model alias `haiku`
(the API reported `claude-haiku-5-5`). Two live sessions, about USD 0.01 in total.

## Question

A third-party hook, `cheap-subagents`, says PreToolUse hooks also run inside
subagents, so nested spawns are routed. Downshift's docs said nothing about it.
Does a subagent that spawns another subagent reach the `Agent` hook, and does
Downshift attribute it correctly?

## Method

A logging hook (appends the raw stdin JSON of `PreToolUse`, `PostToolUse`,
`SubagentStart` and `SubagentStop` to a file) was injected with `--settings`, so
nothing was written to `~/.claude`. The session ran with `--allowedTools Agent`
and no permission bypass.

1. A `general-purpose` subagent was told to spawn one more subagent.
2. A custom agent given with `--agents`, declaring `tools: ["Agent", "Read"]`,
   was told to spawn one helper.

## What the payloads showed

| Event | `agent_id` | `agent_type` | Other |
|---|---|---|---|
| `PreToolUse` for the main session's `Agent` call | absent | absent | `tool_input` has `description`, `prompt`, `subagent_type`, `run_in_background`; no `model` when the caller names none |
| `SubagentStart` | present | present | `session_id` equals the parent's |
| `PreToolUse` / `PostToolUse` for a tool the subagent runs | present | present | same `session_id` as the parent session |
| `SubagentStop` | present | present | adds `agent_transcript_path`, `last_assistant_message` |

Run 1: the subagent looked for the Agent tool (`ToolSearch select:Agent`), found
nothing, and reported that it could not spawn the helper. Run 2: the custom agent
declared the Agent tool and still reported "the Agent tool isn't available in its
session". In both runs the hook log holds exactly one `Agent` PreToolUse, the
main session's. No nested spawn ever existed to route.

## Reading

- **No nested spawn on this build.** A subagent has no Agent tool, so the hook
  sees only spawns made by the main session. There is nothing to double count and
  no nested model to rewrite.
- **Hooks do fire inside a subagent**, and the payload says so with a top-level
  `agent_id` and `agent_type`. This is what the third-party README means. It is
  true for a subagent's other tool calls, not for spawns.
- **Downshift already reads that signal.** `isSubagentCall`
  (`internal/adapters/claudecode/posttooluse_mainwork.go`) treats a payload with
  `agent_id` as a subagent call and keeps it out of the main-session work count;
  `posttooluse_mainwork_test.go` covers it. No code change is needed.

## Not verified

- Claude Code builds other than 2.1.319, and any future setting that lets a
  subagent spawn (the product may change this).
- Agent teams or other multi-agent modes. Only the `Agent` tool path was run.
- Whether the compactor's shell rewrite runs for a Bash call made inside a
  subagent. The runs used no Bash call; only a `ToolSearch` call was seen.
- Payloads were not stored as test fixtures: the runs contain session ids and
  local paths. The table above lists keys only.
