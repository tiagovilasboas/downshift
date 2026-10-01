# Sharing stats without leaking prompts

Use this when contributing evidence for [beta exit](BETA-EXIT.md) (pillar P2).

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
| `real_cost_events` | Events with provider USD |
| `real_saved_usd` | Sum of baseline − actual (catalog pricing) |
| `usage_linked` | PostToolUse usage records |
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
