<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Subagent routing, routed versus control: first measurements (2026-10-15)

What this page answers: does Downshift lower the cost of a coding task without lowering the share of tasks that pass? It is
the first controlled measurement in this repository. It is small, from one operator, and it says where it cannot be trusted.
Aggregates only: no prompts, task text or paths are published here. Raw runs stay private.

## Method

- **Tasks.** Real fixes from this repository's own history (`explore-plan-baseline`, `antigravity-task-text`,
  `setup-named-group`, `agent-type-telemetry`), plus four synthetic mechanical tasks of five independent pieces each (rename,
  doc comments, named constants, mirrored tests). Each real task is cloned at the commit before the fix with no later history and no
  remote; the fix's test files are hidden from the agent and applied afterwards. A task counts only if its checks fail on the base
  and pass with the real fix (checked for all eight).
- **Arms.** `routed`: Downshift hooks active. `control`: the same hooks with `DOWNSHIFT_NO_ROUTE=1`, so each spawn runs on the
  model the harness asked for. Same prompt, same model (Claude Code, sonnet-5-5), runs shuffled so neither arm gets the quiet hours.
- **Judge.** Acceptance is the exit code of the hidden tests, or of the validated checks for the mechanical tasks. Nothing else.
- **Cost.** Claude Code's own `total_cost_usd` per run (list price). Codex reports tokens only; its figures are list-price estimates.
- **Subagents.** Downshift acts only on subagent spawns, so a run that spawns none is identical in both arms. Each run records its
  spawns, and a routed-versus-control comparison counts only runs with at least 3.
- **Runs that never ran.** A run that hits the account's usage limit has no cost and no work; it is excluded, not counted as a failure.

## Results

| Batch | What | n per arm | Accepted routed / control | Avg cost routed / control | Downshifts applied |
|---|---|--:|---|---|--:|
| b2 | the four real tasks, delegation optional | 8 | 7/8 / 7/8 | $0.65 / $0.77 | **0** (1 upshift to Opus) |
| b5 | the four mechanical tasks, one subagent per piece, at least 3 | 12 | 12/12 / 12/12 | $0.59 / $0.67 | 19 of 56 spawns (34%) |
| b6 | the four real tasks, at least 3 subagents, natural delegation | 12 | 8/12 / 9/12 | $0.75 / $0.92, not attributable to routing | 3 of 34 (9%) |

Codex (default account model, no Downshift), 8 runs on the real tasks: 8/8 accepted, about $0.21 per run at list price.

**b2.** The routed arm was cheaper, but Downshift applied no downshift, so the gap is not routing: the routed runs took about half the turns
(13.9 against 30.6). Of 10 spawn decisions, 6 kept the model for lack of signal, 2 downshift verdicts were held by the guardrail, 1 upshift to
Opus was applied (a risk category in the task text). Exploratory prompts written by the parent agent are long and the classifier is unconfident on them.

**b5, the ceiling.** Where the parent hands small mechanical pieces to subagents, 34% of spawns went to Haiku (39.8k output tokens, against 117.5k all on
Sonnet in the control arm), 64% kept their model, 1 was raised. Cost fell 12% overall and acceptance stayed at 12/12 in both arms.

| Task | Routed | Control | Note |
|---|--:|--:|---|
| `mech-rename` | $0.49 | $0.74 | all five subagents downshifted in all three runs: -33% |
| `mech-constant` | $0.66 | $0.77 | -14% |
| `mech-doc-comments` | $0.59 | $0.65 | -10%, the interval includes zero |
| `mech-test-mirror` | $0.62 | $0.52 | in 2 of 3 runs nothing was downshifted, so routed equals control there: the gap is noise |

**b6.** The routed arm cost less ($0.75 against $0.92, same four tasks) but only 3 of 34 spawn decisions were rewritten down, in 3 of the 12 routed runs: 28 kept the model,
and 2 upshift verdicts and 1 downshift verdict were held. The gap is therefore mostly the agent's own variation, as in b2, not routing. Of the 3 runs where a downshift was applied, 2
passed; the failing one is on the task that also fails in the control arm in 2 of 3 runs. The extra routed failure (`setup-named-group`, 2 of 3 against 3 of 3) had no downshift applied.
Spawns: 3 or more in 22 of 24 runs. Baseline: 0 unknown in 34 decisions, and in 6 of them it came from the configured model (`BASELINE_FROM_SETTINGS`), the case that left spawns blind before #126.
Wall time has outliers of 30 to 47 minutes (waits on the account's usage limit), so a mean time is not reported; the median is 141 s.

**Real usage log** (one operator, Claude Code, 127 decisions since 2026-10-12, before the fixes below): 9% rewritten down, 9% rewritten up, 22% held
by a guardrail, 18% with no known baseline. The estimated net saving of `downshift stats` is the difference between two large numbers.

## What this supports

- Downshift routes, and where it rewrites down the task still passes: in the ceiling case, 12 of 12 against 12 of 12.
- It can save money on mechanical delegated work: up to about 12% overall, about a third on pure renames, **list price, one operator, three runs per cell**.
- It does **not** show a saving on natural use: b2 applied no downshift and b6 applied 3 of 34, and the cost gaps there are inside the noise. A realistic figure lies between that and the ceiling and is not measured.

## What it does not support

- Any saving percentage for general use. The 19.3% in `downshift stats` is an estimate per decision at list price, not a measurement.
- "Same quality". The checks verify structure (names, comments exist, tests pass), not the quality of wording or design.
- Intervals. Three runs per cell is too few for more than a direction; the bootstrap intervals printed by the runner are indicative.
- Other harnesses. Cursor was not run; Codex ran without Downshift (no rewrite channel wired in the isolated home).

## Defects the runs exposed (all fixed or documented)

| Defect | Effect | Fix |
|---|---|---|
| `Explore` and `Plan` had no inferred baseline | their spawns were held and never routed | #117 |
| The task text of a multi-subagent Antigravity call came from the first child, the decision from the last | a record paired one child's task with another's tier | #119 |
| The transcript is written asynchronously, so the first spawn of a session often saw no model | 2 of 3 fresh runs had an unknown baseline and nothing was routed | bounded re-read, beta.12 |
| Parallel Agent calls in the first assistant message have no baseline (the message is written when it finishes streaming) | 4 of 5 spawns per run held | fallback to the configured model, marked `BASELINE_FROM_SETTINGS`, #126; limits in [harness-matrix.md](../reference/harness-matrix.md) |

Two defects were in the measuring rig and are recorded because they would have produced false results: a run flag that removed the transcript
(every baseline unknown, so the arms were identical), and counting usage-limit refusals as failed tasks.

## Reproducing

The runner (clone at the base commit, hidden tests, arms, aggregation with Wilson and bootstrap intervals) is in the private downshift-labs repository. The
protocol for a team is in [pilot.md](../guides/pilot.md); `downshift report` computes routed versus control from real usage.
