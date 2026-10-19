<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Real usage, one operator: 2026-10-19 and the last 30 days

What this page answers: what did Downshift do on a real working day in Claude Code, and what can and cannot be claimed from it?
Source: `downshift stats --days=1 --detail`, `downshift stats --days=30 --detail`, `downshift report --days=1` and
`--days=30`, run on 2026-10-19 on the maintainer's machine. Aggregates only: no prompts, task text, paths or session ids.

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

## The day

A long Claude Code session that built a landing page, restyled the Claude Code mods, wrote copy and ran audits. The parent
agent delegated most of the work as long briefs to `general-purpose` subagents, so the tasks were natural, not mechanical.

| Measure | 2026-10-19 |
|---|--:|
| Subagent decisions | 61 |
| Downshifted (rewrite applied) | 5 (8.2%) |
| Upshifted (rewrite applied) | 0 |
| Classified shift held by a guardrail, spawn unchanged | 16 (26.2%) |
| Unchanged, model already right | 27 (44.3%) |
| No known baseline | 13 (21.3%) |
| Frontier rewrites on doubt | 0 |
| Complexity: medium / complex / trivial | 70.5% / 24.6% / 4.9% |
| Observed child cost, catalog priced | $50.45 (47 usage records) |
| Estimated net saving, list price | $1.37 (about 2.7% of observed child cost) |
| Control-group spawns / with an end record | 6 / 0 |

## The last 30 days (same operator, several harnesses)

| Measure | 30 days |
|---|--:|
| Subagent decisions | 1,175 |
| Downshifted / upshifted | 193 (16.4%) / 55 (4.7%) |
| Held by a guardrail (safety-held) | 232 |
| Unchanged | 255 (21.7%) |
| No known baseline | 528 (44.9%) |
| Estimated net saving, list price | $22.43 (saved $52.38, cost more $29.96) |
| Normalised units, routed against unrouted | -1.9% |
| Control-group spawns / with an end record | 31 / 0 |

## What this supports

- The hook decided on every delegated spawn without a model call: 61 decisions today, 1,175 in 30 days.
- Doubt did not spend money: 0 frontier rewrites on doubt today; 16 classified moves were held by a guardrail today and 232 in 30 days.
- On natural, design-heavy delegation the router mostly keeps the model: 5 of 61 downshifts today. This matches the
  2026-10-15 trial ([subagent-routing-ab-2026-10-15.md](subagent-routing-ab-2026-10-15.md)): savings concentrate on mechanical work.

## What it does not support

- Any saving percentage for general use. The $1.37 and $22.43 are per-decision list-price estimates, not measurements, and
  the 30-day dollar estimate and the normalised units disagree in sign, so the net effect is inside the noise.
- A routed-versus-control comparison. `downshift report` needs 20 completed tasks per group; the control group had 0
  completed (no end record) both today and over 30 days, so the measured comparison cannot be computed yet.

## Defects this exposed

| Defect | Evidence | Effect |
|---|---|---|
| Control-group spawns never get an end record | 0 of 6 today, 0 of 31 in 30 days, while routed spawns have 43 and 214 | `downshift report` can never compare routed with control |
| Custom agent types have no baseline | 3 of 3 today, 72 of 72 in 30 days unknown (`claude-code-guide` 1 of 1) | those spawns cannot be routed or priced |
