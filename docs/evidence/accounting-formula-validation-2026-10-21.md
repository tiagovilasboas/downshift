<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# S0: Accounting formula validation — four-arm battery part 1 (2026-10-21)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Status: completed 2026-10-21. Script: `downshift-labs` (private).**

---

## Method

Battery: `~/.downshift-eval/runs/four-arm-part1-2026-10-21/`, 46 runs across 4 tasks
(`mech-constant`, `mech-doc-comments`, `mech-rename`, `mech-test-mirror`), arms
`control` (B), `compact` (C), `routed` (D), `opus` (A), 3 reps each (A/B/C/D × 3
reps × 4 tasks ≈ 46 usable).

Formula under test (`net_counterfactual_cost_v1`):

```
C_actual   = Σ price(model_i, tokens_i)          # actual models and tokens
C_baseline = price(requested_model, Σ tokens_i)   # all tokens at session model
Saved      = C_baseline − C_actual
```

Requested model for arms B/C/D: `claude-sonnet-5-5` (the session model).
Ground truth for `C_actual`: Claude Code's own `costUSD` per model from
`result.json → models[model_id].cost_usd`.

Comparison: `Saved_formula(arm)` vs `measured_saving(arm) = B_mean_cost − arm_cost`
for arms C and D. 24 comparable pairs (arms with at least one rep that used a
non-requested model; sonnet-only runs have `Saved=0` by definition and are excluded
from the bias calculation as the formula correctly returns 0 when no downshift
occurred).

---

## Results

| Task | Arm | Rep | Measured B−arm | Formula Saved | Bias | Models used |
|---|---|---|---|---|---|---|
| mech-doc-comments | routed | 1 | 0.34854 | 0.32168 | −0.027 | haiku+sonnet |
| mech-doc-comments | routed | 2 | 0.30977 | 0.41256 | +0.103 | haiku+sonnet |
| mech-rename | compact | 1 | 0.48356 | 0.53207 | +0.049 | haiku+sonnet |
| mech-rename | compact | 2 | 0.49289 | 0.67246 | +0.180 | haiku+sonnet |
| mech-rename | compact | 3 | 0.46386 | 0.51711 | +0.053 | haiku+sonnet |
| mech-rename | routed | 1 | 0.45176 | 0.59344 | +0.142 | haiku+sonnet |
| mech-rename | routed | 2 | 0.48493 | 0.49133 | +0.006 | haiku+sonnet |
| mech-rename | routed | 3 | 0.30099 | 0.26824 | −0.033 | haiku+sonnet |
| mech-test-mirror | routed | 1 | 0.28150 | 0.32427 | +0.043 | haiku+sonnet |

**Runs excluded from bias (formula correctly = 0):** 15 compact/routed runs where
arm ran 100% on the requested model (`downshifts: 0`). These arms cost less than B
due to natural run-to-run variance, not a routing saving. The formula is correct
to return 0.

### Bias summary (n=9 runs with actual downshifts)

| Metric | Value |
|---|---|
| Mean bias (formula − measured) | +0.058 USD |
| Std dev | 0.065 |
| 95% CI | [+0.016, +0.101] |
| Max \|bias\| | 0.180 USD |
| Bias as % of avg measured saving | ~15% |

---

## Root cause of the positive bias

When a child runs on haiku instead of sonnet, the token count for that child is
**higher** (haiku tasks spawn more turns, more cache reads). The formula prices
all those extra tokens at sonnet baseline, making `C_baseline` larger than the
true counterfactual. In other words: the formula assumes the same token count
whether or not a downshift happened, which overstates the baseline when a cheaper
model generates more output.

This is a **known limitation** of the current formula definition (D4: inferred
baselines included and labelled). It is not a bug; it is the stated assumption.

---

## Assumptions that must accompany any published `Saved` figure (README 1.1 rule 6)

1. `C_baseline` assumes the same token usage as observed — a cheaper model may
   generate more tokens, making the true saving smaller.
2. The formula measures routing saving only; Compactor saving is accounted
   separately (bytes / 4 tokens, additive).
3. Ground truth `C_actual` comes from Claude Code's own `costUSD`; other harnesses
   use the formula's own repricing (no external ground truth available).

---

## Verdict

**GO with caveat** for `net_counterfactual_cost_v1`. The formula has a positive
systematic bias of ~+15% on runs where downshifts occurred. This is within the
stated D4 assumption (inferred baseline). The bias must be:

- Printed next to every `Saved` figure as `(estimate, may overstate by ~15% when
  downshifts occur)` or equivalent.
- Documented in `docs/design/accounting.md` (T0) as the primary known limitation.
- Re-evaluated in part 2 of the battery with more downshift-heavy tasks.

The formula is **not** a reason to stop wave 2. D2 and D4 remain as written.

---

## Reproducibility

Script location: `downshift-labs` (private repo), path TBD after labs PR.
Input: `~/.downshift-eval/runs/four-arm-part1-2026-10-21/**/result.json` (46 files).
Catalog: `internal/catalog/catalog.json` at commit `97ee5479` (`prices_as_of: 2026-10-19`).
Run: `python3 spike_s0_formula.py <battery_dir>`.
