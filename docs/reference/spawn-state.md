# Spawn state: when Downshift works and when it switches

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift-ai

Every subagent spawn the hook sees gets exactly one **state**, computed once in
`internal/telemetry/spawn_state.go` from the decision event. Every harness
gets it through the shared runner (`runHookAdapter`); no adapter holds any of it.

| State | Meaning |
|---|---|
| `switched` | Downshift changed the spawn's model or effort |
| `held` | A change was considered; a guardrail (`HELD_*`), the credit gate or a guessed baseline kept the spawn as it was. Doubt never spends money |
| `observed` | Downshift looked and had nothing to change |
| `paused` | `downshift session --pause`: classified and logged, never changed |
| `unrouted` | A `baseline` line that is not a pause. Only older versions wrote one (their control group or no-route switch). Classified, never changed. No current version leaves a spawn in this state |
| `blocked` | A policy-mode harness (KiroCrew) denied the spawn for a respawn |

Held wins over switched: a hold means the change was not applied.

## Where it shows

| Surface | What you see | Harnesses |
|---|---|---|
| stderr, one line per spawn | `downshift: spawn switched opus → haiku, effort high → low · TRIVIAL`, `spawn held on opus (would be haiku) [HELD_UNCONFIDENT_DOWNSHIFT]`, `spawn observed, spawn unchanged on opus` | all (whether the harness UI shows hook stderr is harness-dependent; see the matrix) |
| `downshift watch` | first line: `● downshift active · last spawn 12s ago: ...` or `○ downshift idle · ...` (idle after 5 min without a spawn); then the tree | all |
| `downshift session` / `--json` | header `N switched, M held`; each row's reason starts `switched from <model>`; JSON rows carry `state`, `from_model`, `reason_codes`, and the view `switched`, `held` | all |
| statusline | `◆ downshift · 3 subagents · 2 switched · 1 held · ...` | harnesses with a statusline bridge |
| `live.json` | the record below, for a panel | all |

Idle means no recent spawn. The hook only runs when a subagent is spawned, so
it cannot show a heartbeat between spawns.

## `live.json` (for panels and mods)

Written atomically next to the event log (`~/.downshift/live.json`, or beside
`DOWNSHIFT_EVENT_LOG`) on every spawn. Best effort and fail-open: a write
failure never delays or blocks a spawn. No prompt, task text, file path, raw
session or agent id is stored; `session` is the first 8 hex of the hashed id.

```json
{
  "updated": "2026-10-19T12:00:00.123Z",
  "session": "a1b2c3d4",
  "counts": {"switched": 2, "held": 1, "observed": 4},
  "last": {
    "state": "switched",
    "harness": "claude-code",
    "session": "a1b2c3d4",
    "time": "2026-10-19T12:00:00.123Z",
    "from_model": "claude-opus-5-5",
    "to_model": "claude-haiku-5-5",
    "from_effort": "high",
    "to_effort": "low",
    "recommended_model": "",
    "complexity": "TRIVIAL",
    "reason_codes": ["CLASSIFIER_CONFIDENT"]
  }
}
```

`counts` reset when the session changes. A reader treats `updated` older than
5 minutes as idle. For `held`, `to_model` is what the child runs on and
`recommended_model` is what Downshift would have chosen.

## `downshift session --panel` (what the agents panel reads)

Prompt-free JSON for the selected session (latest, or `--session`), built in
`telemetry.BuildPanel`. Ready-made labels, so a panel only draws:

```json
{"session":"a1b2c3d4","summary":"Downshift · 2 model · 1 held","switched":2,"model_switched":2,"held":1,"observed":4,
 "tokens":{"spawns":4,"below_spawns":2,"billable":5200,"below_billable":3200,
           "fallback_spawns":0,"fallback_billable":0,"fallback_below_billable":0,
           "unobserved_spawns":0,"cache_read":180000},
 "tokens_line":"3.2k of 5.2k billable below the session model (2 of 4 spawns)",
 "by_agent_type":[{"agent_type":"general-purpose","tokens":{...},
                   "line":"general-purpose: 2.1k of 3.0k billable below the session model (2 of 3 spawns)"}],
 "spawns":[{"tool_use":"<sha256 hex>","state":"switched",
            "from_model":"claude-opus-5-5","to_model":"claude-sonnet-5-5",
            "label":"opus → sonnet (mid)",
            "tokens":{...},"tokens_label":"48k tokens · below the session model"}]}
```

Labels: `opus → sonnet (mid)`, `held: original model kept`, `kept`,
`unrouted (older log)`, `paused (unrouted)`, `blocked: respawn on another model`.
`tool_use` is `telemetry.ToolUseKey(tool_use_id)`: sha256 of
`harness-downshift-session-v1:` + the harness tool call id.

`summary` is the band's one line, composed in Go (`telemetry.PanelSummary`):
`Downshift · N model · M effort · K held`, a zero count left out, `""` when
nothing was changed or held. A panel prints it verbatim and does no arithmetic.

`tokens` is `telemetry.RoutingTokens` for the session (one harness) and for each spawn: the
fields of [stats-export.md](stats-export.md#routing-tokens). **Breaking:** `saved_total` and
`saved_short` per spawn are gone, as are the old `children`, `saved_children`, `work_tokens`,
`work_saved`, `cache_tokens`, `cache_saved`, `work_pricier`, `cache_pricier`, `at_floor`,
`gateway_unpriced`, `saved_inferred` and `saved_without_rewrite` keys of `tokens`. The panel also
carries `below_session_by_harness`: one row per harness with the same fields plus `observed`,
and `efficiency`: the `efficiency` block of `downshift report --json` for the report's default
window (see [report.md §Efficiency](../guides/report.md#efficiency)).

### Trimming the panel: `--no-report` and `--brief`

Both need `--panel` (alone they are a usage error). The default answer is the
whole panel above and is unchanged.

- `--no-report` skips the 30-day report. The full logs are not read, only the
  tail, so the call stays flat as the log grows. The output has no `report`,
  `compactor` or `tokens_saved_estimate`; the session fields are the same.
- `--brief` prints only what the agents band draws:
  `{session, summary, switched, model_switched, effort_changed, held,
  held_guardrail, upshift_not_applied, observed, spawns[]}`, where each spawn is
  `{tool_use, state, from_model, to_model, observed_model, label, below_billable,
  below_short, baseline}` (`baseline` is `session`, `requested_fallback` or `""`). It drops `tokens`, `tokens_line`, `by_agent_type` and each
  spawn's `tokens` and `tokens_label`. Without `--no-report` it adds
  `tokens_saved_estimate` (the report's figure, a number) and
  `tokens_saved_short` (`74k`, `1.2M`, `""` when 0). With `--no-report` neither
  appears.

The counts, `summary` and spawn rows of `--brief` equal the full panel's
(`TestPanelBrief_FieldsMatchFullPanel`).

`observed_model` is optional and per spawn (`telemetry.PanelSpawn`): the model
the child was observed to run on, taken from its resolved record or else its
usage record. It is set only when that model is known and differs from
`to_model`; an equal, dated-equal, unknown or missing observation leaves the key
out. It holds model ids only, never prompts.

Tokens, never dollars, and never summed across harnesses (owner decisions 2026-10-19, 2026-11-03).
"Below the session model" means a spawn's billable tokens (input, output, cache write; cache
read apart) ran on a lower tier than the session model recorded on its decision
(`telemetry.RoutingTokens`, the same rows as `stats --export` `routing_tokens`). A spawn with no
usable session model is placed against its requested model instead, in separate `fallback_*`
fields. `tokens_line` and `tokens_label` are `""` until a child's tokens are known; a reader then
draws no figure. Unrouted, held, kept and refuted children count in the figures. Harnesses with
no proven token source (Cursor, Codex, OpenCode today) have no token figures, never zeros. No
dollars: `downshift report` is tokens only.

The same figures appear in `downshift session` (header, a tokens column per row, a `by type`
line, shown only for types that ran something below the session model), the statusline
(`12k of 40k billable below the session model (3 of 5 spawns)`), `downshift watch`, `dsmon`, the
child-end line and the `stats` headline (`downshift stats`; `stats --detail` for more).
`downshift session` has no tokens for one decision and says only where they would
run: `→ small: opus → haiku · its tokens would run below the requested model`, or
`(not applied, tokens stay on opus)` when the hook would leave the spawn alone.

### Compactor in the panel JSON

`session --panel` adds an optional `compactor` block for **this session**: the runs tagged
with the session's hashed id, like the routing figures beside it.
`{scope, commands, compacted, raw_bytes, saved_bytes, estimated_tokens_saved, token_estimate, families, direct}`,
where `scope` is `"this session"`. Bytes are measured; `estimated_tokens_saved` is
`saved_bytes / 4`, an estimate.

Two other figures have a different scope and say so:

- `tokens_saved_estimate` (the bar) and `report` cover the report window over **all
  sessions**, including runs with no session; `tokens_saved_scope` names it
  (`"last 30 days, all sessions"`).
- `compactor.direct` is the runs of that window with **no session tag** (a direct
  `downshift compactor -- ...`), which the session figures never include:
  `{scope, commands, saved_bytes, estimated_tokens_saved, line}`, with `line` reading
  `direct calls, no session: N, ~X tokens saved`. It is shown apart and never added to the
  session figures.

No session run and no direct run in the window means no block, never zeros.

### The final figure: `tokens_saved_estimate`

One number, owner decision 2026-10-21, defined once in
`telemetry.TokensSavedEstimate`:
`tokens_saved_estimate = Compactor saved_bytes / 4`, in the scope of the view.
Only the Compactor keeps tokens out of the context, so only it is counted. The
tokens that ran below the session model were spent; they are shown apart
(`routing_tokens`) and are never added (before 2026-11-01 they were). The figure is an
estimate and is labelled one everywhere (`~` in the panel). `downshift report` no longer
prints it as a cross-harness line: the Compactor tokens are printed once, per harness. In `session --panel` and in `downshift report` the exec log
is read in the view's own scope and window. The parts stay in the JSON (`tokens`, `compactor`; in `report --json`
`routing_tokens`, `compactor_saved_bytes`, `compactor_tokens_estimate`).

Each run still logs the hashed session when one is known (`telemetry.HashSessionID`,
never the raw id): the hook puts it on the wrapped command
(`downshift compact --session=<hash> -- …`), from the payload's `session_id` or,
when absent, the uuid file name of `transcript_path`. A run with no flag (a
direct `downshift compact -- …` by an agent or a script) carries no session —
it does not fall back to a harness env var. This prevents attributing direct
compactor calls to a harness they did not originate from (audit A22).

Each run also records two new fields (additive; absent in old rows, treated as
`"unknown"` by callers):
- **`harness`**: the harness detected from the environment
  (`HarnessCapabilities.SessionEnv` lookup, e.g. `"claude-code"`, `"kirocrew"`)
  or `"unknown"` when none is found.
- **`entry`**: `"hook"` when `--session=` was set by a harness adapter;
  `"direct"` when the agent or user ran the compactor without `--session=`.

## What a harness cannot honor

Computing the state is not conditional on the harness. What varies is only
display: the child-end line and statusline need `CanShowChildLine` / a
statusline bridge (see [harness-matrix.md](harness-matrix.md)), and a harness
may not show hook stderr in its UI. `live.json`, `downshift watch` and
`downshift session` work for every harness that logs spawns.
