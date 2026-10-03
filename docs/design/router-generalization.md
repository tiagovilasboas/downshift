# Router generalization: evidence and next cut

Status: design + evidence. This document does **not** change production routing.

## Question

The production classifier (`internal/core/classifier.go` + semantic boost) is
100% on `holdout.json` but drops sharply on prompts it was not tuned on. Is
that a vocabulary gap that more keywords would close, or a structural limit
of a closed keyword classifier, and what is a safe next step?

## Data

| Split | n | Role | Author / provenance |
|---|---|---|---|
| `benchmark/tasks.json` (seed) | 200 | tuning | original seed set; the keyword vocabulary was tuned here |
| `benchmark/holdout.json` | 300 | tuning-era holdout | same era as seed |
| `benchmark/fresh.json` | 100 | eval-only, read-only | guarded by `fresh_guard_test.go` |
| `benchmark/heldout2.json` | 120 (30 per label) | **new** eval-only blind split | written for this study before any classifier was run on it; frozen 2026-10-03 05:46 BRT, sha256 `03425eef95f1c88cc56129830dabdaaafe7e5052636e2cbdb233b1b44bd96a5f`; max Jaccard to any prompt in the other splits 0.471 (16 near-duplicates replaced before freezing) |
| outcome tasks (`benchmark/outcomes/tasks`) | 40 | outcome eval | executable tasks, labelled by intended tier |

`heldout2.json` was measured **once** with the configuration below. It is now
an eval-only split under the same guard as `fresh.json`: it must not be read by
tuning code, and it must not change in the same diff as the classifier.

## Method

- **regex**: production `ClassifyWithSemantic`, unchanged.
- **NB**: `tools/baseline/nb_tier.py`, multinomial naive Bayes over unigrams +
  bigrams, alpha = 1, MEDIUM on ties. Settings fixed before evaluation; trained
  on `tasks.json` only. Stdlib Python, offline, no network.
- **regex upshifted by NB**: `max(regex tier, NB tier)`. NB may only move a
  prompt *up*, never down, so it cannot create a new under-route on its own.

Tier accuracy maps TRIVIAL/SIMPLE→small, MEDIUM→mid, COMPLEX→frontier.
F→M / F→S = frontier prompts routed to mid / small (the costly errors);
S→M = small prompts routed to mid (wasted spend).

## Results

| Split | regex (production) | NB alone | regex upshifted by NB |
|---|---|---|---|
| seed (NB trains here) | 69.0%, S→M 49/100 | 100% (training data) | 69.5% |
| holdout | **100%** | 91% | 91% (26 MEDIUM→COMPLEX, 1 TRIVIAL→COMPLEX over-routes) |
| heldout2 (measured once) | **40%**, F→M 14/30, S→M 45/60 | 80%, **F→S 5/30** | 47.5%, F→M 4/30, F→S 0/30 |
| fresh (read-only) | 50%, F→M 5/25, S→M 35/50 | 90%, F→M 1/25 | 54%, F→M 0/25 |
| outcome tasks | 22.5%, F→M 9/10 | 55%, **F→S 7/10** | 27.5%, F→M 6/10 |

## Findings

1. **Root cause is structural, not a missing keyword or two.** The classifier
   is a closed vocabulary that defaults to MEDIUM when no signal fires. Most
   small→mid errors on seed (49/100), heldout2 (45/60) and fresh (35/50) are
   that no-signal default. COMPLEX is perfect on seed/holdout because the
   vocabulary was tuned on that era's phrasing; on heldout2 almost half the
   frontier prompts fall to mid.
2. **NB alone is unsafe.** It is far more accurate on unseen phrasing (80–90%)
   but sends frontier prompts to small (5/30 on heldout2, 7/10 on outcome
   tasks). It must never be allowed to downshift.
3. **Upshift-only NB is a real safety gain with a cost.** It removes most
   frontier under-routes (heldout2 F→M 14→4, fresh 5→0) and never sends
   frontier to small, but costs 9pp on holdout through MEDIUM→COMPLEX
   over-routes (more frontier spend).
4. **Author bias.** I wrote `heldout2.json` and would also write any keyword
   fix, after having seen `fresh.json` failures. A keyword patch that lifts
   heldout2 would be evidence of nothing. That is why this PR stops here.
5. **The outcome "COMPLEX" tasks are algorithmic** (Dijkstra, sudoku, LCS
   diff, ...), arguably not rubric-COMPLEX (cross-cutting, high blast radius).
   Whether routing them to mid or small is acceptable is an outcome question,
   answered by the recorded pass rates per tier in the README, not by the
   label.

## Proposed next cut (separate PR, not this one)

1. Add a learned second opinion (NB or a small linear model over the same
   features) used **upshift-only**, with a margin threshold: upshift only when
   P(higher tier) − P(regex tier) exceeds a threshold tuned on seed + holdout
   alone, chosen to cap holdout over-routes (e.g. ≤ 3%).
2. Validate once on a **new, independently authored** blind split — anonymized
   real telemetry prompts, or a split generated from the rubric by a cheap ZDR
   model with no access to the classifier — never on heldout2/fresh again for
   tuning.
3. Re-derive tier targets from outcome data: if mid passes the algorithmic
   "COMPLEX" tasks at frontier rates, the rubric (not the router) should move
   them to MEDIUM.
4. Keep the risk floor and the guardrails as hard rules above any learned
   signal.

## Blind validation (2026-10-03)

The next cut above asked for validation on an independently authored split.
`benchmark/blind-vitrine.json` (60 prompts, 20 per tier, written by an agent
that never saw this repository; see `benchmark/blind-vitrine.README.md`)
is that split. The plan was frozen before any blind prediction was generated.

### Frozen plan

- Rule written 2026-10-03 07:08:24 BRT. Threshold frozen 07:08:38 BRT, before
  production or NB predictions existed for the blind set.
- **Production tier:** `core.Route(prompt, "claude-code", "").Tier`, with
  graphify off as in a default install. TRIVIAL/SIMPLE map to small, MEDIUM to
  mid, COMPLEX to frontier.
- **NB:** as above, trained on `tasks.json` only.
- **Upshift-only combination:** take NB's tier only when it is higher than
  production's **and** the margin is ≥ T. The margin is the per-feature
  log-likelihood gap between NB's best label in its tier and its best label in
  the production tier.
- **T is chosen on seed + holdout only** (500 prompts) with a fixed rule: the
  smallest T on a 0.02 grid whose combined accuracy is at most 2.0pp below
  production. Result: **T = 0.10** (86.0% vs 87.6%, 10 upshifts; T = 0.08
  drops 2.8pp). NB is in-sample on seed, so holdout carries most of the signal.
- **Pre-registered "clear win" criterion:** fewer FRONTIER→lower than
  production on blind, with tier accuracy no more than 5pp below production.

### Results

Tier accuracy with Wilson 95% CI. Confusion rows are gold small/mid/frontier;
columns are predicted small/mid/frontier.

| Split | Router | Tier accuracy (95% CI) | FRONTIER→lower | SMALL→higher | Confusion |
|---|---|---|---|---|---|
| **blind (n=60)** | production | 46.7% (34.6–59.1) | **5/20** (4 mid, 1 small) | 16/20 | [4 11 5] [5 9 6] [1 4 15] |
| | NB alone | 48.3% (36.2–60.7) | 11/20 (**10 small**) | 0/20 | [20 0 0] [19 0 1] [10 1 9] |
| | production + NB, upshift-only, T=0.10 | 50.0% (37.7–62.3) | **3/20** (2 mid, 1 small) | 16/20 | [4 11 5] [4 9 7] [1 2 17] |
| heldout2 (read-only) | production | 40.0% (31.7–48.9) | 14/30 | 55/60 | [5 45 10] [0 27 3] [0 14 16] |
| | upshift-only, T=0.10 | 48.3% (39.6–57.2) | 4/30 | 55/60 | [5 45 10] [0 27 3] [0 4 26] |
| fresh (read-only) | production | 50.0% (40.4–59.6) | 5/25 | 39/50 | [11 35 4] [0 19 6] [0 5 20] |
| | upshift-only, T=0.10 | 54.0% (44.3–63.4) | 0/25 | 39/50 | [11 35 4] [0 18 7] [0 0 25] |
| holdout | production | 100% | 0/75 | 0/150 | |
| | upshift-only, T=0.10 | 97.0% (94.4–98.4) | 0/75 | 0/150 | 9 MEDIUM→frontier |

### Reading

- **By the frozen criterion, upshift-only wins on blind:** FRONTIER→lower goes
  from 5 to 3, and accuracy rises 3.3pp instead of falling.
- **The effect is small.** Only two blind items change for the better (an
  asyncio shutdown/lost-writes bug and an at-least-once payment webhook design,
  both mid → frontier) and none get worse. That gives exact McNemar p = 0.5,
  far from significant at n = 60.
- **The direction holds on every unseen split** (heldout2 14→4, fresh 5→0,
  blind 5→3). On holdout it costs 3pp, as over-routes to frontier.
- **The margin also blocks some correct upshifts.** NB was right on two more
  blind frontier items that T = 0.10 blocked (Kubernetes rolling-deploy 502s,
  routed to small; vague "search is too slow"). This is a post-hoc
  observation, not a re-tune.
- **Upshift-only cannot fix the remaining under-routes.** It cannot help when
  NB is also low (an offline range-query optimization problem, routed to mid
  and NB small), and it never touches MID→small (5/20 on blind, including the
  flagged zero-downtime migration).
- **NB alone does not transfer.** It reaches 80–90% on short prompts but
  collapses on blind's long prompts with inline code: it calls 49/60 small and
  sends 10/20 frontier prompts to small. It must stay upshift-only.
- **The largest blind failure is waste, not risk:** 16/20 small tasks go to
  mid or frontier, because of the MEDIUM no-signal default and code-heavy
  prompts that trip COMPLEX keywords. Neither variant addresses it.
- **On the two flagged items:** the zero-downtime migration (gold mid) is
  routed to small by every variant. The deadlock analysis (gold frontier) is
  correctly routed to frontier by production, while NB said small.

### Recommendation (pending GO; no routing change yet)

1. Implement the upshift-only NB second opinion with T = 0.10 behind a config
   flag, **default OFF**, emitting the would-be upshift to telemetry (shadow
   mode) so real prompts measure its over-route cost before anyone turns it on.
2. Do not use NB to downshift or to replace the classifier.
3. Treat small-task over-routing (SMALL→higher 80% on blind) as the next
   problem. It needs outcome-informed tier targets and a learned model trained
   on longer, code-bearing prompts (not the 200 short seed prompts). Any new
   evaluation needs a fresh independently authored split, because blind-vitrine
   has now been looked at.

## Reproduce

```sh
# production predictions: one JSON array per split, [{"prompt", "label": tier}],
# from core.Route(prompt, "claude-code", "").Tier (a throwaway test, not committed)
python3 tools/baseline/nb_tier.py --train benchmark/tasks.json \
  --regex seed.prod.json holdout.prod.json \
  --tune benchmark/tasks.json benchmark/holdout.json        # -> chosen_margin 0.1
python3 tools/baseline/nb_tier.py --train benchmark/tasks.json \
  --regex *.prod.json --margin 0.10 --eval <eval-split>.json
```

The eval-only split paths are passed on the command line on purpose: tuning
code must not name them (see `scripts/fresh-guard.sh`).
