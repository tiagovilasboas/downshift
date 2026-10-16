# Sharing stats without leaking prompts

Use this when contributing evidence for [beta exit](beta-exit.md) (pillar P2).

## Generate export

```bash
downshift stats --days=30 --export
```

Paste the JSON into an issue or PR. The export contains **counts only** — no task
text, no file paths from your machine.

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

`reason_codes` counts decisions per reason code (absent when no event in the window carries codes). Each decision event records `routing_policy` (currently `deterministic-v1`) and `reason_codes`, a sorted list of stable codes derived from the decision: `CLASSIFIER_CONFIDENT` or `CLASSIFIER_UNCONFIDENT`, `NO_SIGNAL`, `SEMANTIC_BOOST`, `RISK_CATEGORY`, `RISK_FLOOR`, `GRAPH_ESCALATION`, `NB_UPSHIFT_APPLIED`, `NB_DOWNSHIFT_APPLIED`, `TIER_FLOOR`, `CREDIT_HELD`, `PAUSED` (routing paused for the session), `SCORE_ROUTED` (the opt-in score path picked the model), `HELD_GUESSED_BASELINE` (a downshift held because its baseline was guessed, Claude Code settings or Cursor's parent `model_id`, and the target was not the small tier), `BASELINE_FROM_SETTINGS` (Claude Code: the baseline model was inferred from the configured model because the transcript had none), and one `HELD_*` code per guardrail in `corrections` (for example `HELD_UNCONFIDENT_UNKNOWN_MID` for R9; `HELD_UNMAPPED_RULE` for a guardrail this version cannot name). Codes never carry prompt text, matched words, risk category names or paths. There is no top-level `confidence`: the router has no calibrated probability. Events written before the trace have neither key and are counted as before.

`baseline_source` on cost events is `requested` when the baseline is the model the harness asked for, and `inferred` when it was guessed from the configured model because the harness named none (`BASELINE_FROM_SETTINGS`). Records written before this distinction say `requested` for both; `stats` also checks the linked decision's reason codes. A rewrite that a linked `resolved` record refutes (`rewrite_honored: false`) is counted as not applied: no saving, and never inferred as honored.

`child_ended` events record how a Claude Code Agent/Task child ended. Their `child_status` is one of `completed`, `launched`, `interrupted`, `error`, `stopped` or `unknown`. They are observations, not decisions: they never change `total`, the verdict counts or savings. The error text is never stored. Recording them requires `claude-code-post-tool-use` registered under `PostToolUseFailure` as well as `PostToolUse`; the failure payload was documented but not captured from a real session, so the field names should be checked against one before relying on them.

`cost_sessions`, `cost_subagents`, `max_session_cost_usd` and `max_subagent_cost_usd` summarise observed, catalog-priced cost grouped by the hashed session id and agent id on usage records. Records without an id are in the totals but not in the groups. No individual session id is exported. These are estimates, not invoices.

`real_cost_delta_usd` and `real_overspend_usd` are additive: older readers ignore them, and older exports lack them. Read `real_saved_usd` together with `real_overspend_usd`, since the first hides overspend by clamping each event. `downshift stats` prints both next to "Real saved".

The normalised units (`norm_baseline`, `norm_routed`) charge an applied UPSHIFT its extra catalog list-price cost (the `upshift_cost_ratio` recorded on the decision event, routed over requested price). When either price is unknown the upshift stays neutral at 1.0, so `norm_routed` can exceed `norm_baseline` and the normalised savings can be negative.
