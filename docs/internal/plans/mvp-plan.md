<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Downshift MVP plan: from a clean base to the complete product

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **Accepted, 2026-10-27.** This is the execution order. It consolidates the [routing v2 roadmap](roadmap.md)
(D1 to D13), the Alethe roadmap proposal (A1 to A4, kept outside this repository) and the owner decisions of
2026-10-27 in section 2. Where they disagree, this file wins. Cards: [2026-10-27 MVP cards](../tasks/2026-10-27-mvp/README.md).

## 1. Product thesis

Every task runs on the cheapest model that can do it, and the user never runs out of usage. The thesis comes from
the Alethe analysis (clean-room, no code copied). Downshift acts on it where Alethe only warns: its quota awareness
is a warning chip with no redirect, by its own roadmap.

## 2. Owner decisions, 2026-10-27

- **O1 No dollar accounting, ever.** Savings are reported as tokens per harness. Plan budgets use calls, wall time
  and tokens. The paused `accounting` plan and price tables are removed. Supersedes roadmap D7 and premise P2's
  dollar line.
- **O2 Quota from native sources only.** Claude Code statusline `rate_limits` (five-hour and seven-day windows) and
  the Codex usage report, both already parsed by `internal/quota`. Downshift never reads a login token or keyring.
- **O3 All providers critical: warn and keep the requested model.** No block, no forced cheapest.
- **O4 No task text on disk.** Confirms A3. An interrupted plan is lost; output is written only where `--out` says.
- **O5 Escalation on failure is opt-in.** Off by default. When on: one retry, one tier up, never above the requested
  model, counted against the plan budget. Plan runner only; a hook cannot retry a Claude Code subagent.
- **O6 Setup can start before the decider.** It does not depend on it.
- **O7 Downshift only.** No external orchestrator, proxy or desktop app. Claude Code stays the orchestrator inside
  its sessions; Downshift routes its spawns and runs its own batch plans.
  **Reopened 2026-10-28 (owner):** an MCP delegation server is in scope, as part of Downshift itself, run by the
  same binary (`downshift mcp`, stdio). It is not a separate product. The LLM chooses agent, model and effort
  through it, as Alethe does on its MCP path.
- **O8 The LLM chooses; Downshift applies and measures (2026-10-28, owner).** The orchestrator decides the model,
  the effort and the decomposition of the work, following the Orchestrator-Workers pattern of Anthropic's cookbook.
  Downshift never picks a model on its own: no built-in agent-type table, no automatic upgrade after a failure, no
  built-in risk floor. Downshift still applies what the user declares in `policy.json` (rules, profiles, floors) and
  the tier hint the orchestrator sends, and it still refuses unknown or `explicit_only` ids. Good defaults move from
  code into an orchestrator playbook (a skill installed by setup). Supersedes the N11 table and the risk veto of
  routing-v2; a project that wants a risk floor declares it in its policy.

## 3. Execution order

```
 0 CLEAN BASE ──┬── 1 SETUP ─────────────┐
                └── 2 RIGHT MODEL IN CC ─┴── 3 SHOW AND PROVE ══ MVP ══ 4 PLAN RUNNER ── 5 REACH
```

| Stage | What the user gets | Points | From Alethe | From Anthropic patterns |
|---|---|--:|---|---|
| 0 Clean base | Nothing visible; v1 experiments gone, contract and seams in place | 8 left | worst-window headroom | — |
| 1 Setup | Install that marks what it owns, backs up, installs the skill, proves the hook fires | 8 | owned install, skills | — |
| 2 Right model in Claude Code | The orchestrator's choice is applied; declared rules and hints bound it; no built-in table or risk floor (O8); near the limit declared rules send simple work down | 14 | tiers, declared decision | routing |
| 3 Show and prove | Why each spawn changed or not; tokens per harness; acceptance unchanged; default on | 9 | visible quota state | evaluator |
| **MVP** | stages 0 to 3 | **39** | | |
| 4 Plan runner | A batch of tasks across Claude and Codex with strategy, budget, worktrees, acceptance | 26 | strategies, budgets, worktrees, worker contract | plan big, execute small; orchestrator-workers; parallelization |
| 5 Reach | `.claude/agents` model fields, route skill, Codex, Kiro, then OpenCode, Cursor, Antigravity | 12+ | multi-vendor workers | — |

Stages 1 and 2 run in parallel (no shared files). Everything else is in order.

## 4. Layers that cross every stage

| Layer | State | Used by |
|---|---|---|
| **Token accounting per harness** | `telemetry`, token counts only | 3 (report), 4 (budget) |
| **Quota** | `internal/quota` parses native snapshots; `routing/headroom` assesses them | 2 (decider), 4 (provider choice) |
| **Compactor** | shipped (`execwrap`, `compressor`, `shellwrap`) | 1 (installed with setup), 4 (workers run with it) |
| **Code intelligence** | shipped, opt-in (`codeintel`) | 4 (optional worker context) |
| **Quality** | acceptance suite (`outcome`, `benchmark`) | 3 (gate before default on), 4 (worker acceptance) |

Report shape, one line per harness, never summed:

```
 harness       tokens avoided   tokens moved to       quota used
               (Compactor)      a smaller model       (5h · week)
```

Routing does not reduce tokens; it runs them on a smaller model, which spends less quota. Only the Compactor
removes tokens. The two columns stay apart in every report and claim.

## 5. Engineering premises (owner, 2026-10-27)

Apply to every card, in every stage.

- **Screaming architecture.** Directories name the domain (routing, policy, quota, setup, plan), never the framework or a technical layer.
- **Clean code, SRP, DRY, KISS, YAGNI.** One responsibility per function, method and package. No abstraction without a second real user; no feature for a case nobody has.
- **Layers by domain.** A diagnostic core decides and knows nothing about harnesses. Each harness has an adapter that only parses its payload and emits its response. Adapters hold no policy (the layering rule in `AGENTS.md`).
- **Accounting apart.** Token accounting is its own layer. No feature or module reads it to decide, and every module works when accounting is absent.
- **Each module runs on its own.** A feature must work in isolation: its tests pass without the others, and it can be switched off without breaking the rest.
- **One branch.** All stages go on one branch, `docs/mvp-plan`, merged by stage. Work lands in small deliveries of value, one phase at a time.
- **Tests at both levels.** Every card carries unit tests and end-to-end tests of the behaviour the user sees (hook payload in, decision and note out; plan dry-run and run with fake workers).

Every code card: `make check` green; golden decision tables; layering rule; copyright headers; OS-agnostic paths
(Windows from day one for setup and worktrees); no file over 300 lines; tests isolate `HOME` and
`DOWNSHIFT_STATE_DIR`; no task text persisted. Stage 3 adds: the acceptance suite shows no regression against the
requested-model arm before the default turns on in Claude Code.

## 6. Non-goals

Workspace or terminal manager, board, personas, Spotify, themes, LAN remote control, any network listener, login
token or keyring reads, permission-bypass flags, a routing proxy, dollar figures, persisted task text.

## 7. Open items

- The leaked competitive analysis on the public cleanup branch needs a history rewrite (owner approval pending).
- Tests that time out under heavy machine load (KiroCrew discovery, run and hold) need a deterministic fix.
- Business plan D1: the 4.1% and 8-of-158 figures stay unpublished until their window is fixed.
