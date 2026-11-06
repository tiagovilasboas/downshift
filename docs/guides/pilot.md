<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Team pilot: routed spawns

**2026-11-02:** the routed-versus-control design of this pilot can no longer be run. The control
group was removed by owner decision, so every spawn is routed and there is no unrouted side to compare
with. This guide now describes a routed-only pilot. It reports tokens per completed task and rework for
a team's own work. It cannot show a saving.

A two-week protocol for a team that wants to see what the routed spawns use per completed task and
how often they are redone, on its own work. It uses [`downshift report`](report.md), so nothing new
has to be built. The result is one evidence page with the numbers and their limits.

What a pilot can still close in [beta-exit.md](../reference/beta-exit.md): P2.3 (non-maintainer
exports) and P3.4 (linked usage events). P3.7, the routed-versus-control token comparison, cannot be
closed by a pilot any more.

## Design

| Item | Value | Why |
|---|---|---|
| Unit | One subagent spawn | The hook routes every spawn. No calendar split |
| Harness | Claude Code only | It is the only harness where completion and cost are measurable today (SubagentStop reads the child transcript). See [harness-matrix.md](../reference/harness-matrix.md#child-end-n5-2026-10-13) |
| Primary metric | Billable tokens per completed task, routed spawns | What `downshift report` prints |
| Quality guardrail | Rework rate, and rework by routing verdict | Downshifting has a measured quality cost on executable tasks; the pilot shows whether it turns into rework, and which verdict it comes from |
| Secondary | `usage_linked`, honored rewrites (`resolved`) | Linkage and provenance |
| Duration | 2 weeks, 3 or more people | One machine is not a pilot |

## Before day 1 (every participant)

1. Install a build that has `downshift report`. Check with `downshift help`; if `report` is not
   listed, build from `main`.
2. Run `downshift setup`, then `downshift doctor`. PreToolUse, PostToolUse, PostToolUseFailure and
   SubagentStop must be wired for Claude Code.
3. Spawn one trivial subagent, then run `downshift doctor` again. It must report completion as
   measurable for claude-code.
4. Run `downshift stats --detail` and check that the requested model of that spawn is known. With
   an unknown requested model, savings are zero by construction and the pilot measures nothing.
5. Write down the account plan. The pilot measures tokens, not money: report tokens and quota
   headroom regardless of plan, and say which plan it was in the result.

## During the pilot

- Work normally. Do not steer which tasks go to subagents.
- Do not pause sessions during the pilot. A paused spawn is not routed and is left out of every
  figure, so it shrinks the sample.
- Once a week, each participant opens an issue with the
  [pilot report template](../../.github/ISSUE_TEMPLATE/pilot-report.md).

One person for one week instead of a team: see the [operator guide](operator.md).

## Reading the result

| Report shows | Conclusion |
|---|---|
| Few completed tasks | Keep running. Do not quote a rate from a handful of tasks |
| Rework low across verdicts | The routed spawns were rarely redone in this window |
| Rework high in one verdict | That verdict needs a look. The report names the policy key that turns it off; changing the policy is your decision |

The report has no unrouted side, so none of these rows says tokens were saved.

## Lessons from the first runs

From [the first measurements](../evidence/subagent-routing-ab-2026-10-15.md), taken when a control arm still existed; each cost a wasted batch until it was handled.

- **Count the spawns.** Downshift acts only on subagent spawns. A run that spawns none shows nothing in the report.
- **Keep the transcript.** Claude Code's baseline comes from the transcript, with the configured model as a marked fallback (`BASELINE_FROM_SETTINGS`). A run without session persistence has neither a transcript nor a recorded model, so nothing is routed.
- **Parallel Agent calls in the first message** have no transcript model yet; the fallback covers them only when the model is configured in settings or `ANTHROPIC_MODEL`, not with `--model` or `/model` (see [harness-matrix.md](../reference/harness-matrix.md)).
- **A usage limit is not a failed task.** A run refused by the account's usage limit has no cost and did no work; drop it and rerun it after the limit resets.

## Limits to state in the result

- Downshift reports tokens only, never dollars. Downshift does not control cost and the pilot does
  not measure money.
- There is no comparison side: the figures describe the routed spawns and do not say what they
  would have used on the requested model.
- Rework detection depends on the PostToolUseFailure payload, which has not been captured from a
  real session yet (GAP-CHILD-FAILURE). The guardrail may undercount.
- The result holds for this team's workload, not for every team.

## Output

One page in `docs/evidence/pilot-<team>-<start date>.md`: the design table above with the values
used, each participant's final `downshift report --json` (no prompts or paths), the account plan,
the conclusion row from "Reading the result", and the limits.
