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
| `real_saved_usd` | Sum of baseline − actual (catalog pricing) |
| `usage_records` | All post-hoc usage records, including unlinked records |
| `usage_linked` | Distinct usage records matched to routing decisions in this window (same harness and compatible session); duplicate agents/events and orphan references excluded |
| `baseline_no_route` | Control-group events (`DOWNSHIFT_NO_ROUTE`) |

## Before you paste

- Redact `generated_at` if you prefer (optional).
- Do **not** attach raw `events.jsonl` publicly unless you have reviewed every field.
- Note harness + plan (e.g. Claude Code Max, Cursor Pro usage-based).

## Control-group week (pillar P3)

Alternate routing on/off:

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

`CostUSD` prices cached input at the full input rate. These amounts are
conservative catalog estimates from observed token counts, not cache-discounted
provider charges. Baseline and routed estimates use the same observed tokens;
the baseline is a counterfactual, not a second measured execution.
