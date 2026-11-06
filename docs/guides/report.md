<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# `downshift report`: routed spawns

`downshift stats` estimates savings against a counterfactual baseline. `downshift report`
measures what routed spawns used. Every spawn is routed, so the report states what the routed
spawns did per completed task and where they were redone.

```bash
downshift report              # last 30 days, as markdown headings and tables
downshift report --days=7
downshift report --json       # same numbers, machine-readable
downshift report --text       # same report as plain columns for a terminal
```

Reading the log is all it does. Nothing is written, and nothing in it is a prompt, a task or a
path: counts, rates and token counts. No output (text, JSON or Markdown) prints a dollar figure or a cross-harness total.

Billable tokens per completed task need to see a child end. The report ends with a "Completion measurable, by
harness" section: a harness is measurable once it declares an end event and at least one end record
(a usage record, or a `child_ended` that is not a launch) is in the window. Harnesses whose end
event is unverified or unknown say so instead of printing zeros. See
[harness-matrix.md](../reference/harness-matrix.md#child-end-n5-2026-10-13).

## Which spawns count

Every spawn is routed. The one way to leave spawns unrouted is the per-session pause
(`downshift session --pause`, or `/downshift:status --pause`), an explicit command you give. A
paused spawn is classified and logged with `outcome: baseline`, and the report counts it as excluded.
Log lines written by older versions with `outcome: baseline` (their control group or no-route
switch) still load and are excluded the same way. The report prints how many it left out.

## What is counted

For each spawn the report joins the routing decision, the usage record with tokens and how the child
ended (the same events `stats` reads).

- **Routed spawns**: every spawn the router was allowed to decide on, including those it left
  unchanged. Counting by intent keeps the result honest: holds and guardrails count.
- **Completed task**: a child that ended `completed` and has a usage record with tokens. Launched,
  stopped, unknown and children without tokens are not counted.
- **Rework**: a respawn in the same session within 10 minutes after a child ended in error or
  was interrupted (the same rule as `session` and `stats`). The failed child counts as reworked.
- **Billable tokens per completed task** = the input, output and cache write tokens of the same
  tasks over the number of completed tasks. Cache reads are left out: they follow the size of the
  context a child rereads, not the tier it ran on, and in real sessions they are most of the total,
  so counting them hides the difference a tier makes. A failed attempt that a completed task redid
  is counted with that task. Failed work that never got a successful redo is not added to anyone.
- **Rework rate** = reworked spawns divided by spawns with an observed end (completed, error or
  interrupted).

## What the text report prints

The report opens with **COMPACTION SAVINGS PER HARNESS**, in tokens only, per harness and never summed (the only place the Compactor tokens are printed) (the Compactor runs on every harness and OS with no model-rewrite channel, so it leads). After it, when the window has more than one harness, the text report shows the **current harness** (the harness of the latest event in the window): the full routed table and its routing tokens. **Other harnesses** follow, one block each, in name order, with the same table and routing rows. A window with a single harness keeps the original report, with no extra headings. `--json` keeps the window-wide counts and adds `current_harness`, `by_harness` (current first) and `compaction_by_harness` (the leading section's per-harness rows; the same Compactor figure is not repeated in `tokens_by_harness`).

The routed table shows tokens only. It has one `routed` row:

| Column | Meaning |
|--------|---------|
| `spawns` | Routed decisions in the window. |
| `ended` | Spawns with an observed end: completed, error or interrupted. |
| `completed` | Completed tasks. |
| `billable/completed` | Billable tokens per completed task (see above). `-` when there is no completed task. |
| `rework` | Rework rate. |

A window with no
routed spawn prints `No routed spawn in this window.`

Under the table, **rework by routing verdict** gives one row per verdict (`kept`, `profile`, `rule`,
`hint_down`, `class`, `class_up`, `default`, `agent_default`, `guard_floor`, `held_unknown`, `refused_config`,
`held_after_failure`, `not_recorded`) with its spawns, ended spawns, completed tasks and rework rate.
Each row names the policy key that turns that verdict off, or says that no key does. The report flags
no verdict: you read the rework rate and decide. It never changes the policy. Keys are listed in
[cli.md](../reference/cli.md#downshift-report).

When decisions in the window carry a permission mode, the text report also prints
`Decisions by permission mode`, with a count per recorded label. Claude Code supplies this field;
the count is descriptive and does not change routing.

**Routing tokens** have their own section in the markdown report, right after the Compactor one
(`Routing: tokens on a lighter model, per harness`): one table row per harness with the subagent tokens, the
part on a lighter model (and its share), and the part a Downshift rewrite moved (`-` when none, `unobserved`
when the harness has no rankable spawn). Notes under the table cover the requested-model fallback, unobserved
spawns and cache read. The text report (`--text`) and `stats` print the per-harness block below (`Routing tokens, by harness`).
They are for plans with no per-token bill (Claude Code Pro, Max, Team; Cursor plans). Each harness block lists, below-session first:

- `ran below the session model`: billable tokens (input, output, cache write) that ran on a lower tier
  than the session model, as `N of M billable (X of Y spawns)`. The session model is the one the parent session ran on
  when it spawned; whoever chose the lower tier (the orchestrator, the agent definition or a Downshift
  rewrite) counts. This is not a count of Downshift decisions: most of it can be spawns that already ran lower.
- `of which by a Downshift rewrite`: the part of the line above whose decision was an applied model rewrite
  (outcome `rewrite_emitted`, no guardrail correction, model changed, not refuted by a `resolved` record), as
  `N spawns (X tokens)`. In `--json`: `rewritten_spawns`, `rewritten_billable`.
- `below the requested model`: only for spawns with no usable session model. A different baseline, labelled
  as such, and never added to the line above.
- `unobserved`: spawns neither baseline could rank. A harness with no data reads `unobserved`, never 0.
- `cache read`: cache-read tokens, apart from billable.

Below the session model is not fewer tokens: the report has no baseline to say whether routing used
fewer, so read billable tokens per completed task and rework next to it. The text report has no
cross-harness total line and no total of the Compactor tokens; the Compactor figure is the leading
section. The quota follows under `QUOTA USED PER HARNESS`.
(Before this change the report ended with `Tokens saved (estimate): N`, summed across harnesses, and printed an
`Effect:` line and a `TOKENS PER HARNESS` table with the routing and Compactor columns again.)

`downshift stats` and every live view (session tree, panel, statusline, watch) show tokens only.

The report states no saving and flags no verdict. It describes the routed spawns of the window; a
small window is small, so read the counts next to every rate.

## Bytes fetched back with `retrieve`

With CCR a compacted run keeps its raw output under an id, and `downshift retrieve <id>` (or the MCP
tool `retrieve`) brings part or all of it back into the context. Those bytes are not a saving, so the
report takes them off: a run's saved bytes are `raw - emitted - retrieved`, floored at zero per run,
and a run that was fetched back in full reports 0 saved. Every figure that counts compaction
(the per-harness tokens, the compaction detail, `--daily` and `--history`, `downshift session`'s
panel, the context summary) uses that net number.

The bytes themselves get their own line under the Compactor figures, in the markdown, `--text` and
styled reports: `Fetched back with retrieve: N bytes (already taken off the saved bytes)`. In JSON it
is `numbers.compactor.retrieved_bytes` and `numbers.display.retrieved` (the session panel's
`compactor.retrieved_bytes`). The line and the keys are absent when nothing was fetched back, and a
log with no retrieval rows gives the totals it always gave. The rows hold only the id, the byte count
and the time; see [ccr.md](ccr.md#retrieved-bytes-are-subtracted-from-the-savings).

## Compaction detail

The default output is markdown: the same sections as `--text`, as headings and tables a harness
chat renders. Under the Compactor figures, each harness gets its own block:

- **By family**: per command family (`go-test`, `git-diff`, `npm-test`...), the commands, how many
  were compacted, the output tokens they printed and the tokens the Compactor kept out of the
  context. Largest output first; the first 8 families are shown, `--json` carries all of them.
  Tokens are bytes / 4, an estimate.
- **Not compacted**: why a wrapped command's output went through whole: the output shape was not
  recognized, the output was already short, the command failed (its output is kept whole), it was
  not allowlisted, or the reason was not recorded.
- **Left unwrapped by the hook**: why a shell call never reached the Compactor: a pipe, list, quote
  or redirect, a command too long, already wrapped, a family that is not allowlisted, a deny or ask
  rule, the Compactor off, or a binary given by absolute path.

These counts point at the next compressor to write (a large family that is rarely compacted) and at
command shapes the hook cannot wrap. `--text` prints the same block as `COMPACTION DETAIL PER
HARNESS`, and `--json` as `compaction_detail`. Skip rows hold no session, so a session scope has no
skip counts. Counts and reason codes only: no command text, argument or path.

## Efficiency

"More work done. Less premium intelligence consumed." The Efficiency section follows the Compactor
section (`EFFICIENCY PER HARNESS` in `--text`, `efficiency` in `--json`). It is built from the
routing, usage and end records already in the log: no new collection, no money, and one block per
harness, never summed across harnesses. The Compactor tokens are printed once, in the leading
section, and are not repeated here.

- **Premium preserved**: of the completed tasks (a child that ended completed with a usage record,
  the same definition as the routed table), the share that ran on a non-frontier tier, as `75% (3 of
  4 completed tasks ran below frontier)`. It says the work finished without a frontier model. It is
  not a token saving and not a cost saving: a lower tier is not fewer tokens, and the tokens still
  ran.
- **Tier the task ran on**: the model that actually ran, not the classifier's recommendation. A
  usage record names it and wins; with none, it is the applied model of the decision (the final
  model of an applied rewrite, the requested model of an allow, a hold, or a rewrite the harness
  reported as not honored). A resumed child takes its highest tier. A rewrite the harness never
  confirmed is assumed honored, as in the routing tokens. A model the catalog does not rank is
  `unknown`.
- **Unknown tier**: completed tasks of unknown tier are left out of the ratio and counted
  (`; 2 with unknown tier left out`). With no completed task of known tier the figure reads
  `unobserved`, never 0%.
- **Spawns by tier**: all routed spawns of the window by the tier they ran on, small, mid, frontier
  and unknown, as counts and shares of the harness's routed spawns.
- **Completed and rework**: the routed table's own figures (`completed` of `ended`, and the
  rework rate: failed spawns that were redone, over ended), taken from the same report code, not
  recomputed. In `--json` the efficiency block gives `rework_ratio` as a 0-1 fraction, while
  `premium_preserved_pct` and the tier `pct` shares are 0-100.

Read the shares next to the counts: a small window is small.

## The JSON report

`--json` includes counts and rates, plus token figures: `tokens` by class (`input`, `output`,
`cache_read`, `cache_write`,
`total`, `records`) for the routed spawns, `billable_tokens_per_completed_task`, `tokens_per_completed_task`
(cache reads included, for reference), `routing_tokens` (per harness; the same rows as
[stats-export.md](../reference/stats-export.md#routing-tokens), replacing `effect` and `tokens_saved`) and the completion status per
harness. When permission-mode data exists, it also includes the optional `decisions_by_permission_mode`
map. Only built-in Claude Code mode names are stored (`default`, `acceptEdits`, `bypassPermissions`,
`plan`, `auto`, `dontAsk`); an unrecognized value is recorded as `other`, and a missing value is omitted.
This count is informational and does not affect routing. The report has no dollar key. `excluded_unrouted` counts the paused spawns and the old `baseline` lines left out. `rework_by_verdict` carries the verdict rows, each with `reworked_correlation_ids`: the `correlation_id` of every decision of that verdict that was followed by rework. `downshift stats --export` has no dollar key either (schema 4; see
[stats-export.md](../reference/stats-export.md)).

## Limits

- The report counts tokens and prices none of them. No dollar figure is printed, and no invoice is implied.
- Tasks differ in size and nothing here is randomly assigned, so the report shows what routed
  spawns used, not what they would have used on the requested model. Read the rework rate and the
  counts, not only the per-task figure.
- A harness that does not report usage or child ends contributes spawns but no completed tasks.
- Rework needs a session id; spawns without one are never linked.
- An end without a tool-call id links by session only to an applied rewrite, so an unchanged
  routed spawn on such a harness has no end.
- Before 2026-10-19 a Cursor spawn could also be logged as a claude-code decision (Cursor runs
  the Claude Code hooks); those copies never end, so they inflate the routed spawn count of
  older windows. Current versions leave the copy to the cursor hook and record nothing.

See [stats-export.md](../reference/stats-export.md) for the counters `stats --export` adds around this.
