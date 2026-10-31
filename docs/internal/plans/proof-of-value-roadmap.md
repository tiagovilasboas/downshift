<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Roadmap: proof of value in production (2026-10-31 to 2026-11-21)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

## Why this plan

Downshift is well built and its thesis is sound (`docs/true-north.md`), but its value in real use is not shown yet.
The bench result is real and honest about its size: the controlled suite of 2026-10-29 measured 22.2% paired
savings (95% interval 9.8% to 32.3%, `docs/evidence/controlled-suite-opusplan-natural-2026-10-29.md`). Real use
has not repeated it.

State on 2026-10-31 (`downshift report`, v0.3.1, Claude Code, last 30 days, read on this machine):

| Group | Spawns | Ended | Completed | Tokens per completed task |
|---|---|---|---|---|
| routed | 329 | 284 | 278 | 3.9M |
| control | 39 | 34 | 33 | 3.9M |

Tokens per task: routed +1.0% against control; the 95% interval of tokens saved per task is -1.5M to +1.5M, so no
saving is established. Only 39 of 358 routing decisions rewrote the model (the event log has 280 `allow`). The
earlier record of 2026-10-19 (`docs/evidence/real-usage-2026-10-19.md`) reported -1.9% in normalised units.

Two items an outside review raised are already fixed and are not in this plan: control spawns now get end records
(40 of 44 control decisions are linked to an end or usage record), and `cmd/downshift/hookrun.go` is 390 lines
since #641.

The question for these three weeks: **in real use, how much does Downshift save, for whom?** Every card below either
answers it or removes something that does not help answer it.

## Rules for the three weeks

1. **Feature freeze.** No new package, command or MCP tool until P1 closes. Fixes, measurement and removals only.
2. **Publish the number, whatever it is.** A null or negative result is published with the same care as a positive
   one.
3. **One merge driver.** PRs are merged by the session that opened them, with CI green; no second session merges
   into the same files in parallel (two merges on 2026-10-30 landed without the owning session knowing).
4. **Evidence level on every claim**, as the evidence folder already does.

## Phases

### P0: measure what is there (week 1, in progress)

| Card | What | Points | Depends on | Parallel |
|---|---|---|---|---|
| P0.1 | Why routing barely changes anything in real use: break down the 280 `allow` decisions by reason, the share of spawns with a `[tier=...]` hint, per-session token distribution (median, not only mean), control balance by day | 3 | | yes |
| P0.2 | Compactor real savings: 30 and 7 days of exec events, compacted share, bytes in and out per family, passthrough reasons, top missed savings | 2 | | yes |
| P0.3 | CI faster: shard `go test` across parallel jobs, keep `test` as the required check | 2 | | yes |
| P0.4 | Correct the M6 baseline (61 of 196, not 302) | 1 | | done in this PR |

### P1: the real seven-day comparison (issue #103, weeks 1 to 2)

| Card | What | Points | Depends on |
|---|---|---|---|
| P1.1 | Pick the one lever P0.1 says moves the most (a declared policy default, orchestrator guidance, or a decider rule the owner approves) and ship it behind a flag | 3 | P0.1 |
| P1.2 | Fix the measurement window and the minimum sample before it starts: at least 20 completed tasks per group (the report's own floor), seven calendar days, no binary change during the window | 1 | P1.1 |
| P1.3 | Run the window on the owner's normal work | 0 (time) | P1.2 |
| P1.4 | Publish `docs/evidence/real-seven-day-<date>.md` with routed against control, tokens per completed task, the interval, rework, and what it does not show; close #103 | 2 | P1.3 |

Exit: #103 closed with a published number.

### P2: the compactor as the lead (weeks 2 to 3)

The compactor works on every harness and OS without a model-rewrite channel, so it carries no platform risk. P2
starts from what P0.2 measures.

| Card | What | Points | Depends on |
|---|---|---|---|
| P2.1 | Fix the top missed saving P0.2 finds (a family or passthrough reason with the most raw bytes left uncompacted) | 3 | P0.2 |
| P2.2 | `downshift report` leads with compaction savings per harness, tokens only, before routing | 2 | P0.2 |
| P2.3 | README first screen leads with the compactor and its measured number, routing second | 2 | P2.2 |

### P3: outside operators (issue #104, weeks 2 to 3)

| Card | What | Points | Depends on |
|---|---|---|---|
| P3.1 | A one-page operator guide: install, run a week, export with one command, nothing private leaves the machine | 2 | P2.2 |
| P3.2 | An export command that writes only counts and token totals (no prompts, paths or session ids), with a test that fails on any string field it does not allow | 3 | |
| P3.3 | Find two operators and collect one week each; close #104 | 0 (people) | P3.1, P3.2 |

### P4: focus by subtraction (week 3, owner decision first)

`docs/architecture.md` says DCI has no evidence that it improves answers or cost. Packages with no evidence and no
caller on the measured path move to the labs repository until they prove something.

| Card | What | Points | Depends on |
|---|---|---|---|
| P4.1 | Inventory: for each package, its callers on the hook, compactor and report paths, and its evidence record or none | 2 | |
| P4.2 | Owner decision on the list (candidates: `dci`, `semantic`, parts of `codeintel`, `cascade`) | 0 (decision) | P4.1 |
| P4.3 | Move the approved packages out, one PR per package, each green on its own | up to 3 each | P4.2 |

### P5: platform risk (ongoing)

All routing value rests on one channel: `updatedInput.model` in Claude Code's PreToolUse. It broke once
(`CLAUDE_CODE_SUBAGENT_MODEL_FORCE`). Keep `doctor` failing loudly on that case, and keep the worker route
(`delegate` with `--effort` and a model) as the second channel, which the hook cannot replace for effort.

## What success looks like on 2026-11-21

- #103 closed with a published real seven-day number, positive, null or negative.
- The compactor's real savings published and leading the README.
- At least one outside operator's week collected (#104), two if possible.
- No new package added; at least the P4 inventory done and decided.

## Out of scope for these three weeks

- New harness adapters and new MCP tools (the alethe-style `send`, `steer` and worker status).
- Price or dollar figures.
- Hosted or team features.
