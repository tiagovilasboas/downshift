<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Routing v2: phased roadmap

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: Accepted 2026-10-26 (D1 to D13); sequencing superseded 2026-10-27 by the [MVP plan](mvp-plan.md), which wins where they differ. Scope: model routing and its channels. The Compactor (`execwrap`, `compressor`, `shellwrap`) and DCI (`codeintel`) do not change.

Sources: [routing-v2 design](../../design/routing-v2.md), [cards](../tasks/2026-10-26-routing-v2/README.md), [true-north cleanup](../tasks/2026-10-26-routing-v2/true-north-cleanup.md). The Alethe analysis (REA) is kept outside this repository and is cited by name only.

Phases end at exit criteria, not dates. Where two sources disagree, section 9 names the conflict and this roadmap states which side it follows.

## 1. Premises (2026-10-26)

- **P1 Clean-room base.** The REA is the only input from Alethe. No Alethe source, prompt or schema is used (AGPL-3.0).
- **P2 Tokens only.** Savings are tokens per harness; no dollar account (owner decision O1, 2026-10-27, in the [MVP plan](mvp-plan.md)).
- **P3 Setup in the Alethe shape.** Declared configuration, validated at load, applied by an explicit step that runs a dry run and a diff, and makes a backup first. This is my reading of the brief's wording.
- **P4 No frills.** A card ships only what a decision or a measurement needs.

## 2. Starting point (branch `downshift/true-north`, commit 8c1afdfa)

- Removed: wave A except `internal/scoreindex` (with `cmd/downshift/scores*.go` and `scorerefresh*.go`). `internal/gateway` is also gone, with no importers; this answers D-T1.
- Still present: `internal/classify` (`risk.go` is the only part R5 needs), `fuse`, `semantic`, `session`, `nbtier`, the v1 decider in `internal/core`, and `cmd/downshift/try.go` with `cmd_try.go`.
- Not present: `internal/routing/*`. `go test ./internal/docscheck/` passes.
- Residue: 18 lines in [architecture.md](../../design/architecture.md). About 36 docs (README, CHANGELOG, `docs/README.md`, and `docs/design`, `contrib`, `features`, `guides`, `domains`, `reference`) name a deleted package or command, by grep. `docscheck` checks dead package paths only in AGENTS.md and the CHANGELOG Unreleased section.

## 3. Summary

| Phase | Goal | Cards (points) | Closes when |
|---|---|---|---|
| 1 Clean base | v1 decider removed; hook keeps the requested model | R12a, R12b (6), plus cleanup units | wave B green, docs clean |
| 2 Routing core | v2 decider behind `DOWNSHIFT_ROUTING=v2` | R0, R0b, R1, R2, R3, R4, R5, R6 (22) | golden tables and acceptance suite |
| 3 Setup and skill | launch channel, Claude Code compile, skill link | R7, R8a, R10 (9) | dry run and rollback proven |
| 4 Accounting and report | routing note in reports; v2 default on Claude Code | R11, R9, R13 (8) | stage 1 exit |
| 5 Other harnesses | one PR per harness | R8b, R8c (6), Cursor and Antigravity (no card) | each harness PR green |
| 6 Later | budget admission, flag probe, hosted evaluation | none | decision note per item |

Phases 1 to 5 hold 18 cards and 51 points. Phase 6 has no cards.

## 4. Common gate (every code PR)

`make check` green; golden decision tables; the acceptance suite (`outcome`, `benchmark`) at the same rigour as before, with any regression reported; `./examples/run-all.sh`; layering rule (harness facts in `HarnessCapabilities`, adapters hold no policy; see [layering](../../contrib/layering.md)); copyright headers; OS-agnostic paths; no file over 300 lines; tests isolate `HOME` and `DOWNSHIFT_STATE_DIR`; no dead package references in the docs the PR touches.

## 5. Phases

### Phase 1: clean base

Cards R12a and R12b (3 points each). The README puts them after R9. This roadmap moves them here, because their deletions are the true-north waves that come before v2. The README and design §7 need the same change when this is accepted.

Cleanup units (no card):
1. `scoreindex` with `scores*.go` and `scorerefresh*.go`: the last wave-A row.
2. Telemetry gets its own enum types, so `telemetry` stops importing `core`, `classify`, `fuse` and `session`.
3. Wave B: `fuse`, `semantic`, `session`, `nbtier`, the v1 decider in `core` (`core.Route` keeps the requested model), `cmd_try.go`, `try.go` and their tests.
4. `classify`: delete everything except `risk.go` and its equivalence test. R5 moves those in Phase 2, and the package goes then. The true-north doc lists all of `classify` under wave B, which cannot happen before R5.
5. `docs/design/architecture.md`, `docs/contrib/coverage.md`, `docs/contrib/layering.md`: describe the post-cleanup graph.

Parallel: units 1 and 2 are independent. Unit 3 needs 2. Unit 4 is partial here. Unit 5 goes with the PR that removes its packages.

Exit: wave B green; `archtest` and `docscheck` rules that name deleted packages removed in the same PR; routing is a no-op (requested model kept, hook still fails open). Not released alone (D5).

### Phase 2: routing core

Entry gate: R1 (coverage rows for `internal/routing/*`; `TestAllPackagesInventoried` fails once the directory exists without them), D1 before R2, D2 before R4.

- First, in parallel: R0 (2; candidate set as `HarnessCapabilities` data, see D13), R0b (1; read-only probe of whether PreToolUse fires for `Workflow` `agent()` spawns, result into design §11), R1 (1).
- Parallel, one package each: R2 (5; policy schema, loader, `downshift policy check`, config errors refused at load, quota TTL declared), R3 (3; `headroom`, worst window, rate-limited is +inf, TTL enforced, missing passes), R4 (2; enum hint parser), R5 (3; `guard`, takes the risk scanner).
- Last: R6 (5; `decide` and `note`, precedence of design §2, wired into `runHookAdapter` behind `DOWNSHIFT_ROUTING=v2`).

Exit: golden decision tables for every precedence step and every guard rule (risk never goes small; never widen; `explicit_only` never picked; foreign ids refused; sibling fields untouched); `archtest` allow-list for `internal/routing/...`; flag off behaves as Phase 1.

R0 fixes the #570 regression on the current v1 path, so shipping it early on `main` is an option (D6). This roadmap keeps it here, as briefed.

### Phase 3: setup and skill (Claude Code)

- R7 (3): `downshift route --harness claude-code --json` (stdin: hint and agent type; stdout: model, effort, tier, note). The brief places it here; see D9.
- R8a (3): `downshift policy apply --harness claude-code [--dry-run]` writes `model:` into `.claude/agents/*.md`, with backup and diff. On demand (D3).
- R10 (3): skill `downshift-route` in `~/.agents/skills` with a lock entry; links only `~/.claude/skills`; `setup` runs dry-run first; copy on Windows; never over a user-owned file.

Parallel: R7, R8a and R10 (different files). R10 needs R4 and R7 for the skill text and the launch command.

Exit: `policy apply --dry-run` writes nothing and prints the diff; the backup restores the original; setup links are tested under an isolated `HOME`; a symlinked target is never written.

### Phase 4: accounting and report (stage 1 exit)

- R11 (3): routing note fields in events, enums only (verdict, rule id, profile, tier from and to, headroom window and percent, harness, channel). `report` and `session --panel` show verdicts; `stats` counts from-to changes, never the verdict.
- R9 (2, Claude Code only): v2 default on Claude Code. Other harnesses stay as they are until Phase 5.
- R13 (3, Claude Code sections): harness matrix, capability matrix, README routing section, install guide, `AGENTS.md`. `AGENTS.md` has 199 lines against a 200-line cap, so trim it before adding lines.

Order: R11, then R9, then R13.

Numbers: the percent is the primary figure and comes from one source for progress and report; a fixture test asserts equality. No dollar line (O1). The report shows tokens avoided by the Compactor and tokens moved by routing per harness, in separate columns.

Exit (the README's stage 1 exit): on Claude Code, a declared profile or hint moves an Opus request to the declared tier; risk prompts never go small; with nothing declared the requested model is kept; `report`, `session --panel` and the Agents panel show the routing note; golden tables green; the fixture test passes.

### Phase 5: other harnesses (stage 2)

One PR per harness, in README order. Each PR carries its channel, its golden cases, its skill link (R10 extension) and its harness-matrix row. A harness without its channel keeps today's behaviour and says so in the matrix.

1. Kiro: R8b (3). Native `~/.kiro/agents/*.json` `model`, with backup and diff, never a symlinked target. Block and retry with the v2 decider. Launch flags come from R7. Rows say "decided and applied, not measured".
2. Codex: R8c, split. The card covers Codex and OpenCode in one 3-point line; split it, and verify each format first.
3. OpenCode: the other half of R8c.
4. Cursor: hook channel with the v2 decider. No card; size it before starting.
5. Antigravity: hook channel with the v2 decider. No card; size it before starting.

### Phase 6: later (no cards)

- Budget admission, fail-open (REA lesson 7): a pre-spawn gate using the headroom readings; a missing reading lets the spawn through.
- Probe of optional CLI flags (REA lesson 8): run `--help` once and cache it; refuse known bypass flags on the launch and apply channels.
- Hosted and enterprise evaluation: named in the brief; no source document in this repository. Scope it before any card.

Exit: an owner-accepted decision note per item. Cards come after the note.

## 6. REA lessons

| # | Lesson | Decision | Where |
|---|---|---|---|
| 1 | Worst-window headroom; rate-limited is +inf | Adopted | P2 R3 |
| 2 | Unknown reading passes; unknown config refused | Adopted; config refused at load | P2 R2, R3 |
| 3 | Ordered first-match rules with quota gates | Adopted, matched on declared hint and agent type, not on payload-derived kind | P2 R2, R6 |
| 4 | A profile is the only source of what it sets | Adopted: profile plus override is refused | P2 R2, R6 |
| 5 | Routing note with verdicts | Adopted; enums only, no task text | P2 R6; P4 R11 |
| 6 | Fallback only with room; never widen capability | Adopted as tier-down gates and R5; no separate fallback object | P2 R2, R5 |
| 7 | Budget admission control | Deferred; fail-open | P6 |
| 8 | Probe CLI flags; refuse bypass flags | Probe deferred; bypass refusal required for R7 launch flags from Phase 3 | P3 R7; P6 |
| 9 | Strategy inference and plan caps | Dropped (no batch planner) | section 7 |
| 10 | Prompt rule-sets on first turn | Dropped (changes the prompt) | section 7 |

The REA gap "UI-pushed fitness with no TTL" is fixed by the TTL in P2 R2 and R3.

## 7. Dropped

Perfumaria (no card needs them):
- Per-orchestrator role rows, personas and a named-role object. Profiles in `policy.json` cover the need.
- A "run-cheapest" label that changes nothing.
- Strategy inference and plan caps.
- Prompt rule-sets.
- Hand-maintained pricing tables. Models stay in `catalog.json`; dollars never decide a route.

Conflicts with invariants (REA, "not worth copying"):
- LLM as router with advisory numbers and notify-not-reroute. Only the bounded R4 hint remains (D2).
- Task text in routing or telemetry, and job persistence. The risk scanner reads text in memory only.
- Long-lived worker supervision: app-server, steering, approvals, watchdogs, PTY shells.
- Bypass-permission workers and trust-only read-only.
- Per-harness logic in the decider.

## 8. Owner decisions

Status of each item: all 13 accepted on 2026-10-26. D1 to D4, D6, D8 to D12 follow the recommendation written below. D5, D7 and D13 had no explicit recommendation in the first draft; the choice is the one the premises imply, and the owner can override it before the card that needs it.

- **D1 Policy file location: accepted.** Project `.downshift/policy.json` is the source; a user default lives in the state dir; the project wins. Needed before R2.
- **D2 Hint syntax: accepted.** The hint is an enum prefix, `[kind=… effort=…]`. No hint keeps the requested model; a hint can only move a request down. Needed before R4. The REA warning stands: the outcome must not depend on the LLM sending a hint.
- **D3 Native config compile: accepted.** `policy apply` runs on demand only. `setup` prints the suggestion and never writes.
- **D4 Tier classifier: accepted, delete.** The v1 tier classifier is removed with no offline suggester. Only `risk.go` survives, until R5 moves it. This supersedes the business plan's D8.
- **D5 Release gating: chosen (not in the first draft).** Phase 1 lands only on `downshift/true-north`, never on `main`. Phase 1 reaches `main` together with R6, as one release, because the hook keeps the requested model until then. This is an explicit exception to the rule that each card is mergeable alone, and it applies to Phase 1 only.
- **D6 Ship R0 early: accepted as briefed.** R0 stays in Phase 2. Shipping it earlier on `main` is possible later, but it is not part of this plan.
- **D7 Dollar line in Phase 4: superseded 2026-10-27 by O1.** No dollar line; the accounting v1 plan is removed, not un-paused.
- **D8 Price source: accepted.** Prices live in `catalog.json`. No prices in Go source.
- **D9 R7 timing: accepted as briefed.** R7 is in Phase 3 (stage 1).
- **D10 Rollback after Phase 1: accepted.** With the flag off, the requested model is kept. There is no `DOWNSHIFT_ROUTING=v1` path after Phase 1.
- **D11 Historical docs: accepted.** `docs/archive`, `docs/evidence` and dated task folders keep their old references under a banner.
- **D12 docscheck scope: accepted.** Extend `docscheck` to `README.md` and `docs/design/architecture.md` after Phase 1.
- **D13 Family aliases: chosen (not in the first draft).** `haiku`, `sonnet` and `opus` move into `catalog.json` as data, and Go code reads them from there. This follows the `AGENTS.md` rule against hardcoded model IDs in Go source, so the rule does not need amending.

## 9. Conflicts in the sources (fix when accepted)

1. README stage totals (36 and 15) do not match its cards: stage 1 cards sum to 42 points; stage 2 (R7, R8b, R8c) to 9.
2. R12a and R12b sit after R9 in the README and design §7; true-north puts them before v2. This roadmap follows true-north.
3. true-north lists all of `classify` under wave B, but R5 needs `risk.go` first.
4. The brief puts the TTL in R2; the README puts it in R3. This roadmap declares it in R2 and enforces it in R3.
5. The brief says wave A is done; the cleanup doc still lists `scoreindex` as not done, and it still exists.
6. R1 and R0b are not in the brief's phase list; placed in Phase 2.
7. R8c covers two harnesses on one card, and "one harness per PR" needs a split. Cursor and Antigravity have no card.
8. The [internal roadmap](../roadmap.md) has D-1 and D-2 (OpenRouter gateway rows, gateway removed in 8c1afdfa) and A-P5 (Cursor and Codex rewrite, which overlaps Phase 5). The owner must re-scope them.
9. The REA is outside this repository, so the lesson mapping cannot be checked from here.
10. Design §9 marks accounting "out", while the brief's Phase 4 needs a USD line (D7).

## 10. Verification

- `go test ./internal/docscheck/` in the worktree passes (run before writing and again after).
- `make check` not run (outside this card).
- Dead-reference counts come from grep, not from a gate.
- Not verified: R0b (needs a live Claude Code session); the D8 price source; whether the Agents panel exists in this tree.
