<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# `downshift report`: routed versus control

`downshift stats` estimates savings against a counterfactual baseline. `downshift report`
measures them: a small share of spawns is left unrouted on purpose (the control group), and
the report compares what routed and control spawns cost per completed task.

```bash
downshift report              # last 30 days
downshift report --days=7
downshift report --json       # same numbers, machine-readable
```

Reading the log is all it does. Nothing is written, and nothing in it is a prompt, a task or a
path: counts, rates and catalog-priced dollars only.

Cost per completed task needs to see a child end. The report ends with a "Completion measurable, by
harness" section: a harness is measurable once it declares an end event and at least one end record
(a usage record, or a `child_ended` that is not a launch) is in the window. Harnesses whose end
event is unverified or unknown say so instead of printing zeros. See
[harness-matrix.md](../reference/harness-matrix.md#child-end-n5-2026-10-13).

## The control group

The shared hook runner decides per spawn, for every harness (Claude Code, Cursor, Codex,
Antigravity and KiroCrew), by hashing one key per spawn: the correlation id when the harness sends
one, else the harness tool-use id, else a fresh id for that call. Claude Code, Cursor and Codex send
only a tool-use id, so the settings hook and the mod entry for one spawn, and any retry, land on the
same side. The same key always lands on the same side, no state is kept, and the share is accurate
to within 1 percentage point. Before 2026-10-19 those three harnesses drew a fresh id per hook call,
so the two copies of a spawn routed by both entries could be split between the groups.

| Variable | Meaning |
|----------|---------|
| `DOWNSHIFT_CONTROL_SHARE` | Fraction of spawns left unrouted. Default `0.10`. `0` disables the control group. Values are clamped to `0..1`; an unparseable value falls back to the default. |

A control spawn is classified and logged exactly like any other, but the hook answers with the
fail-open response: the spawn runs on the model the harness asked for. The event is recorded
with `outcome: baseline` and the reason code `CONTROL`. KiroCrew's policy mode never blocks a
control spawn.

A control spawn's launch, end and usage link to its decision exactly like a routed spawn's: by
the tool-call id, then the agent id (see "Cost linkage" in
[harness-matrix.md](../reference/harness-matrix.md)). Logs written before 2026-10-19 have no end
record for any control spawn: the linker skipped `baseline` decisions, so a window that reaches
back before that date undercounts the control arm.

`DOWNSHIFT_NO_ROUTE=1` and a session pause (`downshift session --pause`) still win and are
unchanged. Their baseline events carry no `CONTROL` code, so the report counts them as
excluded rather than as a control sample: they are not random.

## What is compared

For each spawn the report joins the routing decision, the priced usage record and how the child
ended (the same events `stats` reads).

- **Routed group**: every spawn the router was allowed to decide on, including those it left
  unchanged. Comparing by intent keeps the result honest: holds and guardrails count.
- **Control group**: spawns with the `CONTROL` code.
- **Completed task**: a child that ended `completed` and has a priced usage record. Launched,
  stopped, unknown and unpriced children are not counted.
- **Rework**: a respawn in the same session within 10 minutes after a child ended in error or
  was interrupted (the same rule as `session` and `stats`). The failed child counts as reworked
  in its own group.
- **Cost per completed task** = the completed child's cost plus the cost of every failed
  attempt it redid. Failed work that never got a successful redo is not added to anyone.
- **Tokens per completed task** = the input, output and cache tokens of the same tasks
  (priced usage records, failed attempts they redid included) over the number of completed
  tasks. It is the A/B check that a cheaper model did not need more tokens: the report prints
  routed against control once the savings figure is stated. It is a plain ratio, with no
  interval of its own.
- **Rework rate** = reworked spawns divided by spawns with an observed end (completed, error or
  interrupted).

## Savings and the interval

`--json` carries `tokens` (by class: `input`, `output`, `cache_read`, `cache_write`, `total`,
`records`) and `tokens_per_completed_task` in each group.

The report reads the same numbers through two lenses (owner decision 2026-10-19):

- **API lens**, for whoever pays per token. Dollars are the recorded tokens priced at the
  catalog's current list prices when the report runs (`telemetry.RepriceUsage`; the header names
  `prices_as_of`), so a catalog correction reaches the whole history. Which records are priced
  stays what the recorder decided; only the numbers change. Savings are `control cost per
  completed task - routed cost per completed task`, in dollars only: no percentage, which would
  hide the price table it rests on. The interval is a 95% percentile bootstrap (2,000 resamples of
  each group, independent, fixed seed), so the same log always prints the same numbers. When the
  interval includes zero the report says the window does not show a saving.
- **Plan lens**, for plans with no per-token bill (Claude Code Pro, Max, Team; Cursor plans):
  tokens only. Tokens saved from the requested model in the window, work (input+output) and cache
  (read+write) apart (`tokens_saved` in `--json`), and tokens per completed task, routed against
  control.

`downshift stats` and every live view (session tree, panel, statusline, watch) show tokens only;
dollars appear here.

With fewer than 20 completed, priced tasks in either group the report prints
`not enough data` and no savings figure. At a 10% control share that means roughly 200 completed
tasks before the first estimate.

## Limits

- Dollars are catalog list prices applied to recorded tokens, not provider invoices.
- Tasks differ in size. Random assignment balances that in expectation, but a small window can
  still be skewed: read the interval, not only the point estimate.
- A harness that does not report usage or child ends contributes spawns but no completed tasks.
- Rework needs a session id; spawns without one are never linked.
- An end without a tool-call id links by session only to an applied rewrite, in both groups, so
  a control or unchanged routed spawn on such a harness has no end.
- Before 2026-10-19 a Cursor spawn could also be logged as a claude-code decision (Cursor runs
  the Claude Code hooks); those copies never end, so they inflate the routed spawn count of
  older windows. Current versions leave the copy to the cursor hook and record nothing.
- The control group costs money by design: about `DOWNSHIFT_CONTROL_SHARE` of the saving. Lower
  it once the interval is tight, or set it to `0`.

See [stats-export.md](../reference/stats-export.md) for the counters `stats --export` adds around this.
