# Sharing stats without leaking prompts

Use this when contributing evidence for [beta exit](beta-exit.md) (pillar P2).

## Generate export

```bash
downshift stats --days=30 --export
```

Paste the JSON into an issue or PR. The export contains **counts only** — no task
text, no file paths from your machine. Redact `state_dir` if your paste includes it.

Reason codes `E5_BOOST`, `E5_ABSTAIN`, and `GRAPH_ESCALATION` can appear on logs
written before 2026-10-17. Current builds do not emit them. `shadow-report`
summarises routing-v2 candidate observations, not an E5 fuse.

Fields useful for graduation tracking:

| Field | Meaning |
|-------|---------|
| `total` | Routing decisions in window |
| `downshift_rate` | Fraction downshifted |
| `actual_cost_events` | Events with observed token usage priced against a known routed model, including unknown-baseline events |
| `actual_cost_usd` | Sum of observed token costs at catalog list prices (not provider invoices) |
| `real_cost_events` | Events with both actual and baseline catalog-priced USD |
| `real_cost_inferred_baseline` | Of those, events whose baseline was guessed from settings (`baseline_source: inferred`), not requested by the harness |
| `real_saved_usd` | Sum of baseline − actual (catalog pricing), each event clamped at zero, so it never goes negative |
| `real_cost_delta_usd` | Signed sum of baseline − actual over the same events. Negative means the router cost more than the baseline overall. Omitted when `0` |
| `real_overspend_usd` | Sum of the negative per-event deltas as a positive number: money spent above baseline. Omitted when `0` |
| `observed_child_tokens` | Sum of input, output, and cached tokens from usage events that carry token counts |
| `observed_child_usage` | Count of those usage events; `0` means unobserved, not a measured zero |
| `usage_records` | All post-hoc usage records, including unlinked records |
| `usage_linked` | Distinct usage records matched to routing decisions in this window (same harness and compatible session); duplicate agents/events and orphan references excluded |
| `child_status` | Map of child end status (`completed`, `launched`, `interrupted`, `error`, `stopped`, `unknown`) to count, from `child_ended` events; `{}` when none. Observations only: never part of `total` |
| `baseline_no_route` | Unrouted events: `DOWNSHIFT_NO_ROUTE`, a session pause, and the built-in control group (`outcome: baseline`; the control group also carries reason code `CONTROL`) |
| `agent_types` | Decisions per subagent type, `{"Explore": {"decisions": 3, "unknown_baseline": 2}}`, so a gap in the baseline can be pinned on a kind of spawn. Only Claude Code reports a type. Names are limited to the built-in ones (`general-purpose`, `claude`, `Explore`, `Plan`, `statusline-setup`, `claude-code-guide`), plus `unspecified` and `custom`: a custom agent's own name is never stored or exported. Omitted when no event carries a type |
| `by_entry` | Decisions per spawn-time entry (`hook` for settings/plugin PreToolUse, `mod` for the Claude Code `agent.spawn` path). Empty `entry` on older logs counts as `hook`. Omitted when every count is zero |
| `duplicate_tool_use` | How many distinct `tool_use_hash` values appear on more than one decision (both entry points routed the same spawn). Zero omitted |
| `dci` | Object containing Downshift Code Intelligence telemetry: `prompts_evaluated`, `injections_applied`, `injection_rate`, `injection_pct`, `abstentions` (and breakdown: `conversational_greeting`, `chatter`, `no_candidates`, `insufficient_evidence`), `context_bytes_emitted`, `avg_bytes_per_injection`, `latency_ms_p50`, and `latency_ms_p95`. Strictly observed empirical measurements; zero synthetic token counts, zero fictitious dollar estimates. Omitted when no DCI events exist in the window |
| `tokens_rerouted` | The token facts behind the savings, per harness and per model pair. Always present. Object `{basis, by_harness, totals}`; see [Tokens rerouted](#tokens-rerouted) |

## Tokens rerouted

Downshift does not reduce tokens. A routed child runs about the same tokens on another model;
the dollar figures (`real_saved_usd` and friends) are those tokens priced on the model the
harness asked for, a counterfactual. `tokens_rerouted` shows the tokens themselves, so the
dollars can be read against them. It never says "tokens saved". Counts only: no prompt, task,
path, session or agent id. Model ids are the bounded ids already in the log (`unknown` when the
catalog did not recognise one).

```json
"tokens_rerouted": {
  "basis": "tokens as reported by the harness; dollars elsewhere are ...",
  "by_harness": {
    "claude-code": {
      "usage_source": "transcript",
      "status": "observed",
      "cheaper":      {"records": 12, "input": 40000, "output": 90000, "cache_read": 1000000, "cache_write": 100000, "total": 1230000},
      "pricier":      {"records": 0, "input": 0, "output": 0, "cache_read": 0, "cache_write": 0, "total": 0},
      "unchanged":    {"records": 30, "...": 0},
      "unattributed": {"records": 1, "...": 0},
      "by_pair": [{"requested_model": "<id>", "ran_model": "<id>", "direction": "cheaper", "records": 12, "input": 0, "output": 0, "cache_read": 0, "cache_write": 0, "total": 0}],
      "by_model": [{"model": "<id>", "records": 12, "input": 0, "output": 0, "cache_read": 0, "cache_write": 0, "total": 0}]
    },
    "cursor": {"usage_source": "none", "status": "unobserved"}
  },
  "totals": {"cheaper": {}, "pricier": {}, "unchanged": {}, "unattributed": {}}
}
```

Population: every **priced** usage record with tokens (`outcome: usage`, `actual_cost_usd` set,
at least one token). Each lands in exactly one bucket, so the four buckets of a harness add up to
its token mix:

| Bucket | What lands there |
|--------|------------------|
| `cheaper` | The linked decision is an **applied** rewrite (rewrite emitted, no guardrail correction), the harness did not refute it, and the record's baseline cost (the same tokens on the requested model) is above its actual cost. |
| `pricier` | Same, but baseline cost is below actual cost: an applied upshift. |
| `unchanged` | The child ran on the requested model, or at its price: an allow, a held or blocked decision, the control group (`CONTROL`), `DOWNSHIFT_NO_ROUTE` or a pause, a rewrite a linked `resolved` record refutes, or an applied rewrite whose two costs are equal. |
| `unattributed` | No linked decision in the window, or an applied rewrite whose requested model has no price (no `baseline_cost_usd`). Never counted as rerouted. |

Link and refutation rules are the ones `real_saved_usd` uses (`AppliedRewrite`, the refuted
index, `RoutedCost`), so a record that counts as a saving is the one that counts as `cheaper`.

Token classes, per bucket: `input`, `output`, `cache_read` (cached minus cache write), `cache_write`
(cache creation, clamped to the cached total), `total` (their sum) and `records` (usage records).
`cache_read` equals the whole cached count when the write split is unknown, the same rule
`CostUSD` prices by.

- `by_pair`: `cheaper`, `pricier` and `unchanged` tokens per `requested_model` to `ran_model`
  pair and `direction`, largest first. Unattributed records have no pair. A refuted rewrite ran
  on the requested model, so its pair is requested to requested.
- `by_model`: the **token mix**, every priced usage record of the harness by the model that ran,
  largest first.
- `usage_source` is `core.HarnessCapabilities.UsageSource` (`payload`, `transcript`, `none`), read
  from the capability table, not from a harness name. `status` is `observed` (priced records in
  the window), `none_in_window` (the harness has a token seam but no record in this window) or
  `unobserved` (no token seam, no record). Only an `observed` harness carries the bucket objects,
  `by_pair` and `by_model`; an `unobserved` one has none, so it never reads as a measured zero.
  Every harness in `core.AllHarnessCaps()` is listed; one whose declared source is `none` or empty
  (today Cursor, Codex, OpenCode, KiroCrew and Antigravity) is `unobserved`. A harness outside the table with records appears with `usage_source: unknown`.
- `totals`: the four buckets summed over the observed harnesses; absent when none is observed.

`downshift stats` prints the headline-adjacent line `Tokens moved to a cheaper model: 1.2M (in
40k / out 90k / cache 1.1M) · to a pricier model: 0` (cache is read plus write), or `unobserved`
when no priced usage exists; `--detail` adds the per-harness and per-pair table. Records without
a price are not in these figures; `observed_child_tokens` still counts them.

## Before you paste

- Redact `generated_at` if you prefer (optional).
- Do **not** attach raw `events.jsonl` publicly unless you have reviewed every field.
- Note harness + plan (e.g. Claude Code Max, Cursor Pro usage-based).

## Built-in control group

Since the control group is on by default, a share of spawns (`DOWNSHIFT_CONTROL_SHARE`, default
`0.10`, `0` disables) is classified and logged but never rewritten, with reason code `CONTROL`.
`downshift report` compares those spawns with the routed ones: cost per completed task, rework
rate and savings with a 95% bootstrap interval. Reason code `CONTROL` is documented in
[report.md](../guides/report.md); it is not counted under `reason_codes`, because baseline events are
excluded from decision counts.

## Control-group week (pillar P3)

Hand-run alternative when you want a whole week unrouted. Alternate routing on/off:

```bash
# Week A — normal routing (default)
# Week B — control group
export DOWNSHIFT_NO_ROUTE=1
```

Compare two `--export` blobs and your provider billing dashboard for the same dates.

`usage_linked` previously counted every usage record. Use `usage_records` for
that raw total. Linked records are deduplicated by hashed agent ID when present,
otherwise by the usage event correlation ID (legacy fallback: timestamp and
session). Two distinct agents can contribute two linked records. Only the
corrected `usage_linked` is suitable for the P3.4
linked-decision gate. A decision outside the selected window contributes no
link in that window. Cost totals still include priced usage without a link;
linkage alone does not prove the requested baseline is known or a bill changed.

Unknown requested models leave `baseline_cost_usd` absent on new usage events.
Their observed spend remains in `actual_cost_usd`/`actual_cost_events`, but they
contribute neither a comparison to `real_cost_events` nor savings to
`real_saved_usd`. Existing historical cost values are preserved.

`CostUSD` prices cached input at the model's cache-read price when the catalog
has one (`cache_read_cost_per_1m`, taken from a dated price snapshot). When the
catalog has no cache price for a model, cached tokens fall back to the full
input rate, which is an upper bound. These amounts are catalog-list-price
estimates from observed token counts, not provider invoices. Baseline and routed estimates use the same observed tokens;
the baseline is a counterfactual, not a second measured execution.

`observed_child_tokens` and `observed_child_usage` are the end-of-day child
token sum and how many usage records contributed. They are not billing; pair
`observed_child_usage == 0` with "unobserved" in reports, never as zero tokens
observed.

`policy_floor` counts decisions an operator tier floor raised (`.downshift/policy.json`, see [config.md](../guides/config.md)); it is recorded by the shared runner for every harness.

`reason_codes` counts decisions per reason code (absent when no event in the window carries codes). Each decision event records `routing_policy` (currently `deterministic-v1`) and `reason_codes`, a sorted list of stable codes derived from the decision: `CLASSIFIER_CONFIDENT` or `CLASSIFIER_UNCONFIDENT`, `NO_SIGNAL`, `SEMANTIC_BOOST` (the in-process hash boost raised the complexity), `TASK_FOCUS` (a long brief was read by its stated task, with the rest weighed as context; see [complexity-weights.md](complexity-weights.md)), `RISK_CATEGORY`, `RISK_FLOOR`, `NB_UPSHIFT_APPLIED`, `NB_DOWNSHIFT_APPLIED`, `TIER_FLOOR`, `CREDIT_HELD`, `PAUSED` (routing paused for the session), `SCORE_ROUTED` (the opt-in score path picked the model), `HELD_GUESSED_BASELINE` (a downshift held because its baseline was guessed, Claude Code settings or Cursor's parent `model_id`, and the target was not the small tier), `BASELINE_FROM_SETTINGS` (Claude Code: the baseline model was inferred from the configured model because the transcript had none), and one `HELD_*` code per guardrail in `corrections` (for example `HELD_UNCONFIDENT_UNKNOWN_MID` for R9; `HELD_UNMAPPED_RULE` for a guardrail this version cannot name). Codes never carry prompt text, matched words, risk category names or paths. Two optional objects sit beside the codes when their flag is on: `e5_shadow` (`DOWNSHIFT_E5_SHADOW=1`: the parallel observation, never a routing change) and `e5_fuse` (`DOWNSHIFT_E5_FUSE=1`: `model_id`, `nearest_label`, `similarity`, `action` of `none`, `abstain` or `raise`, `latency_ms`, `error_code`), both prompt-free and absent otherwise. There is no top-level `confidence`: the router has no calibrated probability. Events written before the trace have neither key and are counted as before.

`downshift shadow-report` (maintainer command; `--telemetry=<events.jsonl>` to point at a log, `--text` for a readable form) summarises those two objects from decision events, so E5 can be judged in shadow mode on a real log before any enable decision. The JSON carries `e5_shadow` and `e5_fuse` only when at least one event has the object, and the text output prints the headings "E5 shadow" and "E5 fuse" under the same rule, so a log without E5 reads exactly as before. `e5_shadow` gives `observed`, the `error_codes` histogram, `scored` (helper answered: the denominator of every rate), `would_lift` and `would_lift_rate` overall and `by_regex_label`, the `nearest_labels` histogram, `abstain` and `abstain_share` (nearest `ABSTAIN` or `FOREIGN`), `risk_decisions` (reason code `RISK_CATEGORY` or `RISK_FLOOR`) and `false_downgrade_proxy`: risk decisions where `would_lift` is false and the nearest label ranks below `regex_label` on TRIVIAL < SIMPLE < MEDIUM < COMPLEX. That last number is a proxy, not a measured error: the shadow changed no route and no reviewer labelled the task; it flags the pattern that would matter with the fuse on. `latency_ms` is nearest-rank p50, p95 and max over scored observations (a timeout shows in `error_codes`, not in the median). `e5_fuse` gives `observed`, the `actions` histogram (`none`, `abstain`, `raise`), `errors` with `error_codes`, and the same latency fields over answered observations. Both summaries are built from those closed field sets alone; no prompt, path or helper output exists in the input.

`baseline_source` on cost events is `requested` when the baseline is the model the harness asked for, and `inferred` when it was guessed from the configured model because the harness named none (`BASELINE_FROM_SETTINGS`). Records written before this distinction say `requested` for both; `stats` also checks the linked decision's reason codes. A rewrite that a linked `resolved` record refutes (`rewrite_honored: false`) is counted as not applied: no saving, and never inferred as honored.

`UNSUPPORTED_LANGUAGE` means the language guard detected text outside the English
and Portuguese rule coverage and withheld confidence. It appears in decision
`reason_codes`, their aggregate counts and `downshift why`; `stats` also shows the
language-guard count. It records no detected words, prompt text or language name.
The code reports a heuristic guard hit, not a complete language census. A
no-signal abstention can instead carry `NO_SIGNAL` without this code. See
[language coverage](complexity-weights.md#language-coverage).

`child_ended` events record how a Claude Code Agent/Task child ended. Their `child_status` is one of `completed`, `launched`, `interrupted`, `error`, `stopped` or `unknown`. They are observations, not decisions: they never change `total`, the verdict counts or savings. The error text is never stored. Recording them requires `claude-code-post-tool-use` registered under `PostToolUseFailure` as well as `PostToolUse`; the failure payload was documented but not captured from a real session, so the field names should be checked against one before relying on them.

`cost_sessions`, `cost_subagents`, `max_session_cost_usd` and `max_subagent_cost_usd` summarise observed, catalog-priced cost grouped by the hashed session id and agent id on usage records. Records without an id are in the totals but not in the groups. No individual session id is exported. These are estimates, not invoices.

`real_cost_delta_usd` and `real_overspend_usd` are additive: older readers ignore them, and older exports lack them. Read `real_saved_usd` together with `real_overspend_usd`, since the first hides overspend by clamping each event. `downshift stats` prints both next to "Real saved".

The normalised units (`norm_baseline`, `norm_routed`) charge an applied UPSHIFT its extra catalog list-price cost (the `upshift_cost_ratio` recorded on the decision event, routed over requested price). When either price is unknown the upshift stays neutral at 1.0, so `norm_routed` can exceed `norm_baseline` and the normalised savings can be negative.
