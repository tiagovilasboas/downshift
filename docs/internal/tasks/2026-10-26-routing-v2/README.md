<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Routing v2: roadmap and task cards (2026-10-26)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Superseded for sequencing on 2026-10-27 by the [MVP cards](../2026-10-27-mvp/README.md); R numbers stay as
cross-references.

Design: [`../../../design/routing-v2.md`](../../../design/routing-v2.md) (Proposed). Owner decisions of
2026-10-26: stop experiments; rebuild model routing on the Alethe-orchestrator idea (declared decision,
rules, headroom), clean-room; keep the Compactor and DCI as they are; screaming architecture with one
responsibility per domain package; harness-agnostic; skills through the shared `.agents/skills` store; Kiro
covered through native agent config and launch flags.

**Order (owner decision, 2026-10-26): Claude Code first.** Claude Code is the central harness: v2 is
delivered and proven there end to end (stage 1), then the other harnesses get their channels (stage 2).
This keeps the layering rule: the decision (`routing/*`) lives in shared layers from day one and every
harness computes it; only the *apply* channels arrive per harness. A harness without its channel yet keeps
today's behaviour and says so in the harness matrix.

Gate for every code card: `make check`, `./examples/run-all.sh`, golden decision tables, the layering rule,
copyright headers, OS-agnostic paths, no file over 300 lines, tests isolate `HOME` and `DOWNSHIFT_STATE_DIR`.
Each card is one PR, at most 5 points, mergeable alone. v2 ships behind `DOWNSHIFT_ROUTING=v2` until R9.

## Plan

| # | Card | Pts | Depends on | Parallel with |
|---|---|--:|---|---|
| R0 | **Fix #570 regression:** Claude Code candidate set `haiku/sonnet/opus` as `HarnessCapabilities` data; routing never needs a session list | 2 | — | R1 |
| R0b | Verify whether Claude Code PreToolUse fires for `Workflow` `agent()` spawns and what `model` it carries; result goes into design §11 | 1 | — | R0, R1 |
| R1 | Approve design (this PR) and inventory rows in `docs/contrib/coverage.md` for `internal/routing/*` | 1 | — | R0 |
| R2 | `routing/policy`: schema (profiles, rules, gates, tiers), loader, `downshift policy check`; config errors refused at load | 5 | R1 | R3, R4 |
| R3 | `routing/headroom`: strain from existing quota readings (worst window, rate-limited = +inf, TTL → missing; missing passes) | 3 | R1 | R2, R4 |
| R4 | `routing/hint`: enum prefix parser (`kind`, `effortClass`, `tier`) + agent-type hint; anything else ignored | 2 | R1 | R2, R3 |
| R5 | `routing/guard`: risk floor (reuses `classify` risk scanner only), never widen, explicit_only, foreign id, siblings | 3 | R1 | R2–R4 |
| R6 | `routing/decide` + `routing/note`: precedence §2, tier → model via candidates + catalog; golden tables; wired in `runHookAdapter` behind `DOWNSHIFT_ROUTING=v2` | 5 | R0, R2–R5 | — |
| R7 | Launch channel: `downshift route --harness <h> --json` (stdin hint/agent type; stdout model, effort, tier, note) | 3 | R6 | R8, R10 |
| R8a | Compile channel, Claude Code: `downshift policy apply --harness claude-code [--dry-run]` writes `model:` in `.claude/agents/*.md` (backup, diff) | 3 | R6 | R7, R8b |
| R8b | Compile channel, Kiro: `~/.kiro/agents/*.json` `model` (backup, diff; never a symlinked target) | 3 | R6 | R7, R8a |
| R8c | Compile channel, Codex and OpenCode agent model fields (verify each format first) | 3 | R6 | R8a, R8b |
| R10 | Skill `downshift-route` (hint convention + launch usage) in the shared store `~/.agents/skills` with lock entry; `setup` links it into `.claude/.codex/.config/opencode/.gemini/.kiro` skill dirs (dry-run; copy on Windows) | 3 | R4, R7 | R8* |
| R11 | Telemetry: routing note fields in events (enums only); `report` and `session --panel` show verdicts; `stats` counts from ≠ to, never the verdict | 3 | R6 | R7–R10 |
| R9 | Switch default to v2 (`DOWNSHIFT_ROUTING=v1` keeps the old decider one release) | 2 | R6–R11 | — |
| R12a | Delete shadow stack, part 1: NB opinions, adapt memory, score routing (`nbtier`, `nbopinion`, `adapt`, `scoreindex`) | 3 | R9 | R12b |
| R12b | Delete shadow stack, part 2: `routingv2` shadow, `decisionintelligence`, gateway shadow; classifier demoted to offline suggester | 3 | R9 | R12a |
| R13 | Docs: harness matrix, capability matrix, README routing section, install guide, AGENTS.md architecture lines (≤200 lines) | 3 | R9 | R12* |

Total 51 points, 18 cards.

**Stage 1: Claude Code end to end** (42 points)

| Wave | Cards | Why they do not collide |
|---|---|---|
| 1 | R0, R0b, R1 | capabilities data vs a read-only probe vs docs |
| 2 | R2, R3, R4, R5 | four new packages, no shared files |
| 3 | R6 | integrates the four; touches the hook runner (Claude Code adapter is the reference) |
| 4 | R8a, R10 (Claude Code links only: `~/.claude/skills` → `~/.agents/skills`), R11 | agent-definition writer / skill + setup link / telemetry |
| 5 | R9 for Claude Code (default v2 there; other harnesses stay on v1 until stage 2) | flag flip |
| 6 | R12a, R12b, R13 (Claude Code sections) | disjoint deletions, docs |

Exit of stage 1: on Claude Code, a declared profile or hint moves an Opus request to the declared tier; risk
prompts never go small; with nothing declared the requested model is kept; `report`, `session --panel` and the
Agents panel show the routing note; golden tables green.

**Stage 2: the other harnesses** (9 points), one harness per PR, in this order: KiroCrew (R8b native agent
config + R7 launch flags + block and retry with the v2 decider), Codex (R8c), OpenCode (R8c), Cursor and
Antigravity (hook channel with the v2 decider), skill links for each (`.codex`, `.config/opencode`,
`.gemini`, `.kiro`). R7 (`downshift route`) lands at the start of stage 2: it is the channel for launchers.

## Out of scope (stays as is)

The Compactor (`execwrap`, `compressor`, `shellwrap`), DCI (`codeintel`), the accounting v1 plan
(`2026-10-21-accounting-v1`, paused), batteries and experiments, long-lived worker supervision (an
orchestrator's job, not a router's), persisting task text (never).

## Decisions for the owner

- D1. Policy file location: project `.downshift/policy.json` (exists today for the tier floor) plus a user
  default in the state dir. Recommended: both, project wins.
- D2. Hint syntax in the spawn description: a short enum prefix such as `[kind=mechanical effort=light]`.
  Recommended; the alternative is agent-type only (no prompt convention).
- D3. Native config compile: on demand only (`policy apply`), or also run by `setup`. Recommended: on demand,
  `setup` prints the suggestion.
- D4. Keep the v1 classifier as an offline rule suggester for one release, or delete it with the shadow stack.
