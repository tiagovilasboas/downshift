<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Four-arm battery, part 1: mechanical delegated tasks (2026-10-21)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

What this page answers: on delegated mechanical work in Claude Code, how many tokens does each setup spend per
accepted task, and how much of the difference does Downshift's routing explain? It follows the
[benchmark protocol](../guides/benchmark-protocol.md) and extends the
[2026-10-15 A/B](subagent-routing-ab-2026-10-15.md). Aggregates only: no prompts, task text or paths. Raw runs stay
private. One operator, one machine, Claude Code headless.

## Method

- **Tasks.** The four mechanical tasks of the 2026-10-15 A/B (rename, doc comments, named constants, mirrored
  tests), five independent pieces each, one subagent per piece (at least 3 spawns per run). Each task has nine hidden
  checks that fail at the base commit and pass with a reference solution.
- **Arms.** A: Opus fixed, routing off. B: native `opusplan`, routing off. C: `opusplan` with Downshift routing.
  D: C plus the Compactor. Routing off means hooks wired with `DOWNSHIFT_NO_ROUTE=1`; the Compactor is off in A to C
  (no Bash hook, `DOWNSHIFT_EXEC=0`). Every run has its own state directory. Runs shuffled, three in parallel.
- **Metric.** Total tokens (input, output, cache read, cache write, all models) per **accepted** run; failed runs are
  charged to their arm, as `downshift report` charges redone work. Also reported per run, before that charge.
  Intervals: 95% percentile bootstrap over runs.
- **Judge.** Exit code of the hidden checks. Nothing else.
- **Pre-registration.** Tasks, checks, arms and caps were frozen before the first run. Two changes were recorded
  before any result was seen: the binary moved to public main `13f1c91d` (it wraps `cd <dir> && X` and
  `X | tail -N`, which the earlier frozen binary did not), and the cap became tokens instead of dollars. The battery
  was stopped by the owner at 46 of 48 runs.

## Results

| Arm | Accepted | Tokens per accepted run | Downshifts applied |
|---|--:|--:|--:|
| A Opus fixed | 10 of 11 | 2.25M | 0 |
| B opusplan | 10 of 11 | 2.00M | 0 |
| C opusplan + routing | 12 of 12 | 1.52M | 23 of 54 spawns |
| D routing + Compactor | 12 of 12 | 1.46M | 15 of 54 spawns |

| Comparison | Per accepted run | 95% interval | Per run | 95% interval |
|---|--:|---|--:|---|
| Routing, B − C | 24.1% fewer | +2.1% to +46.7% | 16.5% fewer | +0.6% to +29.7% |
| Routing + Compactor, B − D | 26.8% fewer | +4.4% to +48.3% | 19.4% fewer | +3.2% to +32.4% |
| Compactor, C − D | 3.5% fewer | −14.8% to +19.4% | 3.5% fewer | −14.8% to +19.4% |
| Opus fixed vs everything on, A − D | 34.8% fewer | +19.1% to +52.5% | 28.3% fewer | +17.7% to +37.8% |
| opusplan alone, A − B | 11.0% fewer | −28.9% to +39.0% | n/a | n/a |

Per task, tokens per accepted run (accepted / runs):

| Task | A | B | C | D |
|---|--:|--:|--:|--:|
| named constants | 1.95M (3/3) | 3.43M (2/3) | 1.73M (3/3) | 1.70M (3/3) |
| doc comments | 1.81M (2/2) | 1.40M (2/2) | 1.13M (3/3) | 1.04M (3/3) |
| rename | 3.53M (2/3) | 1.97M (3/3) | 1.85M (3/3) | 1.83M (3/3) |
| mirrored tests | 1.98M (3/3) | 1.48M (3/3) | 1.35M (3/3) | 1.28M (3/3) |

## What this supports

- On mechanical delegated work, routing lowers the tokens spent per run by about 16% and per accepted task by about
  24% against native `opusplan`, with both intervals above zero, and acceptance did not drop (12 of 12 in both routed
  arms). Part of the per-accepted gap is one failed `opusplan` run on the named-constants task, charged to its arm.
- Cache tokens dominate every arm (above 98% of the total); the saving is mostly cache that smaller models re-read.

## What it does not support

- **The Compactor.** It is not measurable here: in arm D the agents ran their checks as `a; go test …`, a chain the
  Compactor did not wrap until [#424](https://github.com/tiagovilasboas/downshift/pull/424), merged the same day.
  Only one shell command went through it. Part 2 uses a binary with #424.
- **Everyday use.** These are forced-delegation mechanical tasks: the ceiling case for routing, not a typical session.
- **Other harnesses, invoices, plan limits.** Claude Code only; token counts, not money; no plan-limit measurement.
- **Quality beyond the checks.** The checks verify structure and tests, not wording or design.

## Next

Part 2: the four natural fixes of the A/B (they run `go test` and `go build`), same four arms, binary with #424,
capped in total tokens.
