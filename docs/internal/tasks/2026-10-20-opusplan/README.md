<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# opusplan and plan mode: roadmap (2026-10-20)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Problem: with `opusplan` as the main session model, the Downshift effect is hard to see and sometimes
looks like it does not work. What we know from the code and the log, and what we do not:

- Known: `opusplan` governs the main session; Downshift only rewrites subagent spawns. In plan mode the
  baseline is Opus, in execution it is Sonnet (`configuredmodel.go`).
- Known: 52.1M tokens in the last 30 days ran on a pricier model than requested, all sonnet to opus
  upshifts. Six Opus spawns were classed small, and five were held by `CREDIT_HELD`, which could not be
  reproduced with a fresh quota snapshot.
- Unknown: whether a spawn actually runs in plan mode and inherits Opus (documented as not observed).
- Unknown: the source of the `CREDIT_HELD` signal in the real payload.

## Steps

| Step | What | Depends on | Status |
|---|---|---|---|
| 1 | Record `permission_mode` (mode name only) per decision; show it in `session --why` and count it in `report` | none | running (PR) |
| 2 | `doctor` reports whether a spawn in plan mode was observed; live check: plan session, five mechanical subagents, read `session --why` | 1 | waiting |
| 3 | Policy decision: in an `opusplan` session outside plan mode, cap upshifts at the execution model (Sonnet); allow Opus only by risk category | 1 and 2, owner decision | blocked |
| 4 | Review and planning prompts classed COMPLEX ("trade-offs", "concurrent"); table-driven test in `classifier_edge_test.go` | owner decision | blocked |

Rule: a step that needs an owner decision does not start until the decision is recorded here.
