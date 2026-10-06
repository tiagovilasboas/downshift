# Billing Comparison Report Template

Use this template to document real billing comparisons between provider invoices/dashboards and `downshift stats --export` (satisfies **P3.7** in [beta-exit.md](beta-exit.md)).

---

## 1. Metadata

- **Operator / Team:** 
- **Evaluation Period:** YYYY-MM-DD to YYYY-MM-DD (e.g. 1 calendar week)
- **Primary Harness(es):** Claude Code / Cursor / Codex / Antigravity
- **Account Plan Tier:** (e.g. Claude Code Pro / Max / Team, API key)
- **Downshift Version:** (run `downshift --version` or commit SHA)

---

## 2. Experimental Setup

Describe how the test was partitioned:

- [ ] **A/B Split / Alternating weeks:** Week 1 baseline (`DOWNSHIFT_NO_ROUTE=1`) vs Week 2 routed.
- [ ] **Dual environment:** Parallel operator comparison with similar workloads.
- [ ] **Single period with PostToolUse usage tracking:** Real provider spend recorded via hook telemetry (`outcome: "usage"`).

---

## 3. Downshift Export Summary

Run:
```bash
downshift stats --days=7 --export
```

Paste the sanitized JSON export below:

```json
{
  "total": 0,
  "downshift_rate": 0.0,
  "real_cost_events": 0,
  "real_saved_usd": 0.0,
  "usage_linked": 0,
  "baseline_no_route": 0
}
```

---

## 4. Provider Dashboard Evidence

Attach or transcribe numbers from the provider billing dashboard (Anthropic Console / OpenAI Platform / Cursor Settings) for the same period.

| Metric | Provider Dashboard | Downshift Tracked | Variance / Notes |
|---|---|---|---|
| **Total Spend (USD)** | $0.00 | $0.00 | |
| **Input Tokens** | 0 | 0 | |
| **Output Tokens** | 0 | 0 | |
| **Cached Tokens** | 0 | 0 | |

*Note: Redact billing IDs, workspace names, and personal payment methods.*

---

## 5. Analysis & Sign-off

- **Observed savings:** $X.XX (~XX% reduction on subagent spend).
- **Rework / Failure rate:** (Were any tasks retried due to model capacity?)
- **Conclusion:**
  - [ ] Confirms real provider savings consistent with downshift routing decisions.
  - [ ] Ready for maintainer sign-off on Beta Exit Pillar 3.

**Sign-off:** ____________________  **Date:** YYYY-MM-DD
