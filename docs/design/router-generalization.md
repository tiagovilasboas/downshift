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

## Reproduce

```sh
# regex predictions: one TSV per split, prompt<TAB>LABEL from ClassifyWithSemantic
python3 tools/baseline/nb_tier.py --train benchmark/tasks.json \
  --eval benchmark/tasks.json benchmark/holdout.json <eval-split>.json \
  --regex seed.rx.tsv holdout.rx.tsv <eval-split>.rx.tsv
```

The eval-only split paths are passed on the command line on purpose: tuning
code must not name them (see `scripts/fresh-guard.sh`).
