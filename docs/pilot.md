<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Team pilot: routed versus control

A two-week protocol for a team that wants to know whether Downshift saves money on its own
work. It uses the built-in control group and [`downshift report`](report.md), so nothing new has
to be built. The result is one evidence page that answers a single question: does routing lower
the cost per completed task without raising rework?

What a pilot closes in [beta-exit.md](beta-exit.md): P2.3 (non-maintainer exports), P3.4 (linked
usage events) and P3.7 (period comparison), if the numbers are there.

## Design

| Item | Value | Why |
|---|---|---|
| Unit | One subagent spawn | The hook assigns each spawn to routed or control by hashing its routing id. No calendar split, so a heavy week does not skew one side |
| Control share | `DOWNSHIFT_CONTROL_SHARE=0.5` during the pilot | At the default 0.10 the first estimate needs about 200 completed tasks. At 0.5 it needs about 40. Reset to 0.10 or 0 afterwards |
| Harness | Claude Code only | It is the only harness where completion and cost are measurable today (SubagentStop reads the child transcript). See [harness-matrix.md](harness-matrix.md#child-end-n5-2026-10-13) |
| Primary metric | Cost per completed task, routed versus control, with the 95% interval | What `downshift report` prints |
| Quality guardrail | Rework rate per group, plus misroutes flagged with `downshift feedback` | Downshifting has a measured quality cost on executable tasks; the pilot has to show whether it turns into rework |
| Secondary | `usage_linked`, honored rewrites (`resolved`) | Linkage and provenance |
| Duration | 2 weeks, 3 or more people | One machine is not a pilot |
| Stop rule | At least 20 completed, priced tasks per group (the report unlocks); aim for 50+ per group | Below that the interval usually includes zero and the pilot shows nothing either way |

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
- When a routed child clearly failed because its model was too small, mark it:
  `downshift feedback <id> retry --retry-tier=<TIER> --required-tier=<TIER>`. List pending ids with
  `downshift feedback list`.
- Do not set `DOWNSHIFT_NO_ROUTE=1` or pause sessions during the pilot. Those events are excluded
  from the control group because they are not random, so they shrink the sample.
- Once a week, each participant opens an issue with the
  [pilot report template](../.github/ISSUE_TEMPLATE/pilot-report.md).

## Reading the result

| Report says | Conclusion |
|---|---|
| `not enough data` | Keep running, or raise the share. Do not quote a number |
| Interval includes zero | No saving shown in this window. That is a valid result; publish it |
| Interval above zero, rework not higher in the routed group | Saving shown for this team and workload |
| Interval above zero, rework higher in the routed group | Cheaper per completed task, but at a quality cost. Report both numbers |

## Limits to state in the result

- Dollars are catalog list prices applied to recorded tokens, not invoice amounts, unless the
  invoice comparison in [billing-comparison-template.md](billing-comparison-template.md) is filled.
- Rework detection depends on the PostToolUseFailure payload, which has not been captured from a
  real session yet (GAP-CHILD-FAILURE). The guardrail may undercount; `downshift feedback` is the
  backstop.
- The result holds for this team's workload, not for every team.
- The control group costs money by design, roughly the control share times the saving.

## Output

One page in `docs/evidence/pilot-<team>-<start date>.md`: the design table above with the values
used, each participant's final `downshift report --json` (no prompts or paths), the account plan,
the conclusion row from "Reading the result", and the limits.
