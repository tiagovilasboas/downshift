# Published classifier & outcome metrics

Maintainer-curated **final numbers** for the public Downshift repo. Raw
prompts, splits, and executable outcome tasks live in **downshift-labs**
(private). Re-run benchmarks there after router changes; update this file when
you intentionally publish new figures.

**Last updated:** 2026-10-10 (classifier on `main`, outcome runs 2026-10-03; adapt memory key fix recomputed same day).

These figures describe the **maintainer 40-task Go suite** and the **in-tree
classifier**. They are **not** proof of generalization on unseen traffic and
**not** a billing or invoice comparison.

## Classifier (human complexity rubric, not measured task outcomes)

| Dataset | Tasks | Tier accuracy (95% CI) | FRONTIER→MID | FRONTIER→SMALL | SMALL→MID | SMALL→FRONTIER |
|---|---|---|---|---|---|---|
| Seed regression net | 200 | 69.0% (62.5–75.5%) | 0.0% (0/50) | 0.0% (0/50) | 49.0% (49/100) | 7.0% (7/100) |
| Burned holdout (regression net only) | 300 | 100.0% (100.0–100.0%) | 0.0% (0/75) | 0.0% (0/75) | 0.0% (0/150) | 0.0% (0/150) |

Eval-only splits (fresh, heldout2, blind-vitrine) and generalization studies are
**not** published here. See [EVAL-PRIVATE.md](EVAL-PRIVATE.md).

## Outcome eval — model capacity (recorded runs, 2026-10-03)

40 tasks with stub/reference checks. Each tier was run once with a fixed model.
This table is **model capability**, not router or adapt memory.

| Run tier | Model | Pass (all tasks) | Pass (TRIVIAL+SIMPLE) |
|---|---|---|---|
| frontier | `anthropic/claude-opus-5.5` | 40/40 = 100.0% | 20/20 = 100.0% |
| mid | `deepseek/deepseek-v4-flash` | 31/40 = 77.5% | 17/20 = 85.0% |
| small | `qwen/qwen3-coder-30b-a3b-instruct` | 27/40 = 67.5% | 16/20 = 80.0% |

Classifier routing on that suite, recomputed 2026-10-10 with `ClassifyWithSemantic`
on this `main` and no adapt memory: 6 → small, 14 → mid, 20 → frontier, and
38/40 tasks passed at the tier the classifier picked. The 2026-10-06 line
(0 small, 29 mid, 11 frontier) described the classifier before the algorithmic
signals landed. Small minus frontier on TRIVIAL+SIMPLE labels remains −20.0pp
on the fixed-model runs above, not on the router.

## Adapt tier memory — offline efficacy (same runs, recomputed 2026-10-10)

Recompute with:

```text
downshift eval-adapt \
  --tasks=<downshift-labs>/eval/outcomes/testdata \
  --runs=<downshift-labs>/eval/outcomes/runs
```

Policy: `AdjustTier` applies per-shape hit/miss memory after
`ClassifyWithSemantic`; missing `adapt-memory.json` is a no-op.

### A — Contrafactual per task (upper bound, not production memory)

For each task, hits/misses come **only** from that task’s small/mid/frontier
runs; then `AdjustTier` is applied to the classifier baseline. This measures
what perfect **per-task** memory could achieve, not what a shared on-disk file
does.

| Metric | Value |
|---|---|
| Tasks | 40 |
| Pass @ classifier tier (before) | 38/40 = **95.0%** |
| Pass @ adapt-adjusted tier (after) | 40/40 = **100.0%** |
| Tier moves | 25 down · 2 up · 13 same |

**What A proves:** with task-local outcomes, the policy can reach full pass on
this suite by downshifting when a cheaper tier already succeeded. It does **not**
prove production `adapt-memory.json` will do that.

### B — Shared shape memory (production-shaped aggregation)

Hits/misses are aggregated **across all tasks** that share the same memory key
(`<complexity class>:<dominant feature>`, e.g. `TRIVIAL:coding`), then
`AdjustTier` runs per task. Downshift requires a hit at the cheaper tier with
**no miss at that same tier** on that key. Repo id in the key is **not** in this
leva (no stable repo id on the routing hook without paths).

| Metric | Value |
|---|---|
| Tasks | 40 |
| Pass @ classifier tier (before) | 38/40 = **95.0%** |
| Pass @ adapt-adjusted tier (after) | 38/40 = **95.0%** |
| Tier moves | 15 down · 0 up · 25 same |
| vs baseline | **0.0pp** (matches classifier; gate floor ≥ 38/40 met) |

**What B proves:** with class-scoped keys, shared memory on this suite no longer
collapses below the classifier. It is not the per-task upper bound (A); further
scoping (repo, session) may still help cost without hurting pass rate here.

## How to cite

Link this file or quote the table with the **Last updated** date. Do not treat
holdout accuracy or suite A/B as proof of generalization (see maintainer
benchmark README in labs).
