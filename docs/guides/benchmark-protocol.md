<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Benchmark protocol: Opus fixed, native opusplan, opusplan with Downshift

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

A reproducible protocol for the question the [evidence ladder](evidence.md) leaves open: does Downshift save money
beyond what Claude Code's native `opusplan` already saves, in which task categories, at what quality and failure
cost, and what stays unmeasured. It reuses the control group, `DOWNSHIFT_NO_ROUTE`, the [pilot](pilot.md), the
outcome suite (`internal/outcome`, [benchmark/report.md](../../benchmark/report.md)) and the method of the
[2026-10-15 A/B](../evidence/subagent-routing-ab-2026-10-15.md). Where a step has no command, it says "manual" or
"not available yet". Nothing here invents tooling.

## Arms

| Arm | Session model | Downshift | How to set it |
|---|---|---|---|
| A | Opus fixed (`claude-opus-5-5` for the session and every subagent that inherits it) | hooks wired, `DOWNSHIFT_NO_ROUTE=1` | `"model"` pinned in the run's `.claude/settings.json`, or `/model opus` |
| B | native `opusplan` (Opus plans, Sonnet executes) | hooks wired, `DOWNSHIFT_NO_ROUTE=1` | `downshift setup` default session policy, or `/model opusplan` |
| C | `opusplan` | hooks active, `DOWNSHIFT_CONTROL_SHARE=0` | same settings as B without `DOWNSHIFT_NO_ROUTE` |

Hooks stay wired in every arm so that decisions, usage and child ends are recorded identically; under
`DOWNSHIFT_NO_ROUTE=1` the runner classifies and logs but never rewrites (`outcome: baseline`, no `CONTROL` code).
`DOWNSHIFT_CONTROL_SHARE=0` in C routes every spawn, so the arm comparison is between whole arms rather than inside
one log; keep the default 0.10 instead if you also want `downshift report` to speak for C alone. Each arm records
into its own `DOWNSHIFT_STATE_DIR` ([config.md](config.md)), so logs do not mix. A pinned `--model` or `/model` is
not visible to the first parallel spawns of a headless run; `BASELINE_FROM_SETTINGS` marks the fallback
([harness-matrix.md](../reference/harness-matrix.md)). Confirm what each arm asked for from `requested_model` in the
log, not from the settings file.

Same harness version, same plan, same repository state, same prompts, same `.downshift/policy.json` (or none), same
`autoCompactWindow`, same session-start mode. Shuffle run order across arms (quiet hours bias cost and latency).

## Tasks

Build the task set once, freeze it, and keep it private if it contains repository text (the A/B runner and its
tasks live in downshift-labs; the two public samples are `internal/outcome/testdata/t01-clamp` and `t02-reverse`).
Each task is a base commit, a prompt, and a hidden acceptance check that fails on the base and passes with a known
fix; validate that pair before the first run, as the A/B did for all eight of its tasks.

| Category | What it holds | Expected behaviour | Minimum |
|---|---|---|---:|
| Mechanical | rename, doc comments, named constants, mirrored tests; independent pieces | the ceiling for downshifts (b5: 34% of spawns) | 4 tasks |
| Medium | a bounded fix with one or two files and existing tests | mid tier, mostly held | 4 |
| Complex | a design change across packages, hidden integration test | frontier, upshift or held | 4 |
| Security-sensitive | secrets, auth, payments, destructive SQL wording | risk floor or abstention; never small | 4 |
| Delegation-heavy | the prompt requires 3 or more subagents | where routing can act at all | 4 |
| Delegation does not pay | a one-file task that a parent might still delegate | expect no saving; measures coordination overhead | 4 |

Label each task once (`TRIVIAL`, `SIMPLE`, `MEDIUM`, `COMPLEX`) with `downshift try "<prompt>"` for the record, then
do not tune the classifier against the set. Three runs per task per arm is the floor the A/B used and is enough for a
direction only; intervals need more. Runs that hit the account usage limit are rerun, not counted as failures.

## Per-run record

Store one row per run, aggregates only, no prompt or task text (the telemetry policy in
[stats-export.md](../reference/stats-export.md) applies to the sheet as well). Task ids are opaque labels.

| Field | How to get it | Status |
|---|---|---|
| Task id, category, label, arm, run index | from the task manifest | manual |
| Requested and executed model per spawn | `downshift session --tree --json` for the run's session (`CLAUDE_CODE_SESSION_ID`); `requested_model` / `final_model` on the decision; `resolved` record when present | available |
| Functional result | exit code of the hidden check applied after the run | manual |
| Test result | the repository's own suite after the run (`go test ./...` or equivalent) | manual |
| Cost per run | Claude Code's own `total_cost_usd` for the run (list price, as in the A/B) | manual, harness output |
| Cost per subagent, tokens | usage events via `SubagentStop`: `actual_cost_usd`, `input_tokens`, `output_tokens`, `cached_tokens`, `cache_write_tokens` | available on Claude Code |
| Latency | wall clock around the run, and per spawn from decision and usage timestamps | manual; no duration field in telemetry |
| Retries | "Respawned after a failure" in `downshift stats`, `↻` rows in the tree; plus a manual count of re-delegations seen in the run | inferred + manual |
| Failures | `child_status` `error` / `interrupted`; `PostToolUseFailure` payload is uncaptured, so also count by hand | available, undercounted |
| Routing decisions and guardrails | `downshift session --why`, `reason_codes` and `corrections` in `stats --export` | available |
| Protocol deviations | model switched mid-run, usage limit hit, manual intervention, hook error on stderr | manual |

Export at the end of each arm: `downshift stats --days=N --export` and `downshift report --days=N --json` from that
arm's state dir. Keep the `events.jsonl` files private; publish the exports.

## Analysis

1. **Did the arms differ as configured?** Share of spawns whose `requested_model` was Opus in A, Sonnet in B and C;
   in C, downshifts applied, upshifts applied, held, abstained (`NO_SIGNAL`, `CLASSIFIER_UNCONFIDENT`). If C applied
   no rewrite on a category, its cost gap there is noise, not routing (the b2 and b6 lesson).
2. **Acceptance per arm and category** with Wilson intervals. `downshift eval-outcome --report` prints them for
   outcome-suite pass rates (`internal/outcome/report.go`); for run acceptance across arms the computation is manual.
3. **Cost per accepted run** per arm and category: the sum of `total_cost_usd` over the runs of a task divided by the
   accepted runs, so a failed attempt is charged to the task, as `report` charges redone children. Bootstrap the
   difference B−C and A−B; `report` does this inside one log for routed versus `CONTROL`, across arms it is manual.
4. **Failure cost**: extra runs, retries and manual fixes needed in C and not in B, priced at their recorded cost.
5. **Attribution**: within C, compare tasks where a downshift was applied with tasks where none was; only the first
   group can credit routing.

The answer has four parts, and each is reported even when it is "no" or "unknown":

| Question | Answered by |
|---|---|
| Does Downshift save beyond native opusplan? | B−C cost per accepted run, with its interval; A−B states what opusplan alone saves |
| In which categories? | the per-category table; expect mechanical and delegation-heavy to carry the effect and the last two categories to show none |
| At what quality and failure cost? | acceptance per arm, retries and failures per arm, and the failure-cost line |
| What remains unmeasured? | the "not available" rows above: latency per step, parent coordination tokens, quality beyond the checks, invoice dollars |

## Privacy

No prompts, task text, repository paths, transcripts or `events.jsonl` leave the machine. Published material is the
per-run table with opaque ids, the two exports per arm, and the task category definitions. This is the same rule
telemetry follows: counts, rates and catalog-priced dollars only.

## Limits to state in the result

- Dollars are list prices from the catalog or from the harness's own estimate, not an invoice; on a subscription
  plan there is no per-token invoice to compare.
- One operator and one machine is a measurement, not a pilot; the [pilot](pilot.md) is the team-scale protocol.
- Three runs per cell gives a direction. Intervals that include zero are a valid result and are published as such.
- Acceptance checks verify structure and tests, not wording or design quality.
- Failures are undercounted until a real `PostToolUseFailure` payload is captured (GAP-CHILD-FAILURE).
- Other harnesses are out of scope: only Claude Code records usage and child ends today.

## Not available yet

- A runner that sets up the three arms, shuffles runs and aggregates them: the A/B runner is private; here it is manual.
- A duration field on decision, usage or end events.
- Parent-session tokens and turns from inside the hook.
- A quality judge beyond hidden tests.
- An independent outcome suite of 20 or more frozen tasks with per-task cost (GAP-SUITE); this protocol is one way to
  build it.

## Output

One page in `docs/evidence/benchmark-opusplan-<start date>.md`: the arm table with the values used, the task
categories with counts, the per-category results, the four answers, the limits, and the two exports per arm.
