<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MVP task cards (2026-10-27)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Plan: [MVP plan](../../plans/mvp-plan.md). These cards replace the stage tables in
[routing v2 cards](../2026-10-26-routing-v2/README.md); the R numbers stay as cross-references. Each card is one PR,
at most 5 points, with the gate in the MVP plan section 5.

## Stage 0: clean base (in review)

| # | Card | Pts | State |
|---|---|--:|---|
| B0 | Routing v2 design | — | PR #580, green |
| B1 | Remove v1 experiments, gateway and orchestration | — | PR #581, draft, green |
| B2 | Routing domain contract (`internal/routing/contract`) | — | PR #583, green |
| B3 | Telemetry stops importing `core` | — | PR #584, draft, green |
| B4 | Decision behind an interface (`core.Decider`) | — | PR #585, draft, green |
| B5 | Headroom from quota readings (`routing/headroom`, R3) | — | branch `feat/routing-headroom`, no PR |
| B6 | Declared policy and `policy check` (`routing/policy`, R2) | — | branch `feat/routing-policy`, no PR |
| B7 | Wave B: delete the v1 decider per the [cutover plan](../../../design/routing-v2-core-cutover.md) | 5 | not started |
| B8 | Remove `accounting` and price tables (O1) | 2 | not started |
| B9 | Move the competitive analysis out of the public branch history | 1 | needs owner approval |

## Stage 1: setup (parallel with stage 2)

| # | Card | Pts | Depends on |
|---|---|--:|---|
| S1 | Ownership markers on every file setup writes; `--undo` and reinstall respect them | 3 | — |
| S2 | Install the Downshift skill into `~/.claude/skills` and `~/.codex/skills` (marker, backup, dry run; copy on Windows) | 3 | S1 |
| S3 | `doctor` proves the hook fired: last hook time and decision; statusline quota source present | 2 | — |

## Stage 2: right model in Claude Code

| # | Card | Pts | Depends on |
|---|---|--:|---|
| C0 | Verify whether PreToolUse fires for workflow `agent()` spawns and which `model` it carries (R0b) | 1 | — |
| C1 | Hint parser, enum prefix and agent type; can only move down (R4) | 2 | B2 |
| C2 | Risk guard: risk never goes small, never widen, `explicit_only`, foreign ids (R5) | 3 | B2 |
| C3 | Decider and routing note: precedence explicit > profile > rule > hint > keep; behind `DOWNSHIFT_ROUTING=v2` (R6) | 5 | B4–B7, C1, C2 |
| C4 | Claude quota from the statusline into headroom; all critical warns and keeps (O3) | 3 | B5, S3 |

## Stage 3: show and prove (closes the MVP)

| # | Card | Pts | Depends on |
|---|---|--:|---|
| V1 | Routing note in `report` and `session --panel`; tokens per harness in two columns (R11, O1) | 3 | C3 |
| V2 | Acceptance suite baseline: v2 against the requested-model arm, regressions reported | 3 | C3 |
| V3 | Real Claude Code session as evidence; default on in Claude Code; pause switch; release and migration note (R9, R13) | 3 | V1, V2 |

## Stage 4: plan runner

| # | Card | Pts | Depends on |
|---|---|--:|---|
| P1 | Plan schema v1, `plan validate`, `plan strategies` | 3 | V3 |
| P2 | `plan dry-run`: strategy, providers, tiers and budget; writes nothing | 3 | P1 |
| P3 | `plan run` with a Claude worker: stdin prompt, fixed worker contract, mechanical acceptance | 5 | P2 |
| P4 | Codex worker | 3 | P3 |
| P5 | Worktrees for write workers; explicit cleanup command | 3 | P3 |
| P6 | Budget admission: calls, wall time, tokens; a missing reading lets work through | 3 | P2 |
| P7 | Provider choice by headroom across Claude and Codex | 3 | P4, P6 |
| P8 | Opt-in escalation: one retry, one tier up, within budget (O5) | 3 | P3, P6 |

## Stage 5: reach

| # | Card | Pts | Depends on |
|---|---|--:|---|
| A1 | `policy apply --harness claude-code [--dry-run]` writes `model:` in `.claude/agents/*.md` (R8a) | 3 | C3 |
| A2 | Route skill: hint convention and launch usage (R10) | 3 | C1, S2 |
| A3 | Codex channel (R8c, Codex half) | 3 | C3 |
| A4 | Kiro channel (R8b) | 3 | C3 |

OpenCode, Cursor and Antigravity are sized before they start, one PR each.

## Totals

Stage 0: 8 left. Stage 1: 8. Stage 2: 14. Stage 3: 9. MVP: 39. Stage 4: 26. Stage 5: 12 before the
remaining harnesses.
