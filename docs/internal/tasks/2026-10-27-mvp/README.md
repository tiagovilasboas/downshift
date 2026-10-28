<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# MVP task cards (2026-10-27)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Plan: [MVP plan](../../plans/mvp-plan.md). These cards replace the stage tables in
[routing v2 cards](../2026-10-26-routing-v2/README.md); the R numbers stay as cross-references. Each card is one PR,
at most 5 points, with the gate in the MVP plan section 5.

## Stage 0: clean base (done on the branch, 2026-10-27)

| # | Card | Pts | State |
|---|---|--:|---|
| B0 | Routing v2 design | — | Done |
| B1 | Remove v1 experiments, gateway and orchestration | — | Done |
| B2 | Routing domain contract (`internal/routing/contract`) | — | Done |
| B3 | Telemetry stops importing `core` | — | Done in the root package; `fromcore` still imports `core` by design |
| B4 | Decision behind an interface (`core.Decider`) | — | Done |
| B5 | Headroom from quota readings (`routing/headroom`) | — | Done |
| B6 | Declared policy and `policy check` (`routing/policy`) | — | Done |
| B7 | Delete the v1 decider | 5 | Done in the same change as C3 (no flag, no keep default). Left over: NB files, `internal/nbtier`, gateway files |
| B8 | Remove the dollar accounting (O1) | 2 | Done. The catalog price table stays as a routing signal |
| B9 | Move the competitive analysis out of the public history | 1 | Done |

## Stage 1: setup (not started; can run in parallel with stage 2)

| # | Card | Pts | Depends on |
|---|---|--:|---|
| S1 | Ownership markers on every file setup writes; `--undo` and reinstall respect them | 3 | — |
| S2 | Install the Downshift skill into `~/.claude/skills` and `~/.codex/skills` (marker, backup, dry run; copy on Windows) | 3 | S1 |
| S3 | `doctor` proves the hook fired: last hook time and decision; statusline quota source present | 2 | — |

## Stage 2: right model in Claude Code (done except the real-session proof)

| # | Card | Pts | Depends on |
|---|---|--:|---|
| C0 | Verify whether PreToolUse fires for workflow `agent()` spawns and which `model` it carries (R0b) | 1 | — |
| C1 | Hint parser, enum prefix and agent type; can only move down (R4) | 2 | B2 |
| C2 | Risk guard: risk never goes small, never widen, `explicit_only`, foreign ids (R5) | 3 | B2 |
| C3 | Decider and routing note: precedence explicit > profile > rule > hint > keep; behind `DOWNSHIFT_ROUTING=v2` (R6) | 5 | B4–B7, C1, C2 |
| C4 | Claude quota from the statusline into headroom; all critical warns and keeps (O3) | 3 | B5, S3 |

## Stage 3: show and prove (closes the MVP; not started)

| # | Card | Pts | Depends on |
|---|---|--:|---|
| V1 | Routing note in `report` and `session --panel`; tokens per harness in two columns (R11, O1) | 3 | C3 |
| V2 | Acceptance suite baseline: v2 against the requested-model arm, regressions reported | 3 | C3 |
| V3 | Real Claude Code session as evidence; default on in Claude Code; pause switch; release and migration note (R9, R13) | 3 | V1, V2 |

## Stage 4: plan runner (specification done, see `docs/internal/plans/plan-runner-spec.md`; implementation not started)

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

## Stage 5: reach (not started)

| # | Card | Pts | Depends on |
|---|---|--:|---|
| A1 | `policy apply --harness claude-code [--dry-run]` writes `model:` in `.claude/agents/*.md` (R8a) | 3 | C3 |
| A2 | Route skill: hint convention and launch usage (R10) | 3 | C1, S2 |
| A3 | Codex channel (R8c, Codex half) | 3 | C3 |
| A4 | Kiro channel (R8b) | 3 | C3 |

OpenCode, Cursor and Antigravity are sized before they start, one PR each.

## Totals

Stage 0: done. Stage 1: 8. Stage 2: 14, done except the real-session proof (C0). Stage 3: 9. MVP: 39, about 17 left. Stage 4: 26. Stage 5: 12 before the
remaining harnesses.

## State of stage 2 (2026-10-27)

C1, C2, C3 and C4 are done on the branch (QA PASS at `2330a3e1`). C0 is not verified: nobody has seen the hook decide in a real Claude Code session. Open gaps are in [the roadmap, section 8b](../../plans/roadmap.md).

## Business plan waves (W0 to W7)

The cards from the `downshift-business-plan` alignment are in [the roadmap, section 8d](../../plans/roadmap.md). Every card is at most 5 points. Wave 4 and the orchestrator wait for the owner decisions BP1 and BP2.

## Closing state (2026-10-27)

Branch `downshift/mvp-ponto-zero`, published, no open PR, no worktree, no unpublished work. The business plan repository has no remote; its last commit is local.

| Item | State | Owner decision needed |
|---|---|---|
| Stage 0 | Done | none |
| Stage 2, real-session proof (C0, R0b) | Authorized by the owner, not run | none; run when the owner says go |
| Stage 1, setup | Not started | none |
| Stage 3, report and acceptance (V1, V2, V3) | Not started | none |
| Stage 4, plan runner | Spec done (54 requirements + R53 to R62); not started | G1 acceptance criterion; G4 CLI flags; G10 harness contract table; G12 default tiers |
| Stage 5 and other harnesses | Not started | none |
| Business plan waves W0 to W6 | Cards in roadmap 8d and 8c-ter | BP1 accepted; BP2 deferred; G14 to G16 resolved |
| Dashboard | Discontinued (N9) | removal card W6.6 waits for the owner |
| Release readiness (W7.1 to W7.11) | Cards in roadmap 8d and 8c-quater; W7.1 and W7.2 first, because v2 routes nothing on a fresh install without them | Approved: profile table (N11) |
| Economy mode | W7.9, one tier down on every harness, same bounds | Owner approved the idea; table approved |
| Harness-agnostic rule | N12: tiers are abstract, harness facts are data | None |
| Stage 4 decisions | G1, G4, G10, G12 | Owner |
| Clean-room | Plan runner built from behaviour notes only | none |
