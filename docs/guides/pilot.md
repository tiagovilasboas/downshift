<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Team pilot: routed versus control

A two-week protocol for a team that wants to know whether Downshift lowers the tokens each completed task needs on its own
work. It uses the built-in control group and [`downshift report`](report.md), so nothing new has
to be built. The result is one evidence page that answers a single question: does routing lower
the tokens per completed task without raising rework?

What a pilot closes in [beta-exit.md](../reference/beta-exit.md): P2.3 (non-maintainer exports), P3.4 (linked
usage events) and P3.7 (period comparison), if the numbers are there.

## Design

| Item | Value | Why |
|---|---|---|
| Unit | One subagent spawn | The hook assigns each spawn to routed or control by hashing its routing id. No calendar split, so a heavy week does not skew one side |
| Control share | `DOWNSHIFT_CONTROL_SHARE=0.5` during the pilot | At the default 0.10 the first estimate needs about 200 completed tasks. At 0.5 it needs about 40. Reset to 0.10 or 0 afterwards |
| Harness | Claude Code only | It is the only harness where completion and cost are measurable today (SubagentStop reads the child transcript). See [harness-matrix.md](../reference/harness-matrix.md#child-end-n5-2026-10-13) |
| Primary metric | Tokens per completed task, routed versus control, with the token check | What `downshift report` prints |
| Quality guardrail | Rework rate per group | Downshifting has a measured quality cost on executable tasks; the pilot has to show whether it turns into rework |
| Secondary | `usage_linked`, honored rewrites (`resolved`) | Linkage and provenance |
| Duration | 2 weeks, 3 or more people | One machine is not a pilot |
| Stop rule | At least 20 completed tasks per group (the report unlocks); aim for 50+ per group | Below that the report prints `not enough data` and the pilot shows nothing either way |

## Before day 1 (every participant)

1. Install a build that has `downshift report`. Check with `downshift help`; if `report` is not
   listed, build from `main`.
2. Run `downshift setup`, then `downshift doctor`. PreToolUse, PostToolUse, PostToolUseFailure and
   SubagentStop must be wired for Claude Code.
3. Set the control share in the shell that launches Claude Code:
   ```bash
   export DOWNSHIFT_CONTROL_SHARE=0.5
   ```
   Confirm with `downshift config`.
4. Spawn one trivial subagent, then run `downshift doctor` again. It must report completion as
   measurable for claude-code.
5. Run `downshift stats --detail` and check that the requested model of that spawn is known. With
   an unknown requested model, savings are zero by construction and the pilot measures nothing.
6. Write down the account plan. With an API key, the provider invoice can be compared. With a
   subscription (Pro, Max, Team), there is no per-token invoice: report tokens and quota headroom,
   and say so in the result.

## During the pilot

- Work normally. Do not steer which tasks go to subagents.
- Do not set `DOWNSHIFT_NO_ROUTE=1` or pause sessions during the pilot. Those events are excluded
  from the control group because they are not random, so they shrink the sample.
- Once a week, each participant opens an issue with the
  [pilot report template](../../.github/ISSUE_TEMPLATE/pilot-report.md).

## Reading the result

| Report says | Conclusion |
|---|---|
| `not enough data` | Keep running, or raise the share. Do not quote a number |
| Token check shows routed at or above control | No saving shown in this window. That is a valid result; publish it |
| Routed below control, rework not higher in the routed group | Fewer tokens per completed task for this team and workload |
| Routed below control, rework higher in the routed group | Fewer tokens per completed task, but at a quality cost. Report both numbers |

## Lessons from the first routed-versus-control runs

From [the first measurements](../evidence/subagent-routing-ab-2026-10-15.md); each cost a wasted batch until it was handled.

- **Count the spawns.** Downshift acts only on subagent spawns. A run that spawns none is identical in both arms, so the report is read only over runs with at least 3.
- **Keep the transcript.** Claude Code's baseline comes from the transcript, with the configured model as a marked fallback (`BASELINE_FROM_SETTINGS`). A run without session persistence has neither a transcript nor a recorded model, so nothing is routed and both arms look the same.
- **Parallel Agent calls in the first message** have no transcript model yet; the fallback covers them only when the model is configured in settings or `ANTHROPIC_MODEL`, not with `--model` or `/model` (see [harness-matrix.md](../reference/harness-matrix.md)).
- **A usage limit is not a failed task.** A run refused by the account's usage limit has no cost and did no work; drop it from the comparison and rerun it after the limit resets.
- **Do not credit a gap to routing unless a downshift was applied.** In the first batch the routed arm was cheaper while no downshift was applied: the difference was the agent's own behaviour.

## Limits to state in the result

- Dollars are catalog list prices applied to recorded tokens, not invoice amounts, unless the
  invoice comparison in [billing-comparison-template.md](billing-comparison-template.md) is filled.
- Rework detection depends on the PostToolUseFailure payload, which has not been captured from a
  real session yet (GAP-CHILD-FAILURE). The guardrail may undercount.
- The result holds for this team's workload, not for every team.
- The control group costs money by design, roughly the control share times the saving.

## Output

One page in `docs/evidence/pilot-<team>-<start date>.md`: the design table above with the values
used, each participant's final `downshift report --json` (no prompts or paths), the account plan,
the conclusion row from "Reading the result", and the limits.
