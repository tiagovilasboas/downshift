# Score-gap recheck — 2026-10-10

Branch `measure/score-gaps` (classifier commit `5a888e7af6b074f46aa7b816944e1d1131f921c9`).
Observed facts only; no classifier tuning in this pass.

## Issue #101 — P1.6 Cursor rewrite honored

| Check | Status |
|---|---|
| [Harness matrix](../harness-matrix.md) Cursor Pro/Ultra usage-based | **Unconfirmed** (unchanged) |
| [cursor-rewrite-honored-2026-10-10.md](cursor-rewrite-honored-2026-10-10.md) | Unchanged: child executor model **not present** in the subagent transcript; `rewrite_honored` absent on the spawn row |

P1.6 and GAP-CURSOR-HONOR remain open.

## Issue #102 — Usage-linked sample size (P3.4 / export hygiene)

On **2026-10-10**, maintainer machine:

```text
downshift stats --days=7 --export
```

Observed export fields (not inflated to 50):

| Field | Value |
|---|---:|
| `usage_linked` | 5 |
| `usage_records` | 15 |
| `baseline_no_route` | 0 |

Directional dogfood only; not a billing control week.

## Issue #103 — Billing comparison template (P3.7)

[docs/billing-comparison-template.md](../billing-comparison-template.md) still has empty placeholders (operator, dates, dashboard table at zero). No filled control-week report exists in-repo; the comparison run was **not** executed.

## Issue #104 — External operator exports (P2.3)

No third-party or external-operator billing/stats exports were collected for this recheck. P2.3 stays **open**.
