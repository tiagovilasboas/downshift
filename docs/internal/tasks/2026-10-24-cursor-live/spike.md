# Spike: Cursor hook on 3.27.12 (one spawn)

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Date: 2026-10-24. Cursor 3.27.12 (`df132ffba90dfc15a12e6b8f020ba4e5d026c4b0`, arm64).
Installed binary `v0.1.2-65-g480391dc`. No prompt, path, or account id is stored here.

## Question

The doctor reported Cursor hooks wired and zero events. Is the hook dead, or
does a real Task spawn reach `downshift cursor` and then get held for a reason
that already lives in the shared runner?

## What was run

One local Task spawn, subagent type mechanical, no model pinned. The child
answered and finished. After it, `~/.downshift/events.jsonl` was counted by
harness and outcome only.

## Finding

The hook runs. The spawn was held by the shared guardrail, and the end hook
left no record.

| Fact | Status | Source |
|---|---|---|
| `~/.cursor/hooks.json` has `preToolUse` matcher `Task` → `downshift cursor`, and `subagentStop` → `downshift end cursor` | CONFIRMED | file read 2026-10-24; doctor `hooks wired` 2 of 2 |
| Live log from `2026-10-21T02:16:35Z` to just before the spawn: 537 `claude-code`, 12 `codex`, 9 `kirocrew`, 0 `cursor`. No rotated `events.jsonl` backup | CONFIRMED | line count of `~/.downshift/events.jsonl` (558 lines, 439478 bytes) |
| Those `claude-code` decisions are family names (`haiku`, `sonnet`, `opus`, `fable`), agent always `subagent` | CONFIRMED | same file, decision rows only. They are not Cursor copies relabelled |
| One Task spawn at `2026-10-24T14:48:35.592149Z` wrote harness `cursor`, source `hook`, outcome `allow` | CONFIRMED | that row |
| Verdict `UNKNOWN`, safe verdict `OK`, tier `mid`, complexity `MEDIUM`, corrections `R9_UNCONFIDENT_UNKNOWN_MID`, reason codes `CLASSIFIER_UNCONFIDENT`, `HELD_UNCONFIDENT_UNKNOWN_MID`, `NO_SIGNAL` | CONFIRMED | same row |
| `requested_model` and `final_model` are `unknown`. `requested_model_raw` is absent | CONFIRMED | same row. Absent raw means the adapter passed an empty current model, not an id the catalog rejected (`internal/telemetry/telemetry.go`) |
| Recommended model on the row is `claude-4.5-sonnet-thinking`. Nothing was written to the spawn | CONFIRMED | `recommended_model`, outcome `allow` |
| No second cursor row. No `child_ended`, `resolved`, or `usage` | CONFIRMED | recount after the child finished: cursor count is 1 |
| `runEnd` records nothing when `ParseChildEnd` returns false, and exits 0 | CONFIRMED in code | `cmd/downshift/end.go`. A missing process and a rejected payload look the same in the log |
| Cursor's current hooks page still lists `preToolUse` matcher `Task` with `updated_input`, and `subagentStart` with `subagent_model` whose output is only `permission` and `user_message` | CONFIRMED on the page | https://cursor.com/docs/hooks fetched 2026-10-24 via the docs index. `subagentStart` is not a rewrite channel |
| Which payload fields carried the parent model on this spawn | CONFIRMED absent on preToolUse | probe `2026-10-24T14:51:53Z`: top-level `model` empty, no `model_id`, `tool_input` keys `description`, `prompt`, `subagent_type` only |
| Whether `subagentStop` ran | CONFIRMED | probe lines at `14:51:43Z` and `14:51:57Z`; `status` key present (value not stored by probe) |
| preToolUse carries `session_id` and `tool_use_id` | CONFIRMED | same probe line |
| subagentStop carries `subagent_id` matching Task `tool_use_id`, no `tool_call_id` | CONFIRMED | hashes `77b8480b` on both events for one spawn |
| subagentStop top-level `model` is parent slug (`composer-2.5-fast`) | CONFIRMED | probe fields; arrives after routing, not used for baseline |

## Where a fix is allowed to live

The hold is the shared rule. `R9_UNCONFIDENT_UNKNOWN_MID` allows the spawn
unchanged. This phase does not change R7, R8, or R9.

An empty current model is parsed in the Cursor adapter (`RequestedModel` /
`Handle` in `internal/adapters/cursor/cursor.go`): `tool_input.model`, then
top-level `model_id`, then top-level `model`. `core.Route` already receives
that string. A new field name, if the probe shows one, is read in the adapter
and passed into the same call. It is not a new policy and not a core branch
on the harness.

The end record is the shared runner (`downshift end`, `childend.Record`).
The Cursor parser only maps fields. A live `subagentStop` shape that the
parser rejects is fixed in `ParseChildEnd`, with a fixture that has no task
text, and a test through `runEnd`.

`subagentStart` stays unwired. Its documented output cannot carry a model.

## Not this spike

- Rebuilding the binary. The row above was written by `v0.1.2-65-g480391dc`. The two commits after it are the changelog close and `gh run view --log` compression.
- Syncing `~/.cursor/rules/kiro-*.mdc` from harness-core. That is the rules generator, not the router.
- The native quota bridge. Cursor still blocks that transport for non-built-in extensions ([gap diagnostic](../../../evidence/cursor-gap-diagnostic.md)).
- Claiming the rewrite was honored. This spawn emitted no rewrite. The 2026-10-10 spawn emitted one and the child transcript had no executor model ([rewrite evidence](../../../evidence/cursor-rewrite-honored-2026-10-10.md)).
