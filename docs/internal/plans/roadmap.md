<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Routing v2: phased roadmap

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: Accepted 2026-10-26 (D1 to D13); sequencing superseded 2026-10-27 by the [MVP plan](mvp-plan.md), which wins where they differ. Scope: model routing and its channels. The Compactor (`execwrap`, `compressor`, `shellwrap`) and DCI (`codeintel`) do not change.

Sources: [routing-v2 design](../../design/routing-v2.md), [cards](../tasks/2026-10-26-routing-v2/README.md), [true-north cleanup](../tasks/2026-10-26-routing-v2/true-north-cleanup.md). The external-orchestrator behaviour report is kept outside this repository and is not reproduced here.

Phases end at exit criteria, not dates. Where two sources disagree, section 9 names the conflict and this roadmap states which side it follows.

## 1. Premises (2026-10-26)

- **P1 Clean-room base.** The external-orchestrator behaviour report is the only input from that project. No source, prompt or schema from it is used (AGPL-3.0).
- **P2 Tokens only.** Savings are tokens per harness; no dollar account (owner decision O1, 2026-10-27, in the [MVP plan](mvp-plan.md)).
- **P3 Declared setup.** Declared configuration, validated at load, applied by an explicit step that runs a dry run and a diff, and makes a backup first. This is my reading of the brief's wording.
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

## 3a. Current state (2026-10-29, latest; source of truth for routing phases)

Sections 5 below are the 2026-10-26 plan, kept for history. Where they conflict with this block or with 8a to 8d, the later text wins. For the MVP stages (setup, MCP, report, plan runner), the [MVP closing state (2026-10-29)](../tasks/2026-10-27-mvp/README.md#closing-state-2026-10-29) wins over this table.

| Phase | State | What is left |
|---|---|---|
| 1 Clean base | Done on the branch (N1 to N3) | NB files, `internal/nbtier`, gateway files, docs citing `try` (W7 cleanup) |
| 2 Routing core | Done on the branch: R1 to R6, R9 | R0 unverified; R0b and real-session proof (C0) not run |
| 3 Setup and skill | Partly: MVP setup cards S1 to S3 done (2026-10-28); R7 and R10 not started; R8a not started | R7, R8a, R10; W7.1 to W7.3 |
| 4 Accounting and report | Partly: R9 done | R11 (routing note), R13 (docs), V1 to V3, control arm (W6.1, W7.7) |
| 5 Other harnesses | Not started | Per-harness defaults (W7.10); effort (W7.11) |
| 6 Later | No cards | Budget admission, flag probe, hosted evaluation |

Plan runner (MVP stage 4): specification done with requirements R1 to R62. Partly built through the MCP worker tools (queue, read-only launches, bypass write route in a worktree); the plan file runner is not started. Open decisions: G1, G4, G10, G12 (see the spec's gap table).

Release gate (D5): nothing reaches `main` until the real-session proof, AppSec (W7.4), remote CI (W7.5) and the migration note (W7.6) are done.

Approach note (2026-10-29): Universal Orchestrator, MCP conformance, and non-bespoke testing strategy are defined in [universal-orchestrator-harness-approach.md](universal-orchestrator-harness-approach.md). Standard conformance replaces custom synthetic test batteries with the official MCP Inspector, architecture fitness functions (`internal/archtest`), golden files for compaction, and SWE-bench-style external verification gates.

Gateway Routers note (2026-10-29): Multi-provider proxy routers (9router and OmniRoute) operate at the transport layer (failover, multi-key load balancing across Anthropic, DeepSeek, OpenAI, Groq), while Downshift operates at the orchestration and compaction layer. Downshift connects with them as first-class companions: detecting them in `doctor`, normalising gateway model prefixes (e.g. `cc/*`, `auto/*`), and wiring harness base URLs without bloating Downshift with a bespoke GUI or embedded proxy.

## 3b. Setup Evolution, Universal Orchestrator & Native Markdown Skills (2026-10-29)

Following clean-room competitive analysis of external orchestrator tools (such as Alethe, commit `17acb090` / research `2026-10-28`), Downshift incorporates the highest-value onboarding and orchestration paradigms while **strictly avoiding the heavy monolithic anti-patterns** (400MB Electron app, PTY scrapers, credential interception, local HTTP shared-secret servers, AGPL licensing):

1. **Native Markdown Skills over Bespoke Runtimes:**
   Rather than building an embedded multi-process orchestration runtime in Go, Downshift treats the user's native AI harness (Claude Code, Cursor, OpenCode, OpenAI Codex, Google Antigravity, Kiro, KiroCrew — ordered by community adoption) as the orchestrator. Downshift installs portable, zero-overhead `.md` skills directly into the harness configuration directories (`~/.claude/skills/`, `~/.cursor/rules/`, `~/.config/opencode/skills/`, `~/.kiro/steering/`, etc.):
   - `downshift-orchestrator.md`: Teaches the Lead LLM to plan, decompose, respect worker concurrency caps, and query Downshift's MCP tools (`providers`, `status`, `apply`).
   - `downshift-delegate.md`: Enforces structured Task Envelopes (one-sentence goal, bounded file paths, verification gate, `[tier=small]` hints).
   - `downshift-rules.md`: Inviolable engineering guardrails (never commit without test passing, strict scope boundaries, structured handoff).

2. **Pre-installed Engineering Playbooks as Native Markdown:**
   Modular, opt-in engineering standards delivered as plain Markdown rules:
   - `research.md`: Strict read-only exploration and symbol mapping with exact line citations.
   - `implementation.md`: Clean code, SRP, KISS, files under 300 lines, no premature abstractions.
   - `review.md`: AppSec and architectural fitness reviews (OWASP ASVS standards) without editing source.
   - `tdd-verification.md`: Test-driven discipline, wrapping commands with `downshift compactor -- <test-cmd>`.
   - `docs.md`: Technical documentation hygiene and author privacy preservation.
   - `incident-rescue.md`: Emergency debugging, root-cause isolation, and surgical bugfixes.

3. **The Universal Skill (Strategies & Token Hygiene):**
   A dedicated `downshift` skill managing execution modes (`economy`, `balanced`, `parallel`, `research`, `rescue`) and strict token hygiene (passing bounded file subsets instead of full transcripts, reading diffs instead of command scrollback).

3. **First-Run Interview in `downshift setup`:**
   Interactive terminal wizard asking three strategic questions on first run:
   - *Routing & Cost Posture:* Balanced (Recommended), Aggressive Economy, or Conservative.
   - *Worker Concurrency Cap:* Maximum simultaneous background subagents (1 to 8, default 4).
   - *Default Time Budget:* Maximum duration per subagent task (default 900s).

4. **Live Adapter Proof & Smoke Verification in Onboarding:**
   Real-time handshake with installed harness CLIs during setup (firing in-process test payload in < 50ms) to verify permissions and paths before concluding.

5. **Closing "Aha! Moment":**
   Clean terminal summary displaying detected provider quotas side by side, active routing tiers, and the immediate command to verify in the user's preferred harness.

### Setup Evolution Waves (Cards & Points)

| Card | Scope | Points | Deliverable |
|---|---|---|---|
| **S-W1** | **Interactive TTY Detection & Silent Fallback** | 2 | `downshift setup` auto-detects interactive terminal vs script/pipe (`--yes`, `--dry-run` unaffected) |
| **S-W2** | **First-Run Interview State Machine** | 3 | Lipgloss-driven wizard collecting policy preset, concurrency cap, and companion gateways |
| **S-W3** | **Native Markdown Skills & Playbooks Generator** | 3 | Automated generation and linking of `.md` skills and engineering playbooks into target harness folders |
| **S-W4** | **Live Adapter Proof & Smoke Check** | 3 | Real-time payload handshake during setup with latency measurement (< 50ms) and instant fix hints |
| **S-W5** | **Aha! Moment Summary & Verification Gate** | 2 | Final visual status card, `doctor` parity, updated `install.md`, `README.md`, and `make check` pass |

Total: 5 cards, 13 Story Points. Fully modular and incremental (each ≤ 3 SP).

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

Exit (the README's stage 1 exit): on Claude Code, a declared profile or hint moves an Opus request to the declared tier; risk prompts are not floored by the binary (O8, 2026-10-28; a project may declare a floor); with nothing declared the requested model is kept; `report`, `session --panel` and the Agents panel show the routing note; golden tables green; the fixture test passes.

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

## 8a. Decision notes (2026-10-27)

The owner changed the route for Phases 1 and 2 on 2026-10-27. These notes amend the items above. They do not reopen the 2026-10-26 acceptances.

- **N1 No flag. v2 replaces v1 in one change.** `DOWNSHIFT_ROUTING` and the keep default are dropped. Phase 1 wave B and R6 land together: the v2 decider becomes the only decider in the same change that removes the v1 decider. This supersedes the flag wording in R6, the no-op exit of Phase 1, and D10: with no flag there is no flag-off path, and rollback is a revert.
- **N2 Routing is never off on the branch.** A consequence of N1: the no-op state of Phase 1 does not exist. The release gate holds: nothing reaches `main` before R6 and R13 are proven (D5).
- **N3 Kept: `internal/session`, `classify`, `fuse`, `semantic`.** Outcome and benchmark use `ClassifyWithSemantic`, and session selection still uses `internal/session`. Phase 1 units 3 and 4 are partial: only the v1 parts go. The rest is a later card.
- **N4 R9 moves into the R6 change.** Making v2 the default is the same change as the decider (N1), so R9 is done in Phase 2. R11 and R13 stay in Phase 4 and must be done before the release.
- **N5 R0b before R6.** R0b (does PreToolUse fire on workflow `agent()` spawns) is still unverified in the design, §11. It runs before R6. The real-session proof, authorized by the owner on 2026-10-27, is its evidence.
- **N6 One branch, no PR per card.** The cards land as commits on `downshift/mvp-ponto-zero`, gated by `make check` on each commit. The branch named in D5 is now this one. The one-PR-per-part rule does not apply to this line.
- **N7 Real-session proof before default.** R6 does not become the default until the acceptance suite (V2) shows no regression and a real Claude Code session shows the decision. Owner authorized the session on 2026-10-27.

**Status (updated 2026-10-27, closing; branch `downshift/mvp-ponto-zero`, all work published; code QA PASS at `2330a3e1`, later commits are docs and specification).**
- **Phase 1:** done on the branch with the decisions N1 to N3. Cleanup still open: the NB files, `internal/nbtier`, the gateway files and the docs that still cite `try`.
- **Phase 2:** R1, R2 and R3 are done. R4 (hint) and R5 (risk guard) are done. R6 (the v2 decider wired to the hook, replacing v1) is done on the branch. R0 is unverified against the 2026-10-26 card. R0b is not run. The real-session proof is pending.
- **Phase 3:** R7, R8a and R10 are not started.
- **Phase 4:** R9 is done, as decision N4. R11 and R13 are not started.
- **Phases 5 and 6:** not started.
- **Release:** nothing is on `main`. It waits for the real-session proof (D5).

### 8b. Assessment (2026-10-27)

**Grade: 6 of 10.** The grade is for the product, not the effort. The branch is `downshift/mvp-ponto-zero`, QA PASS at `2330a3e1`.

**Delivered and verified**
- Clean base (MVP stage 0): v1 experiments, gateway, orchestration and `scoreindex` removed; telemetry no longer imports `core`; routing contract, policy and headroom in place.
- No dollar in code (O1): the cost formula and the dollar event fields are gone; reports and dashboard are in tokens only; the hook note carries no percentage. The catalog price table stays as an internal routing signal.
- Routing v2 (MVP stage 2): hint reader, risk guard, Claude statusline quota, tier floor and the v2 decider wired to the hook. The v1 decider is removed with no flag and no intermediate state. `make check` is green and the adapter smoke passes.
- Process: one branch, no open PR, harness-core agents installed and called by name, model table and open-PR cap in harness-core and the global rules.
- Plan runner specification (MVP stage 4 input): 54 requirements, written clean-room from behaviour notes. No upstream code or text is used (see N8).

**Gaps, in order of impact**
1. **No real-session proof.** Nobody has seen the v2 decide in a real Claude Code session. Everything is proven by tests and smoke only. This also closes R0b.
2. **The hint may not reach the decider.** The adapter passes the prompt before the description, and the hint lives in the description.
3. **Profile and pinned model do not fire through the hook.** The request carries neither field, so precedence steps 1 and 2 are inert.
4. **`explicit_only` preservation test removed.** The v2 protects the target, not the requested model. Needs an owner decision.
5. **The all-critical warning has no field of its own.** It is a `kept` verdict with the quota marked ignored, which is hard to tell apart in a report. Fixing it changes the contract.
6. **Acceptance suite (V2) has not run** against the requested-model arm, so "no regression" is not proven.
7. **Cleanup pending.** NB files, `internal/nbtier`, gateway files, and about 40 docs that cite the removed `try` command.
8. **`main` still holds v1.** Nothing from the branch may reach it before gap 1 is closed (D5).

**Stages not started:** setup (stage 1, R7/R8a/R10), report and prove (stage 3, R11/R13 and V1/V3), plan runner (stage 4), other harnesses (stage 5). About 17 points of the 39-point MVP remain; the plan runner spec sums to 33 points against 26 in the MVP plan.

**Next:** the real Claude Code session (authorized by the owner on 2026-10-27), before stage 1 and the plan runner.

### 8c. Clean-room note (N8)

The upstream project studied for the plan runner is AGPL-3.0-or-later; its copyright belongs to its author and contributors. A local MIT relicense commit exists only in a local clone, is unpublished, and grants nothing. The `downshift-plan` copy was deleted on 2026-10-27. Downshift uses behaviour notes and its own specification only. No upstream file, function, comment or string is copied.

### 8c-quater. Default profile table (N11, 2026-10-27) — removed 2026-10-28 (O8)

**Superseded.** The owner decided on 2026-10-28 (O8) that the orchestrator LLM chooses the model and Downshift only applies it: the table below no longer exists in the binary, and its defaults are guidance in the `downshift-orchestrator` playbook skill. Kept here as the record of what was approved.

Approved by the owner. Tiers follow the harness-core model table: small is Haiku, mid is Sonnet, frontier is Opus on Claude Code; Fable is never picked automatically.

| Harness | Agent type | Default tier | With a risk signal |
|---|---|---|---|
| Claude Code | `Explore` | small | mid |
| Claude Code | `claude-code-guide` | small | mid |
| Claude Code | `statusline-setup` | small | mid |
| Claude Code | `general-purpose` | keep requested | keep requested |
| Claude Code | `Plan` | keep requested | keep requested |

The owner's own agents (harness-core roster and `mechanical-worker`) go in the owner's `policy.json`, not in the binary. Economy mode (W7.9) lowers each row one tier within the same bounds.

### 8c-quinquies. Harness-agnostic by construction (N12, 2026-10-27)

Owner direction: every feature is harness-agnostic, so the next harness costs data, not code. This restates the layering rule of `AGENTS.md` for the roadmap.
- **Tiers are abstract** (small, mid, frontier). Each harness maps them to its own model names in `catalog.json`.
- **Harness facts are data** in `HarnessCapabilities`: whether the harness accepts only its own model names (`StrictModelName`), whether effort can be applied (`CanApplyEffort`), which environment variable can override the hook (`ModelOverrideEnv`). Shared code never branches on a harness name.
- **Defaults, economy mode and the plan runner** are the same on every harness. A harness that cannot apply a decision still computes it, and the harness matrix says so.

**Claude Code facts from the official subagent docs (read 2026-10-27).** These are Claude Code only; other harnesses use their own formats.
- The Agent tool `model` parameter accepts only the aliases `sonnet`, `opus`, `haiku` and `fable`, not a full model id. The catalog already writes the `native_name` alias (`StrictModelName`).
- Precedence: the per-invocation parameter (which a mod's `agent.spawn` hook can replace), then the subagent `model` frontmatter, then `CLAUDE_CODE_SUBAGENT_MODEL`, then the main conversation's model. `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` overrides the first two; the adapter already knows it.
- Built-in `Explore` and `Plan` keep their listed model when only the default variable is set.
- Effort per subagent: `effort` frontmatter, and an Agent tool `effort` parameter since v2.1.292. `CLAUDE_CODE_EFFORT_LEVEL` wins over both. `ClaudeCodeCaps` still says `CanApplyEffort: false`, which is stale for that version.

### 8c-ter. Business plan divergences resolved (N10, 2026-10-27)

Owner decision: follow the business plan where it diverges from the plan runner spec.
- **Concurrency (G14):** hard cap 16 workers, default 4.
- **Worktrees (G16):** under `.downshift-worktrees/`, ephemeral branch for writes, detached for reads.
- **Merge gate (G15):** an explicit, fail-closed command (`plan merge`, R61). Never automatic (R62).
The plan runner estimate is now 41 points, with two new fronts (P11 events, P12 merge gate).

### 8c-bis. Dashboard decision (N9)

**N9 Dashboard discontinued (owner, 2026-10-27); its commands removed (owner confirmed, 2026-10-29).** The web dashboard (`dashboard/` and the embedded assets under `internal/api/dashboard`) gets no new work. It is not extended for workers, quota or diffs. Whether to remove it is a separate cleanup card (W6.6). The visual surface for multi-worker supervision, when it comes, is the terminal (see BP1), not a browser.

### 8d. Business plan alignment (2026-10-27)

**Source of truth.** The `downshift-business-plan` repository (MIT, commit `e4a3b91`) is the strategic, product and architecture reference. Read for this section: `README.md`, `roadmap/execution-waves.md`, `roadmap/evaluation-and-gates.md`, `product/capability-matrix.md` and the five files in `architecture/`. Not yet read: `business/*`, `product/multi-harness-governance.md` and `product/tui-command-center-spec.md`; they are mapped by title only. The plan rests on the public Orchestrator-Workers pattern. Nothing here copies third-party code.

**Priority order from the plan.** Close the Beta-exit blockers first (wave 0), then orchestration (waves 2 to 5). Our v2 routing (MVP stage 2) is the base for both. The real-session proof stays first in our queue, because it feeds wave 5 and gates any release (D5).

**Cards, each at most 5 story points**

| Wave | Card | Pts | What it delivers | State |
|---|---|--:|---|---|
| W0 Beta exit | W0.1 | 2 | P1.6: capture which model a Cursor child ran on a paid usage-based plan | Blocked: needs a paid plan |
| W0 | W0.2 | 1 | P2.3: publish the export template for external operators | Blocked: needs an operator |
| W0 | W0.3 | 3 | P3.4: raise usage-linked events above 5 of 50, by binding more harness transcripts | Open |
| W0 | W0.4 | 3 | P3.7: billing-period comparison template and check | Open |
| W0 | W0.5 | 5 | P4: grow the blind set from 37 toward 200 tasks, next slice only | Open |
| W1 Foundations | W1.1 | 3 | This alignment and the validator link to the plan | Done with this section |
| W2 Worktree runner | W2.1 | 5 | `internal/worktree`: provision, slug grammar, lease file, read-only and isolated-write modes, hermetic git, symlink-escape check | Not started |
| W2 | W2.2 | 5 | Merge gate: numstat, three-way dry run, verify under `downshift compact`, compare-and-swap | Not started, see BP3 |
| W2 | W2.3 | 3 | `downshift worktree gc` crash recovery and the git version check in `doctor` | Not started |
| W3 Quota shield | W3.1 | 5 | Two windows (5 hours and weekly) with thresholds as policy data | Partly done: Claude readings and headroom exist |
| W3 | W3.2 | 5 | Cross-vendor spillover in the plan runner's provider choice | Not started |
| W3 | W3.3 | 3 | Warning and queue behaviour at the hard ceiling, shown in `quota status` | Not started |
| W4 TUI | W4.1 to W4.3 | 5, 5, 3 | `downshift watch` and `downshift plan` terminal views | Blocked: see BP1 and BP2 |
| W5 Conformance | W5.1 | 5 | Cross-harness conformance using recorded real-session fixtures (our record and replay design) | Not started |
| W5 | W5.2 | 3 | Hook latency under load, inside the 1.5 s budget | Not started |

Wave totals: W0 14, W2 13, W3 13, W4 13, W5 8. The plan-runner spec (33 points) overlaps W2 and W3; it is reconciled in BP3 to BP5.

**State on 2026-10-28 (MVP stage 2b and 3).** Built: the MCP server with four information tools and `setup --mcp` (K3 to K6), detected models only (K1, K7: no id is fixed in code, `setup` no longer writes `session-models.json`, the `providers` tool can store reported ids as a one-hour hint that a fresh detection overrides), and the routing note in `report` and `session --panel` with tokens per harness in two columns (V1). Pending the owner's acceptance. Next: V2 (acceptance baseline, needs the owner's budget approval) and V3 (real-session proof). Partly met on 2026-10-28: two real Claude Code spawns carrying `[tier=small]` in the prompt were moved to haiku (routing note `hint_down`), and the MCP server answered a real session's `providers` call. Open: the haiku alias drift (OR1), the release note, and a proof with the control group off. The setup registers the MCP server by default now (`--no-mcp` skips it). Cards: `docs/internal/tasks/2026-10-27-mvp/README.md`.

**Open reconciliations (owner decisions)**

- **BP1 Surfaces (accepted by the owner, 2026-10-27).** The product starts on Claude Code and becomes harness-agnostic later, so the contract is neutral from the first card. Order: (1) the plan runner engine with JSON commands as the contract (`plan status`, `worktree apply|rework|discard`, `route --json`); (2) a Claude Code skin on top (dock and statusline) that holds no policy; (3) an MCP stdio server as a thin neutral channel over the same engine, after the engine exists; (4) the TUI as a neutral cockpit only once a second harness runs workers. The MCP server is a tool the orchestrator calls, so it fits O7: Claude Code stays the orchestrator. Wave 4 is deferred to step 4.
- **BP2 TUI dependency.** Deferred with wave 4. A TUI library is an external Go dependency and must stay off the hook path. Decide it when step 4 starts.
- **BP3 Merge gate.** The plan fast-forwards the operator's branch after verification. The plan-runner spec never applies work back automatically. Pick one.
- **BP4 Worktree layout.** The plan puts worktrees under the repository with an ephemeral branch. The plan-runner spec puts them in the state directory, detached, with no branch.
- **BP5 Capability matrix is wrong today.** It marks v2 routing as in production. v2 is on the branch only, not on `main`. The fix belongs in the business plan repository.
- **BP6 Thresholds.** The plan uses 70, 85 and 95 percent. Our default critical threshold is 80. Make all of them policy data.
- **BP7 Stale model names.** The plan names old models in examples. Models stay in `catalog.json`, never in code or specs.
- **BP8 Package names.** The plan names `internal/orchestration` and `internal/worktree`. The plan-runner spec uses its own package names. Align them before the first card starts.
- **BP9 Dollar in the TUI sketch.** The plan's TUI mock shows an estimated saving in USD. O1 forbids it. The header shows tokens only.
- **BP10 Compaction claim.** The capability matrix says compaction cuts up to about 50 percent. The measured figure is 82.3 percent fewer bytes on clean outputs and 23.7 percent overall. Use the measured figures.
- **OR1 Newest haiku as the small target (decided by the owner, 2026-10-28).** The native `haiku` alias runs the newest haiku (claude-haiku-5-5), so the catalog's small target is now claude-haiku-5-5, written by its native name; claude-haiku-4-5 keeps its exact id and price and is not written by name. Cheap, small and standard tiers always follow the newest model the harness offers; the target comes from detection, and the catalog only gives names and prices. Open: the README savings figure (75 percent) must be recomputed (the catalog gives 97.5 percent at list price); quality of the newest haiku against the baseline before it is the default (the quality rule); and the telemetry should record the resolved model from the child record, not the requested id.

**More gaps found (2026-10-27)**
- **No remote CI evidence for the branch.** `gh run list` shows no run for `downshift/mvp-ponto-zero`. The `test-os` job (Windows and macOS) has never run on the new code, and `internal/worktree` will need build tags for process-group calls.
- **Docs still describe v1.** `docs/contrib/layering.md`, `docs/design/architecture.md` and `docs/contrib/coverage.md` still mention NB hooks, `nbtier`, the score index and `RouteAgent`. About 40 other docs cite the removed `try` command.
- **Hook latency of v2 is not measured.** The decider reads the policy file and the quota snapshot per spawn, inside the 1.5 s budget. Card W5.2 covers it.
- **Real-session proof and R0b** are still open (see 8b).
- **Plan-runner spec decisions** are open: the acceptance criterion (G1), the estimate of 33 points against 26 (G13), the harness contract table (G10), default tiers (G12) and the CLI flags (G4).
- **Product decisions** are open: preserving an explicit-only requested model, and a field for the all-critical warning.
- **Business plan files not read yet:** `business/*`, `product/multi-harness-governance.md`, `product/tui-command-center-spec.md` was read; the first two sets were not.
- **Cleanup:** NB files, `internal/nbtier`, gateway files.

**Absorbed design cards (W6), each at most 5 points**

| Card | Pts | What it delivers | Depends on |
|---|--:|---|---|
| W6.1 | 5 | Measurement frame in tokens: confirm the 10 percent control arm survived the v1 removal, then a paired routed-versus-control report with the guardrails (`frontier-on-doubt` is zero, rework rate) | Real-session proof, V2 |
| W6.2 | 3 | Zero-prompt rework correlation: task id, salted tool-use hash and salted scope hash, and the `reworked` flag | W6.1 |
| W6.3 | 3 | Event schema aligned with the plan table (`route.decided`, `worker.lifecycle`, `worktree.gated`, `quota.observed`), allowed fields only | None |
| W6.4 | 3 | Plan runner admission: 4 workers by default (cap 16), 2 per harness, defer on cool-down, in-memory output ring buffer of 256 KiB | Plan runner engine |
| W6.5 | 3 | Cross-platform cancel: SIGINT, SIGTERM, SIGKILL on the process group, and a job object on Windows, in per-OS files | Plan runner engine |
| W6.6 | 3 | Dashboard code removal (N9): `dashboard/`, embedded assets, `internal/api` and the docs. The commands were removed 2026-10-29 | Owner decision |

W6.3 can start now. W6.4 and W6.5 are pieces of the plan-runner fronts and are merged with them, not built apart. All W6 cards use tokens only (O1).

**Release readiness cards (W7), each at most 5 points**

These close gaps that no stage covered. W7.1 and W7.2 come first: without them v2 routes nothing on a fresh install, because with nothing declared the requested model is kept.

| Card | Pts | What it delivers | Depends on | State |
|---|--:|---|---|---|
| W7.1 | 3 | A default policy shipped with the binary, keyed by harness and agent type, giving an abstract tier (small, mid, frontier); each harness maps the tier to its own models through `catalog.json`. Risk guard unchanged. Agent-type match ignores case and accepts a plugin name with or without its prefix. Approved by the owner on 2026-10-27 (table N11) | Real-session proof | Removed 2026-10-28 (O8): defaults moved to the orchestrator playbook |
| W7.2 | 2 | `downshift policy init` in `setup`: dry run first, writes `.downshift/policy.json` only after a yes | W7.1 | Done (`50f80a0a`) |
| W7.3 | 3 | Hint skill (R10) moved earlier: the parent writes the hint the decider reads | Real-session proof | Open |
| W7.4 | 3 | AppSec review of the hook-path change (decider cut) before anything reaches `main` | None | Self-review draft (`docs/internal/appsec-review-w7-4.md`), independent review pending |
| W7.5 | 2 | Remote CI on the branch, including the Windows and macOS job | None | Verified locally (`make check` cross-vet); remote PR CI pending |
| W7.6 | 2 | Release note and migration: version bump, `try` removed, NB variables ignored, new default behaviour, rollback by revert | W7.1 | Draft (`docs/releases/unreleased-migration-notes.md`) |
| W7.7 | 3 | Dogfooding protocol: duration, sessions, opt-in control arm, fields measured in tokens, stop criteria from the business plan | Real-session proof | Open |
| W7.8 | 1 | Estimate against actual for stages 0 and 2, as the harness-core rule asks at closing | None | Partial (`docs/internal/estimate-vs-actual-w7-8.md`: estimates mapped, actual effort tracking pending) |
| W7.9 | 3 | Economy mode as policy data (`"mode": "economy"` in `policy.json`, toggled by a command), the same on every harness: every default goes one tier down; a type that keeps the requested model goes one tier below it. Bounds stay: risk never goes small, `explicit_only` never picked, never above the requested model | W7.1 | Superseded by O8: acts only on declared policy; removal pending |
| W7.11 | 2 | Claude Code effort: confirm the Agent tool `effort` parameter on the installed version (v2.1.292 or later), then set `CanApplyEffort` from data for that version; `doctor` warns when `CLAUDE_CODE_EFFORT_LEVEL` or `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` would override the hook | Real-session proof | Open |
| W7.10 | 2 | Per-harness defaults beyond Claude Code: read the agent types each harness really sends (OpenCode first), add its rows to the default table; until then its types keep the requested model while economy mode still applies | W7.1, that harness's real payload | Superseded by O8: per-harness defaults go to the playbook |

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
