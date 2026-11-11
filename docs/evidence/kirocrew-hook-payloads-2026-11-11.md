<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# KiroCrew agent hooks: first real payload from kiro-cli (2026-11-11)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai

**Date:** 2026-11-11. **Binary:** `kiro-cli` 3.1.0 (engine `v3`, set in its `cli.json`). Part of GAP-PAYLOADS (KiroCrew/Kiro slice).

## Setup observed

- Hooks come from the `kirocrew` agent file in `~/.kiro/agents`, not from `~/.kiro/hooks/downshift.json` (that file is the Kiro IDE wiring, harness `kiro`).
- That agent declares `preToolUse` with matcher `*` running two scripts (a push-scope guard and the Downshift `kirocrew-pre-tool-use.sh`, which execs `downshift kirocrew`) and `postToolUse` with matcher `*spawn_run*` running `kirocrew-post-tool-use.sh`. A separate `postToolUse` row for `execute_bash` appends the raw stdin payload to the KiroCrew `audit.log`.
- Run: one non-interactive session, `kiro-cli chat --agent kirocrew --no-interactive --trust-workspace --trust-tools=execute_bash,subagent` in a scratch directory, prompt "run `ls` once, then spawn one subagent that runs `pwd`". No user configuration was written.

## Observed

1. The `execute_bash` postToolUse hook fired. The audit log gained one line (477 to 478 lines) holding the raw payload. Anonymised shape:

```json
{"session_id":"sess_<uuid>","hook_event_name":"PostToolUse","cwd":"<scratch dir>",
 "tool_name":"execute_bash",
 "tool_input":{"command":"ls","description":"...","cwd":null,"run_in_background":false,"timeout":null},
 "tool_response":"Output:\n\n\nExit Code: 0"}
```

2. The session recorded the subagent call as a `sub_agent_start` event with `subAgentName`, `prompt`, `explanation` and a `subSessionId`; the call id is prefixed `invoke_subagent_`. The tool the agent saw was named `subagent`, not `spawn_run`.
3. `events.jsonl` gained no row (1688 before and after) and `exec-events.jsonl` gained only rows from the Claude Code session that ran this work. So the Downshift kirocrew hooks left no trace for this run.

## Not observed

- The raw stdin of the Downshift `preToolUse` hook, and of any `postToolUse` for the subagent call: the `*spawn_run*` matcher does not match the tool name used here, and a pass-through writes no row. Capturing the pre payload needs a temporary capture hook in the agent file, which is a config change not made here.
- Any `model` field in a subagent `tool_input`; the session record shows none (only `subAgentName`, `prompt`, `explanation`).
- `model_metadata`, `session_models`, `usage_quota` and the other optional fields the adapter declares.

## Adapter fields against what arrived

| Adapter field (`internal/adapters/kirocrew/kirocrew.go`, `childend.go`) | Arrived (execute_bash post) |
|---|---|
| `hook_event_name`, `cwd`, `session_id`, `tool_name`, `tool_input` | present, same names |
| `tool_use_id` (`childend.go`) | absent |
| `tool_response` | present, a string, not an object with a status |
| `correlation_id`, `model_metadata`, `session_models`, `available_models`, `included_models`, `unavailable_models`, `usage_quota` | absent |
| spawn tool names `spawn_run`, `spawn_sub_agents` (`kirocrew.go`) | kiro-cli v3 uses `subagent`; no routing match |

## Conclusion

kiro-cli runs agent hooks with the field names the adapter reads, but the v3 subagent tool is named differently from the spawn tools the adapter routes, so a kiro-cli subagent spawn is not classified. The `tool_response` string form also differs from the status object `childend.go` expects (its own comment marks that as unverified). The spawn payload remains unconfirmed until a capture hook is allowed.
