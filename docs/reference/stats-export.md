# Sharing stats without leaking prompts

Use this when contributing evidence for [beta exit](beta-exit.md) (pillar P2).

> [!NOTE]
> An outside operator sharing one week (issue #104) should use `downshift export` and the
> [operator guide](../guides/operator.md) instead: it prints counts and token totals only and refuses
> to print anything outside a strict allowlist.

## Generate export

```bash
downshift stats --days=30 --export
```

Paste the JSON into an issue or PR. The export contains **counts only** — no task
text, no file paths from your machine. Redact `state_dir` if your paste includes it.

> [!NOTE]
> In `events.jsonl` the row type is identified by the `outcome` field, not a
> `type` field. The native harness transcript format (for example Claude Code
> JSON lines with `{"type":"assistant",...}`) is a distinct schema and is not
> the downshift `events.jsonl` schema; do not look for a `type` field here.

Reason codes `E5_BOOST`, `E5_ABSTAIN`, and `GRAPH_ESCALATION` can appear on logs
written before 2026-10-17. Current builds do not emit them.

Fields useful for graduation tracking:

| Field | Meaning |
|-------|---------|
| `schema_version` | Schema version (`4`: tokens only, no dollar keys; `tokens_rerouted` replaced by `routing_tokens`) |
| `total` | Routing decisions in window |
| `downshift_rate` | Fraction downshifted |
| `observed_child_tokens` | Sum of input, output, and cached tokens from usage events that carry token counts |
| `observed_child_usage` | Count of those usage events; `0` means unobserved, not a measured zero |
| `usage_records` | All post-hoc usage records, including unlinked records |
| `usage_linked` | Distinct usage records matched to routing decisions in this window (same harness and compatible session); duplicate agents/events and orphan references excluded |
| `child_status` | Map of child end status (`completed`, `launched`, `interrupted`, `error`, `stopped`, `unknown`) to count, from `child_ended` events; `{}` when none. Observations only: never part of `total` |
| `baseline_no_route` | Unrouted events (`outcome: baseline`): spawns left unrouted by a session pause, plus baseline lines written by older versions (their control group or no-route switch). The key keeps its name for readers of older exports |
| `agent_types` | Decisions per subagent type, `{"Explore": {"decisions": 3, "unknown_baseline": 2}}`, so a gap in the baseline can be pinned on a kind of spawn. Only Claude Code reports a type. Names are limited to the built-in ones (`general-purpose`, `claude`, `Explore`, `Plan`, `statusline-setup`, `claude-code-guide`), plus `unspecified` and `custom`: a custom agent's own name is never stored or exported. Omitted when no event carries a type |
| `by_entry` | Decisions per spawn-time entry (`hook` for settings/plugin PreToolUse, `mod` for the Claude Code `agent.spawn` path). Empty `entry` on older logs counts as `hook`. Omitted when every count is zero |
| `duplicate_tool_use` | How many distinct `tool_use_hash` values appear on more than one decision (both entry points routed the same spawn). Zero omitted |
| `routing_tokens` | Routing tokens per harness. Always present. Object keyed by harness (`unknown` stays its own row), no total across harnesses; see [Routing tokens](#routing-tokens) |

> [!NOTE]
> Schema version 4 is tokens only (version 3 was too). **Breaking in version 4:** `tokens_rerouted` (its `cheaper`/`pricier`/`unchanged`/`unattributed` buckets, `by_pair`, `by_model` and `totals`) is gone, replaced by `routing_tokens`. The export has no dollar keys: `actual_cost_usd`, `real_saved_usd`, `real_cost_delta_usd`, `real_overspend_usd`, `max_session_cost_usd`, `max_subagent_cost_usd`, `actual_cost_events`, `real_cost_events`, `real_cost_inferred_baseline`, `cost_sessions` and `cost_subagents` are gone. `routing_tokens` is the source for the tokens that ran below the session model.

`downshift stats` text (short and `--detail`) prints no dollar figure and takes no `--cost-per-unit`: its headline is the
routing tokens, one line per harness. `downshift report` shows tokens per completed task and rework for the routed spawns. Machine readers use `routing_tokens` from the export.

## Routing tokens

Downshift does not reduce tokens. A routed child runs about the same tokens on another model;
`routing_tokens` says where they ran against the session model. It is the one routing figure of
every surface (`downshift report`, `stats`, the panels and the Agents pane), per harness and
**never summed across harnesses**. Counts only: no prompt, task, path, session or agent id, and
no model id.

```json
"routing_tokens": {
  "claude-code": {
    "spawns": 12, "below_spawns": 7, "billable": 410000, "below_billable": 250000,
    "fallback_spawns": 2, "fallback_billable": 30000, "fallback_below_billable": 12000,
    "unobserved_spawns": 1, "cache_read": 1800000
  },
  "codex": {"spawns": 0, "below_spawns": 0, "billable": 0, "below_billable": 0,
            "fallback_spawns": 0, "fallback_billable": 0, "fallback_below_billable": 0,
            "unobserved_spawns": 3, "cache_read": 0}
}
```

Population: every usage record with tokens (`outcome: usage`, at least one token), folded per
spawn: a resumed child with several records counts as one spawn.

Baseline per spawn:

| Fields | Baseline |
|--------|----------|
| `spawns`, `billable`, `below_spawns`, `below_billable` | The session model (`parent_model` on the decision), when the catalog ranks it. `below_*` is the part that ran on a lower tier than the session model, whoever chose it: the orchestrator, the agent definition or a Downshift rewrite. |
| `fallback_spawns`, `fallback_billable`, `fallback_below_billable` | The requested model, only for spawns with no usable session model. A different baseline: never added to the session fields. |
| `unobserved_spawns` | Neither baseline could be ranked (no linked decision, no session model, a model the catalog does not know). |

`billable` is input + output + cache write. `cache_read` is its own field and is never inside
`billable` or `below_*`. The only comparator is the catalog tier (small < mid < frontier), the same
for native and gateway ids. A harness with no spawn is absent: read it as unobserved, never 0.

`downshift stats` prints one `<harness>: <N> of <M> billable below the session model (...)` line per
harness, or `tokens not observed yet` when no priced usage exists; `--detail` adds the same rows with
their fallback, unobserved and cache-read lines. `observed_child_tokens` still counts every record.

## Routing note and tokens per harness (`downshift report`)

`downshift report` (text and `--json`) carries two sections with no money in them, built from the
same events and window as the rest of the report.

- `why_spawns_changed`: per harness, `decisions`, `recorded`, `not_recorded`, `by_verdict` and
  `top_reasons` (at most five). Keys are enums, tiers and declared rule or profile ids; any value that
  is not an identifier is dropped when read, so prose, a path or a prompt cannot reach it. The source
  is the optional `routing_note` object on a decision event (`verdict`, `reason`, `rule_id`,
  `profile`, `tier_from`, `tier_to`). Events written before it, or by a path that does not record it,
  count under `not_recorded`.
- `tokens_by_harness`: per harness `routing_observed` with `routing` (the same row as
  [Routing tokens](#routing-tokens)) and `quota_used` (`status` `native` or `unknown`,
  `five_hour_pct`, `week_pct`, `observed_at`, `stale`). **Breaking:** `moved_observed`,
  `tokens_moved_to_smaller_model`, `below_session_observed` and `below_session` are gone, and
  `compactor_tokens_avoided` moved out (it is printed once, under `compaction_by_harness`).
- `compaction_by_harness`: per harness `compactor_tokens_avoided` (estimated, saved bytes / 4).
  The text report prints the same figure once, in its leading section; there is no total across
  harnesses.
- `routing_tokens` (top level of the report): the same per-harness rows, replacing `tokens_saved`
  and `effect`.

## Before you paste

- Redact `generated_at` if you prefer (optional).
- Do **not** attach raw `events.jsonl` publicly unless you have reviewed every field.
- Note harness + plan (e.g. Claude Code Max, Cursor Pro usage-based).


`usage_linked` previously counted every usage record. Use `usage_records` for
that raw total. Linked records are deduplicated by hashed agent ID when present,
otherwise by the usage event correlation ID (legacy fallback: timestamp and
session). Two distinct agents can contribute two linked records. Only the
corrected `usage_linked` is suitable for the P3.4
linked-decision gate. A decision outside the selected window contributes no
link in that window. Token totals still include usage without a link;
linkage alone does not prove the requested baseline is known or a bill changed.

Unknown requested models get no tier comparison: their tokens are `unattributed`. New usage events write no dollar
figure at all. Older lines may still carry `actual_cost_usd` and `baseline_cost_usd`; the reader ignores them and they are never exported.

Tokens are the harness-reported counts. The tier comparison is a counterfactual, not a second measured execution, and
nothing in the export is a provider invoice.

`observed_child_tokens` and `observed_child_usage` are the end-of-day child
token sum and how many usage records contributed. They are not billing; pair
`observed_child_usage == 0` with "unobserved" in reports, never as zero tokens
observed.

`policy_floor` counts decisions an operator tier floor raised (`.downshift/policy.json`, see [config.md](../guides/config.md)); it is recorded by the shared runner for every harness.

`reason_codes` counts decisions per reason code (absent when no event in the window carries codes). Each decision event records `routing_policy` (currently `deterministic-v1`) and `reason_codes`, a sorted list of stable codes derived from the decision: `CLASSIFIER_CONFIDENT` or `CLASSIFIER_UNCONFIDENT`, `NO_SIGNAL`, `SEMANTIC_BOOST` (the in-process hash boost raised the complexity), `TASK_FOCUS` (a long brief was read by its stated task, with the rest weighed as context; see [complexity-weights.md](complexity-weights.md)), `RISK_CATEGORY`, `RISK_FLOOR`, `NB_DOWNSHIFT_APPLIED`, `NB_UPSHIFT_APPLIED` (legacy: the NB upshift opinion was removed 2026-10-31 and is no longer emitted on fresh events, but old events carry it and still count), `TIER_FLOOR`, `CREDIT_HELD`, `PAUSED` (routing paused for the session), `SCORE_ROUTED` (the opt-in score path picked the model), `HELD_GUESSED_BASELINE` (a downshift held because its baseline was guessed, Claude Code settings or Cursor's parent `model_id`, and the target was not the small tier), `BASELINE_FROM_SETTINGS` (Claude Code: the baseline model was inferred from the configured model because the transcript had none), and one `HELD_*` code per guardrail in `corrections` (for example `HELD_UNCONFIDENT_UNKNOWN_MID` for R9; `HELD_UNMAPPED_RULE` for a guardrail this version cannot name). Codes never carry prompt text, matched words, risk category names or paths. Older events may also carry `e5_shadow` and `e5_fuse` objects; those experiments were removed (2026-11-01) and the reader ignores both keys. There is no top-level `confidence`: the router has no calibrated probability. Events written before the trace have neither key and are counted as before.


`baseline_source` on cost events is `requested` when the baseline is the model the harness asked for, and `inferred` when it was guessed from the configured model because the harness named none (`BASELINE_FROM_SETTINGS`). Records written before this distinction say `requested` for both; `stats` also checks the linked decision's reason codes. A rewrite that a linked `resolved` record refutes (`rewrite_honored: false`) is counted as not applied: no saving, and never inferred as honored.

`UNSUPPORTED_LANGUAGE` means the language guard detected text outside the English
and Portuguese rule coverage and withheld confidence. It appears in decision
`reason_codes`, their aggregate counts and `downshift why`; `stats` also shows the
language-guard count. It records no detected words, prompt text or language name.
The code reports a heuristic guard hit, not a complete language census. A
no-signal abstention can instead carry `NO_SIGNAL` without this code. See
[language coverage](complexity-weights.md#language-coverage).

`child_ended` events record how a Claude Code Agent/Task child ended. Their `child_status` is one of `completed`, `launched`, `interrupted`, `error`, `stopped` or `unknown`. They are observations, not decisions: they never change `total`, the verdict counts or savings. The error text is never stored. Recording them requires `claude-code-post-tool-use` registered under `PostToolUseFailure` as well as `PostToolUse`; the failure payload was documented but not captured from a real session, so the field names should be checked against one before relying on them.

> [!NOTE]
> **Removed from the event format on 2026-10-31 (owner decision: the Downshift product uses no price).**
> The `events.jsonl` input records no longer carry `estimated_savings`, `gateway_price` or
> `upshift_cost_ratio`; the catalog list-price `norm_baseline`/`norm_routed` lens that read
> `upshift_cost_ratio` is gone with them. Old event lines that still carry these keys stay
> readable: the event decoder ignores unknown keys, so those lines still enter every report.
> The export summary schema is unchanged by this: none of the three were export keys, so
> `schema_version` was `3` at that point; it is `4` since `tokens_rerouted` was replaced by `routing_tokens`.
