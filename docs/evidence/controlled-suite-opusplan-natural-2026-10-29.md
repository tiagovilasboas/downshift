<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Controlled suite: routed versus control on Claude Code (2026-10-29)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Level: **measured once: one harness, one batch, four tasks, six repetitions per cell.** On this suite, routing kept
acceptance within the pre-registered limit and reduced tokens in matched runs. It says nothing about other harnesses,
other tasks, or other days.

## What was run

- Batch `opusplan-natural-20261029`: 48 runs, which is 4 tasks × 2 arms × 6 repetitions, three runs in parallel.
- Control arm: each spawn is classified and left unchanged. Routed arm: Downshift chooses the model for each spawn
  under the project policy.
- Tasks: `setup-named-group`, `explore-plan-baseline`, `antigravity-task-text`, `agent-type-telemetry`.
- Budget: 190M tokens of work plus cache, as authorised. The batch used 74.2M. Accounting is in tokens only; no cost
  figure is used anywhere in this document.
- Criterion, fixed before the batch in the labs harness (`tools/harness-runs/parity.py`): PARITY passes when acceptance
  drops by no more than 10 points and the lower bound of the 95% paired-savings interval is above zero.

## What happened

| Task | Control accepted | Routed accepted | Control tokens | Routed tokens |
|---|---|---|---|---|
| `agent-type-telemetry` | 6/6 | 6/6 | 19.35M | 13.46M |
| `antigravity-task-text` | 6/6 | 6/6 | 4.58M | 3.35M |
| `setup-named-group` | 6/6 | 6/6 | 7.97M | 6.85M |
| `explore-plan-baseline` | 1/6 | 2/6 | 11.06M | 7.62M |
| **Total** | **19/24** | **20/24** | **42.95M** | **31.28M** |

Tokens are work plus cache per arm.

| Measure | Result |
|---|---|
| Acceptance, control | 19/24 (79%), 95% Wilson interval 59.5% to 90.8% |
| Acceptance, routed | 20/24 (83%), 95% Wilson interval 64.1% to 93.3% |
| Acceptance drop (control minus routed) | −4.2 points; the limit is 10 |
| Paired savings | mean 22.2% over the 19 runs where both arms were accepted; 95% bootstrap interval 9.8% to 32.3% (10,000 resamples, seed 20261029) |
| Pooled tokens (descriptive) | 42.95M to 31.28M, 27.2% lower |
| Verdict | **PASS**: both pre-registered conditions hold |

The compactor measured nothing in this batch (0 bytes raw, 0 bytes saved), so compaction is not part of these numbers.

## What it does and does not show

- It shows that on this suite routing did not lower acceptance beyond the limit, and that the matched runs used fewer
  tokens with a savings interval above zero.
- It does not show a difference in acceptance. Six repetitions per cell leave wide intervals that overlap heavily, so
  "parity" here means no detectable drop beyond the limit, not equal quality.
- `explore-plan-baseline` was accepted only 1 or 2 times out of 6 in both arms. Its numbers reflect the task and the
  acceptance check, not a routing effect, and they are not part of the savings claim beyond the paired measure above.
- Raw run logs stay in the labs workspace. This file is the summary of the batch that the labs parity criterion
  judged.
