<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Benchmark protocol: routing, output cutting and RTK

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Routing alone saved about 12% on mechanical delegated work and an estimated 3% in natural use
([evidence.md](evidence.md), rungs D and B). Output cutting, which shrinks tool output before it reaches the model,
acts on the main session as well. This protocol answers one question before more is built: **does routing still earn
its place once output cutting is in the stack, and how does the stack compare with RTK alone?** It extends
[benchmark-protocol.md](benchmark-protocol.md) (tasks, per-run record, privacy, limits) and does not repeat it.

Status: **protocol only. Nothing here has been run.** Every number below is a design parameter or a pre-registered
threshold, not a result.

## Arms

All arms: same harness version, same plan, same repository state, same prompts, same session model, one
`DOWNSHIFT_STATE_DIR` per arm, run order shuffled across arms.

| Arm | Routing | Output cutting | How to set it |
|---|---|---|---|
| 0 (control) | off (`DOWNSHIFT_NO_ROUTE=1`, hooks still record) | none | as arm B of the protocol it extends |
| a | Downshift routing only | none | `DOWNSHIFT_CONTROL_SHARE=0` |
| b | routing | Headroom companion in the request path | arm a plus `headroom wrap` for the harness (Headroom's own command, see [companions.md](companions.md)) |
| c | routing | `downshift exec` wrapper (opt-in) | arm a plus the wrapper as documented once released |
| d | off | RTK alone, if installed | RTK's own hook setup; `DOWNSHIFT_NO_ROUTE=1` |
| e (optional, isolates routing) | off | `downshift exec` | arm c with `DOWNSHIFT_NO_ROUTE=1` |

Arm d is dropped, and the result says so, if RTK is not available. Arm e is what separates the marginal value of
routing on top of exec (c against e); without it, c against d is the only stack comparison. Confirm each arm's
configuration from the log (`requested_model`, companion record from `downshift doctor`), not from settings files. A
companion's saving is never credited to routing.

## Tasks

Reuse the frozen task set and categories of [benchmark-protocol.md](benchmark-protocol.md#tasks). Output cutting acts
on tool-heavy work, so the set must include tasks with large test, build or log output. Categories: mechanical, medium,
delegation-heavy, and one added here, **output-heavy** (a bounded fix whose loop prints long test or build output).
Complex and security-sensitive tasks stay in the set as safety checks (acceptance only), not as a saving claim. Each
task keeps its hidden acceptance check, validated on base and fix.

## Metrics

| Metric | Source | Status |
|---|---|---|
| **Cost per accepted task** (primary): cost of every run of the task divided by accepted runs, so a failed run is charged to the task | Claude Code `total_cost_usd` per run (list price) | measured per run by the harness; dollars are list price, not an invoice |
| **Tokens per accepted task, split work and cache**: input+output apart from cache read+write | whole-session usage from the harness; child tokens from usage events | child tokens observed by Downshift; whole-session tokens are harness output, read by hand |
| **Quota-window consumption on a subscription plan**: share of the 5-hour and 7-day windows used by the arm's runs | the harness's own usage display before and after a block of runs, same account, no other use | manual and coarse; the only metric that speaks for plans with no per-token bill |
| **Acceptance** | exit code of the hidden check, then the repository suite | manual |
| Retries, failures, rework | as in the protocol it extends | inferred plus manual |
| Latency | wall clock | manual; not a cost claim |

Work and cache are reported apart because cache reads carry the smallest price gap between models and are most of a
session's tokens; one token total would credit routing with a saving it cannot make on that class. Output cutting
should show mostly in work and fresh-input tokens and in how cache grows over the session; report both.

Quota windows are measured in blocks: run one arm's block back to back, read the windows before and after, let the
window reset or use a second identical account, and alternate block order across repeats. The harness rounds window
readings; a difference smaller than one display step counts as no gain.

## Sample size

- Floor: 12 saving-claim tasks (4 mechanical or output-heavy, 4 medium, 4 delegation-heavy) plus 4 safety tasks, 5 runs
  per task per arm, which is 60 saving-claim runs per arm. The earlier A/B used 3 runs per cell and gave a direction
  only.
- A per-category claim needs all 4 tasks of the category at 5 runs (20 runs); fewer is reported as anecdote.
- Intervals: percentile bootstrap, 2,000 resamples, fixed seed, resampling tasks first and runs within a task second.
  The interval is stated whether or not it includes zero.
- Runs that hit the usage limit are rerun, not counted as failures. Runs with a protocol deviation are listed and
  analysed with and without.
- Before the full set, a 2-task, 3-run pilot per arm checks that the arms differ as configured and that the quota
  readings are usable. Pilot runs are not part of the result.

## Pre-registered cut criterion

Fixed before any run, on the saving-claim tasks only, primary metric cost per accepted task:

1. **Acceptance gate.** An arm whose acceptance is more than 5 points below arm 0 or below arm d is disqualified from
   any saving claim.
2. **Routing becomes an optional feature, and stops being the headline,** if both hold:
   - arm c lowers cost per accepted task by less than 10% relative to arm d (RTK alone), judged on the point estimate;
   - the quota-window readings show no gain for c over d (difference within one display step on both the 5-hour and
     7-day windows).
3. If arm d was not run, the same test is applied to arm c against arm e (exec alone): routing adding less than 10% on
   top of output cutting, with no quota gain, makes routing optional.
4. Routing stays a headline feature only if the 95% interval of its marginal saving excludes zero and the point
   estimate is at least 10%.
5. Anything between is reported as inconclusive and the sample is doubled once, not re-cut.

"Optional" means: off by default in `setup`, documented as an advanced lever, no longer described as the main way
Downshift saves tokens. It does not mean removal; the guardrails and evidence tooling keep their value. The threshold
is a decision rule, not a prediction.

## What is measured and what is estimated

Rung per [evidence.md](evidence.md): once run, this protocol produces **rung D evidence** (real tasks, arms against
control, acceptance by hidden tests, one operator) and, if the sample passes, a first input to rung E. It cannot reach
E by itself: `downshift report` defines E on its own rule and needs 20 priced completions per group.

| Figure | Class |
|---|---|
| Cost per run from `total_cost_usd` | measured by the harness, at list price |
| Child tokens and child cost from usage events | measured tokens, catalog-priced dollars (rung B) |
| Whole-session tokens and parent coordination | measured by the harness, read by hand; the hook does not see the parent |
| Quota-window consumption | measured, coarse, manual |
| Any ceiling or composition figure in [evidence.md](evidence.md#where-the-saving-can-come-from) | hypothetical until this runs |
| RTK and Headroom figures from their own pages | their claims, not ours; here they are only arms to be measured |

## Limits

Everything in the [limits of the protocol it extends](benchmark-protocol.md#limits-to-state-in-the-result) applies:
list prices not invoices, one operator, direction not proof at small n, structural acceptance checks, Claude Code only.
Added here: quota windows are coarse and depend on account state; output cutting results depend on how much tool
output the task set prints, so the answer is for this task mix and not for natural use; a tool that rewrites output
can change what the agent sees, so acceptance is the guard and any acceptance drop is a finding in itself.

## Output

One page in `docs/evidence/benchmark-output-cutting-<start date>.md`: the arm table as run, task counts per category,
per-arm and per-category results with intervals, the cut-criterion test with its verdict, the quota readings, and the
list of what stayed unmeasured. Aggregates only; no prompts, paths or `events.jsonl`.
