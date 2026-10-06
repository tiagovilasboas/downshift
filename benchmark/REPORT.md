# Published classifier & outcome metrics

Maintainer-curated **final numbers** for the public Downshift repo. Raw
prompts, splits, and executable outcome tasks live in **downshift-labs**
(private). Re-run benchmarks there after router changes; update this file when
you intentionally publish new figures.

**Last updated:** 2026-10-06 (beta.6 era, default in-tree classifier).

## Classifier (human complexity rubric, not measured task outcomes)

| Dataset | Tasks | Tier accuracy (95% CI) | FRONTIER→MID | FRONTIER→SMALL | SMALL→MID | SMALL→FRONTIER |
|---|---|---|---|---|---|---|
| Seed regression net | 200 | 69.0% (62.5–75.5%) | 0.0% (0/50) | 0.0% (0/50) | 49.0% (49/100) | 7.0% (7/100) |
| Burned holdout (regression net only) | 300 | 100.0% (100.0–100.0%) | 0.0% (0/75) | 0.0% (0/75) | 0.0% (0/150) | 0.0% (0/150) |

Eval-only splits (fresh, heldout2, blind-vitrine) and generalization studies are
**not** published here. See [EVAL-PRIVATE.md](EVAL-PRIVATE.md).

## Outcome eval (executable Go tasks)

40 tasks with stub/reference checks (maintainer suite). Recorded runs on
2026-10-03:

| Run tier | Model | Pass (all tasks) | Pass (TRIVIAL+SIMPLE) |
|---|---|---|---|
| frontier | `anthropic/claude-opus-5.5` | 40/40 = 100.0% | 20/20 = 100.0% |
| mid | `deepseek/deepseek-v4-flash` | 31/40 = 77.5% | 17/20 = 85.0% |
| small | `qwen/qwen3-coder-30b-a3b-instruct` | 27/40 = 67.5% | 16/20 = 80.0% |

Router on that suite: 0 → small, 29 → mid, 11 → frontier; 9/10 COMPLEX-labelled
tasks routed below frontier. Small minus frontier on TRIVIAL+SIMPLE labels: −20.0pp.

## How to cite

Link this file or quote the table with the **Last updated** date. Do not treat
holdout accuracy as proof of generalization (see maintainer benchmark README in
labs).
