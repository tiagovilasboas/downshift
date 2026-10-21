# Spawn state: when Downshift works and when it switches

Downshift by Tiago Vilas Boas
https://github.com/tiagovilasboas/downshift

Every subagent spawn the hook sees gets exactly one **state**, computed once in
`internal/telemetry/spawn_state.go` from the decision event. Every harness
gets it through the shared runner (`runHookAdapter`); no adapter holds any of it.

| State | Meaning |
|---|---|
| `switched` | Downshift changed the spawn's model or effort |
| `held` | A change was considered; a guardrail (`HELD_*`), the credit gate or a guessed baseline kept the spawn as it was. Doubt never spends money |
| `observed` | Downshift looked and had nothing to change |
| `paused` | `downshift session --pause`: classified and logged, never changed |
| `control` | Control group or `DOWNSHIFT_NO_ROUTE=1`: classified, never changed |
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
    "to_model": "claude-haiku-4-5",
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
{"session":"a1b2c3d4","switched":2,"held":1,"observed":4,
 "tokens":{"children":4,"saved_children":2,"work_tokens":5200,"work_saved":3200,
           "cache_tokens":180000,"cache_saved":104000},
 "tokens_line":"saved 62% of work · 58% of cache",
 "by_agent_type":[{"agent_type":"general-purpose","tokens":{...},
                   "line":"general-purpose: work 70% · cache 64% (2 of 3 agents)"}],
 "spawns":[{"tool_use":"<sha256 hex>","state":"switched",
            "from_model":"claude-opus-5-5","to_model":"claude-sonnet-5-5",
            "label":"opus → sonnet (mid)",
            "tokens":{...},"tokens_label":"48k tokens · saved from opus"}]}
```

Labels: `opus → sonnet (mid)`, `held: original model kept`, `kept`,
`control (unrouted)`, `paused (unrouted)`, `blocked: respawn on another model`.
`tool_use` is `telemetry.ToolUseKey(tool_use_id)`: sha256 of
`harness-downshift-session-v1:` + the harness tool call id.

`observed_model` is optional and per spawn (`telemetry.PanelSpawn`): the model
the child was observed to run on, taken from its resolved record or else its
usage record. It is set only when that model is known and differs from
`to_model`; an equal, dated-equal, unknown or missing observation leaves the key
out. It holds model ids only, never prompts.

Tokens, never dollars (owner decision 2026-10-19). "Saved" means a child's
tokens ran on a cheaper model than the one requested (`telemetry.TokenSavings`,
the same buckets as `stats --export` `tokens_rerouted`); work (input+output)
and cache (read+write) are counted apart. `tokens_line` and `tokens_label` are
`""` until a child's tokens are known; a reader then draws no figure. A child on
a pricier model reads `pricier than <model>` and adds to `work_pricier` /
`cache_pricier`, never to the saved share. Control, held, kept and refuted
children count in the totals. Harnesses with no proven token source (Cursor,
Codex, OpenCode today) have no token figures, never zeros. No dollars:
`downshift report` is tokens only.

The same figures appear in `downshift session` (header, a tokens column per row,
a `by type` line, shown only for types that saved something), the statusline (`saved N% of work · M% of cache`),
`downshift watch`, `dsmon`, the child-end line (`48k tokens (work 2.1k,
cache 46k) · saved from opus`) and the `stats` headline (`downshift stats`; `stats --detail` for more).
`downshift try` has no tokens for one decision and says only where they would
run: `→ small: opus → haiku · its tokens would run on a cheaper model than
requested`, or `(not applied, tokens stay on opus)` when the hook would leave
the spawn alone.

### Compactor in the panel JSON

`session --panel` adds an optional `compactor` block that totals the **whole**
`exec-events.jsonl` (owner decision 2026-10-21): every session and every run,
with or without a session hash. It does not depend on `--session`; the routing
figures beside it stay per session.
`{scope, commands, compacted, raw_bytes, saved_bytes, estimated_tokens_saved, token_estimate}`,
where `scope` is `"all"`. Bytes are measured; `estimated_tokens_saved` is
`saved_bytes / 4`, an estimate. An empty or missing log means no block, never zeros.

### The final figure: `tokens_saved_estimate`

One number, owner decision 2026-10-21, defined once in
`telemetry.TokensSavedEstimate`:
`tokens_saved_estimate = routing tokens saved + Compactor saved_bytes / 4`.
Routing tokens are `work_saved + cache_saved` (tokens that ran on a cheaper
model than requested, measured); the Compactor part is bytes / 4, so the sum is
an estimate and is labelled one everywhere (`~` in the panel,
`Tokens saved (estimate)` in `downshift report`). In `session --panel` the scope
is routing for **this session** plus the Compactor total of the **whole exec
log** (the same scopes the tiles had). In `downshift report` it is routing for
the `--days` window plus the Compactor all-time total (the exec log is not
windowed). The parts stay in the JSON (`tokens`, `compactor`; in `report --json`
`tokens_saved`, `compactor_saved_bytes`, `compactor_tokens_estimate`).

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

### `mods/savvy-progress`

Vendored, so it is patched only through its documented local patch
(`mods/savvy-progress/downshift-local.patch`, README section "Downshift local
patch"; mod version 1.2.12). The patch runs `downshift session --panel` and shows
one tile for the saving. It reads `Tokens saved` and draws `tokens_saved_estimate`
with a `~` (an estimate); the Tokens and Time tiles stay. With an unknown session
the figure is withheld. The mod never splits the total. The
header and the band show one line, `Downshift · N switched · M held`, and each
agent row shows a one-word status, ↓ and the tokens that agent moved to a smaller
model (`work_saved` + `cache_saved`). `tokens_line`, `tokens_label`,
`by_agent_type` and `observed_model` stay in the contract for the CLI; the mod
does not draw them. With no binary or bad output it draws what upstream draws. No
routing logic lives in the mod.

## What a harness cannot honor

Computing the state is not conditional on the harness. What varies is only
display: the child-end line and statusline need `CanShowChildLine` / a
statusline bridge (see [harness-matrix.md](harness-matrix.md)), and a harness
may not show hook stderr in its UI. `live.json`, `downshift watch` and
`downshift session` work for every harness that logs spawns.
