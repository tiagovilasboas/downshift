<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Product roadmap: from router to cost control plane

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

Owner-facing plan. Status as of 2026-10-13. The public summary stays in
[ROADMAP.md](../../ROADMAP.md); engineering task specs live in `docs/internal/tasks/`.

## Positioning

**Today:** "picks a cheaper model for each subagent".
**Target:** "the neutral cost control plane for AI coding agents: proves how much
you saved without losing quality, corrects itself when it is wrong, and gives a
team one policy and one report across every harness".

Why this position: harnesses are absorbing basic routing (Claude Code
`opusplan` and per-agent models, Cursor Auto). A better classifier alone becomes
someone's built-in feature. Cross-harness policy, proof and reporting do not,
because each vendor only sees its own product.

## North-star metric

**Cost per completed task, at equal quality**, routed vs control, with a
confidence interval. Quality = the task did not need a retry, respawn or a
higher-tier redo.

Guard metrics: frontier-on-doubt count = 0; real overspend (USD) visible; retry
rate per tier; share of spawns routed; share of spend covered (subagents vs main
session).

## Milestones

| # | Milestone | Outcome | Gate to exit |
|---|-----------|---------|--------------|
| M0 | Trustworthy core | No silent overspend; metrics can show a loss | done 2026-10-13, see "Done" |
| MX | Visible in the flow | The developer sees subagents and routing live, inside the harness | U1-U2 live, U3-U4 next |
| M1 | Proof of value | "Saved $X, quality unchanged" with a CI, automatically | 2 weeks of real data, control group, report |
| M2 | Self-correcting | Downshift raises its own tier when cheap fails | retry-rate guard live, explainable decisions |
| M3 | Zero config, every harness | Install and it works; session models and capability learned from public data | S1-S4 live, discovery at session start |
| M4 | Covers where the money goes | Main session and workflow agents, not only subagents | plan/execute split where the harness allows |
| M5 | Team and company | Central policy, budgets, aggregated prompt-free reporting | first team pilot |

## M0: trustworthy core (done)

| ID | Item | Status |
|----|------|--------|
| P0.1 | R7/R8: doubt never spends money; semantic boost capped; PT-BR signals | done (`c8f1f8b`) |
| P0.2 | Inferred session on Claude Code (`InferSession`) | done (`c25afe7`, `aa3a538`) |
| P0.3 | `SEMANTIC_BOOST` provenance and frontier-on-doubt alarm | done |
| P0.4 | One `deterministicLift`; tier floor no longer held; dead gate removed (T1) | done |
| P0.5 | Target by catalog tier, not list position (T2) | done |
| P0.6 | `stats` shows a loss: signed delta, overspend, upshift cost (T3) | done |
| P0.7 | `CLAUDE_CODE_SUBAGENT_MODEL` baseline; 32 KB classify cap (T4, T5) | done |
| P0.8 | Final-decision guardrail invariant after every Route layer | done (`8e5cd24`) |

## MX: visible in the flow

Why: the product's work is invisible unless the developer opens a report. Seeing
each subagent's model, cost and redo inside the harness is what builds trust.
Specs: `docs/internal/tasks/2026-10-13-ux/`.

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| PX.1 | Statusline session summary (U1) | live line, < 50 ms, never breaks the statusline | S | T3 |
| PX.2 | Subagent end line (U2) | one line with model, tokens, cost, would-be cost | S | — |
| PX.3 | `/downshift` command: session tree, why, pause (U3) | tree shows redo (↻) per subagent | M | PX.2 |
| PX.4 | `downshift watch` replaces the `dsmon` build (U4) | same binary, every harness | M | PX.3 |

## M1: proof of value

Why first: without it nobody defends the tool in a budget review, and every
later milestone needs its numbers.

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P1.1 | Built-in control group: a configurable share of spawns (default 10%) is classified and logged but not rewritten, chosen by a hash of the spawn id | control share within 1 pp of target over 500 spawns; `outcome: baseline`; prompt-free | M | — |
| P1.2 | Cost per completed task: link decision, usage and child status into one task record; mark retry/respawn within the window as rework | `stats` prints cost per completed task, routed vs control, with rework counted | M | P1.1 |
| P1.3 | Savings report with a confidence interval (bootstrap over tasks) | `downshift report --days=14` prints "saved $X (CI a..b), rework +/-y pp" | S | P1.2 |
| P1.4 | Billing reconciliation when an admin usage key exists (Anthropic, OpenAI) | report shows catalog-priced vs provider-billed delta | M | P1.3 |
| P1.5 | Dogfood: two weeks on the maintainer Mac with P1.1 on | report committed to `downshift-labs/research/results/` | S | P1.3 |
| P1.6 | Gate the eval pin in CI (T6, option 2: vendored prompt-free summary) | CI fails when the 37/40 floor regresses | S | owner decision |

## M2: self-correcting

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P2.1 | Retry-rate guard: per task shape, if rework after a downshift exceeds a threshold, raise that shape's floor automatically and say so | simulated failures raise the floor; a clean run lowers it back after N successes | M | P1.2 |
| P2.2 | Explainable decision: one human line per spawn ("haiku: rename, no risk, confident") in `systemMessage` and `stats --explain` | covers every reason code; no prompt text | S | — |
| P2.3 | Upshift by capability, not "last id in the list" | decision recorded; tests; `explicit_upshift` keeps the strongest | S | **owner decision** |
| P2.4 | Haiku 5.5 / Sonnet 5.5 measured on the 40-task suite (separate `runs-claude/`) | pass rate per label; decides how much MEDIUM can go to Haiku | S | owner OK to spend plan usage |
| P2.5 | Learned complexity scorer EN/PT-BR (shadow, abstain to MEDIUM) | beats regex on held-out set incl. PT-BR; ~2,400 labels | L | labels |

## M3: zero config, every harness

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P3.1 | Public score index (S1): Epoch AI + OpenRouter, single source per comparison, id map as data | `downshift models scores --status` shows coverage per harness session | M | in progress |
| P3.2 | Setup + daily refresh (S2) | detached, never blocks a hook, `DOWNSHIFT_SCORES=off` | S | done, uncommitted |
| P3.3 | Capability selection (S3), shadow by default | `score_would` in stats; live with `DOWNSHIFT_SCORE_ROUTING=1` | M | done, uncommitted |
| P3.4 | Promote capability selection to live | shadow data shows lower cost per completed task, no rework increase | S | P1.2, P3.3 |
| P3.5 | Session-start discovery for Codex, Cursor, Kiro (their session-start hooks) | lists refreshed per session; `InferSession` enabled only with evidence | M | per-harness evidence |
| P3.6 | Docs for scores (S4) | `docs/reference/scores.md`, attribution, privacy note | S | P3.1 |

## M4: cover where the money goes

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P4.1 | Measure the share of spend that is subagents vs main session | `stats` shows coverage % | S | P1.2 |
| P4.2 | Plan-expensive / execute-cheap mode for the main session where the harness allows (Claude Code `opusplan`-like policy) | opt-in; measured in M1 terms | M | P4.1 |
| P4.3 | Workflow-created agents (GAP-WORKFLOW-AGENTS) | routed or documented as out of reach per harness | M | vendor hook support |

## M5: team and company

| ID | Task | Acceptance | Size | Depends |
|----|------|------------|------|---------|
| P5.1 | Central policy file (allowed models, floors, budgets) distributable by MDM/managed settings | one file applied to every harness on a machine | M | — |
| P5.2 | Aggregated prompt-free reporting: merge exports from many machines, per team/project | one report for N developers | M | P1.3 |
| P5.3 | Budget alerts and overspend alerts (frontier-on-doubt, real overspend) | alert hooks (webhook/Slack) opt-in | S | P5.2 |
| P5.4 | Pilot with one team | written case study with numbers | M | M1-M3 |

## Decisions pending (owner)

1. **P2.3** Upshift target: last id in the list (today) or smallest model that
   meets the tier (recommended).
2. **P1.6/T6** CI gate for the eval pin: option 2 recommended.
3. **P2.4** Spend plan usage to measure Haiku/Sonnet 5.5 (80 short calls).
4. `downshift-labs` suite freeze ("Congelo?").
5. Network: the daily refresh is the first outbound call triggered by a hook
   (anonymous GET, no user data). Keep on by default with `DOWNSHIFT_SCORES=off`,
   or make it opt-in.

## What not to do now

- Do not tune the regex classifier further: precision is not the bottleneck;
  proof and trust are.
- Do not let a public index decide a task's tier: it measures models, not tasks.
- Do not turn capability selection live before M1 numbers exist.
