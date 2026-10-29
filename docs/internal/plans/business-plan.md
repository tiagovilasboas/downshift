<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Business plan: routing v2 and the open-core model

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **Proposal, 2026-10-26; updated 2026-10-27 and 2026-10-28.** Execution order and the owner decisions (O1 to O8) are in the [MVP plan](mvp-plan.md). O8 (2026-10-28) changes who decides the model: the orchestrator LLM chooses and Downshift applies and measures; the sections below are updated to match. Remaining open items are in section 9.

Scope: the product on `main` and the routing v2 plan in [routing-v2](../../design/routing-v2.md). The Compactor and DCI are out of scope, as they are there. This plan contains no market size, price, user count or revenue figure. None is in the repository, and none is invented here.

## Evidence levels

| Level | Meaning |
|---|---|
| **E1** | Aggregate telemetry from one operator's machine. Source: diagnosis of `origin/main` 800a8e4e (2026-10-26). The raw log and the diagnosis file are not in this repository. |
| **E2** | Synthetic reproduction (the diagnosis's 9 of 9 held spawns). |
| **E3** | Controlled A/B on forced-delegation tasks, in the repository: [four-arm battery](../../evidence/four-arm-mechanical-2026-10-21.md). One operator, one machine. |
| **E4** | Offline labelled sets from a frozen eval in the private `downshift-labs` repository. Not in this repository. |
| **E5** | Owner premise or design reasoning. Not measured. |

## 1. Problem and who has it

- **Subagents inherit the parent model.** [harness-matrix](../../reference/harness-matrix.md) states that an agent type with no `model:` inherits the session model. A spawn therefore runs on the parent's model unless the parent passed a smaller one. In E1 data the parent passed a per-call model on 154 of 158 Claude Code spawns: haiku on 39%, sonnet on 37%, Opus on 20% (E1).
- **Subscription users.** Claude Code runs real subagents on the paid plans (Pro, Max, Teams, API); Free does not (harness-matrix). The 5 h and 7 d limit windows come from the statusline `rate_limits`, which is not yet observed live (GAP-CLAUDE-QUOTA in [capability-matrix](../../reference/capability-matrix.md)).
- **"Hit weekly limits".** This is the owner's premise (E5). The repository holds no measurement of how often users reach a limit.
- **Teams on several harnesses.** Also a premise (E5). Today only Claude Code is "Completed" in the harness matrix; codex and cursor are Functional, kirocrew and antigravity Experimental, opencode Beta, grok config only.

The usable lever is narrow: routing can only lower a request that asks for Opus. That is about one spawn in five (E1), so the value ceiling is set by how often that case occurs.

## 2. Owner premises (2026-10-26)

- **P1 Clean room.** The clean-room behavioural analysis of an external AGPL-3.0 orchestrator (kept outside the repository) is the learning base. No code, prompt or schema from it is copied.
- **P2 Tokens, never dollars.** Savings are reported as tokens per harness: tokens the Compactor avoided, tokens routing moved to a smaller model, and quota used. No dollar figure is computed or shown (owner decision O1, 2026-10-27). Routing reads quota, never cost.
- **P3 Declared setup.** A declared policy (profiles, ordered rules, quota gates, thresholds) written by one setup command; a deterministic decision in code; a routing note on every decision.
- **P5 The LLM chooses (O8, 2026-10-28).** The orchestrator LLM chooses agent, provider, model and effort for each delegation, as in Anthropic's Orchestrator-Workers pattern. Downshift applies the choice within the user's declared policy and measures it. No built-in tier table, no automatic upgrade, no built-in risk floor.
- **P4 Keep only what serves this product.** Drop the Tauri UI, personas, prompt rule-sets, pricing UI, long-lived orchestrator shells, steering and approvals. A batch plan runner with one-shot workers is in scope (A1, stage 4 of the MVP plan).

P1 and P3 are the header and section 2 of [routing-v2](../../design/routing-v2.md) and the [routing-v2 task README](../tasks/2026-10-26-routing-v2/README.md). P2 is routing-v2 section 9. P4 is only partly reflected: the README's out-of-scope list keeps worker supervision and experiments out, but names none of the dropped features, so P4 is not yet written in any repository doc.

## 3. Value proposition and evidence

**Promise (O8):** Downshift applies the model the orchestrator chose, inside the policy the user declared, records why in a routing note, and reports the tokens that moved. It no longer keeps a risk prompt off the small tier by itself: that judgment belongs to the orchestrator, guided by the playbook, and a project can declare a tier floor in `policy.json`.

| Claim | Figure | Level | Source |
|---|---|---|---|
| Child tokens moved to a smaller model, real use | 4.1% (23.9M of 579.1M) | E1 | routing-v2 section 1, from the E1 diagnosis. The window label needs a fix (section 9, D1). |
| Claude Code spawns that changed model | 8 of 158 (5.1%), all from Opus | E1 | same |
| Claude Code rewrites for a subscription user after #570 | 0 in the log; 9 of 9 synthetic spawns held | E1 and E2 | same; #570 removed the session list |
| Rewrite rows that were no-ops | 22 of 30 (73%), from binaries before #517 | E1 | diagnosis section 2(e) |
| Decisions corrected by a guard | 67 of 200 (33.5%) | E1 | diagnosis section 2(c) |
| Tokens per accepted run, routed vs opusplan, mechanical tasks | 24.1% fewer (interval +2.1% to +46.7%); accepted 12 of 12 vs 10 of 11 | E3 | [four-arm battery](../../evidence/four-arm-mechanical-2026-10-21.md), Results |
| Classifier tier accuracy on three blind sets | 42% to 53%; abstention 62% to 77% | E4 | diagnosis section 2(b) |

Read each claim at its level. The only token-saving result is E3, and the file itself calls it "the ceiling case for routing, not a typical session". The 4.1% is the baseline stage 1 must beat, not a promise. v2 claims no saving until it runs ([routing-v2 section 10](../../design/routing-v2.md)).

## 4. Business model

Apache-2.0 open core. [MONETIZATION.md](../../../MONETIZATION.md) lists four revenue models, with no prices:

- **Hosted.** A possible managed offering (dashboards, org policies). Not built, not announced.
- **Support.** Paid integration help for enterprises.
- **Enterprise.** SSO, audit retention, SLA, private benchmarks.
- **Trademark.** "Downshift" as the brand for official binaries and cloud. Apache-2.0 grants no endorsement right.

The free layer is the distribution: the binary, the harness integrations and the compatibility matrix. The maintainer competes on release velocity, official releases, harness compatibility and hosted convenience. No revenue has been earned or forecast.

`docs/internal/` is public in git by convention ([publishing-boundaries](../../reference/publishing-boundaries.md)). SKU and price detail belongs in the private repository, not here.

## 5. Competition

**External desktop orchestrator** (AGPL-3.0, Tauri app), from the clean-room analysis:

- On its MCP path, the planner LLM (a Claude Code or Codex session) picks agent, model and effort by default. Deterministic rules (roles, rules with quota gates) fire only when the planner names a role or sends a kind. Downshift now has the same shape (O8): the orchestrator chooses, and declared rules apply only to what it names.
- Workers on the MCP path are Claude and Codex. The skill path also runs Kimi, Antigravity and custom CLIs.
- Vendor usage readings are polled by the UI every five minutes, partly from other tools' credential files. The default is advisory ("notify, do not reroute"). Opt-in auto-switch is still roadmap. Downshift reads native snapshots only and acts on them: simple spawns go down near a limit, and the plan runner picks the provider with headroom.
- Personas and Kiro are not built.

**Downshift's position:**

- **The orchestrator chooses (O8).** The choice is the orchestrator LLM's. Downshift's code applies it deterministically and runs no LLM of its own.
- **Harness-agnostic.** One decider in shared layers. Adapters only parse the payload and declare capabilities (the layering rule in AGENTS.md). Seven harness entries; Claude Code is the only one "Completed".
- **Fail-open.** 1 MiB input cap, 2 s read, 1.5 s deadline; a panic leaves the spawn unchanged.
- **Prompt-free telemetry.** Events hold enums, tiers and token counts, never task text or paths.

That product is broader: a desktop orchestrator with worktrees, approvals and long-lived workers. Downshift should not compete on that.

**Risk: harness vendors ship native routing.** Claude Code already accepts a per-call `model` ([harness-matrix](../../reference/harness-matrix.md)) and `CLAUDE_CODE_SUBAGENT_MODEL` ([capability-matrix](../../reference/capability-matrix.md)). If vendors default children to small models, the headroom for the decision layer shrinks. The mitigation is a declared policy that stays portable across harnesses, not a design built around one vendor's defaults.

## 6. Costs and constraints

- **One maintainer.** [GOVERNANCE.md](../../../GOVERNANCE.md) sets a solo-maintainer rule: no self-approval. Bus factor is one.
- **CI.** `.github/workflows/ci.yml` has `test` (Linux, the required gate), `test-os` (windows-latest and macos-latest; runs when OS-sensitive paths change or on tags), `race`, `python`, `build-cross-platform`, `release` and `verify-release`. `e2e` is informational, not required. `sync-prices.yml` runs weekly and opens a PR for the price catalog; Cursor rows are entered by hand ([prices](../../reference/prices.md)).
- **Gate per routing card.** `make check`, `./examples/run-all.sh`, golden decision tables, copyright headers, no file over 300 lines, tests that isolate `HOME` and `DOWNSHIFT_STATE_DIR` (routing-v2 README).
- **Clean-room cost.** P1 limits how routing v2 is written. The analysis sits in a scratch location, which is not a durable home (D6).
- **Harness drift cost.** Each harness has its own hook payload and version. One upstream change (#570) removed every Claude Code rewrite for subscription users.
- **Runtime cost.** Downshift makes no LLM call. The orchestrator's choice costs what the orchestrator already spends; the hint is a short enum prefix.
- **Not measured:** maintainer hours and infrastructure spend. No figure is recorded here.

## 7. Risks and mitigations

| Risk | Evidence | Mitigation |
|---|---|---|
| Harness API drift (the #570 class) | #570 removed the session list; 0 Claude Code rewrites for subscription users (E1, E2) | Candidate sets become `HarnessCapabilities` data (routing-v2 R0). A shared conformance test drives the same 8 scenarios through every hook. Golden decision tables. |
| Vendor usage readings | GAP-CLAUDE-QUOTA: statusline `rate_limits` not yet seen live; Cursor export blocked on 3.24.9 | Native snapshots only, with the 1 h stale grace in the [quota guide](../../guides/quota.md). Missing readings pass. No undocumented endpoint as a hard dependency. |
| Quality regression | Blind-set tier accuracy 42% to 53%; a small task called mid on 70% to 85% of small items (E4) | No tier change ships without matched-rigor evidence: acceptance suite and an A/B like the four-arm battery. The orchestrator's judgment, guided by the playbook, replaces the built-in risk floor (O8); a project can declare a tier floor. Declared rules replace regex inference. |
| Claims drift from evidence | The "last 30 days" label does not match the source's date span (D1); 73% of rewrite rows were no-ops | Every number carries its level. Count from-to changes and honored rewrites, never verdicts. No saving claim before v2 runs. |
| Headroom ceiling | 76% of Claude Code spawns already on haiku or sonnet (E1) | Describe the Opus-request case honestly. Measure the share of Opus-requested tokens that move. |
| Single-operator evidence | Every E1 and E3 figure comes from one machine | Stage 1 evidence from more machines through voluntary `downshift report` output. No telemetry leaves the machine by default. |
| Vendor native routing | Per-call `model` already exists (section 5) | Portable declared policy; the decider stays in shared layers. |

## 8. Success metrics and kill criteria

**Metrics** (definitions fixed before a comparison is read):

- **M1 Moved child-token share, Claude Code.** Child tokens whose honored final model is a smaller tier than requested, over all child tokens. Baseline: 0 on `main` after #570 (E1); 4.1% under the old inferred ladder (E1, window to fix).
- **M2 Tokens per accepted task, acceptance suite.** Tokens per accepted run, reported with the acceptance count, under the same rigor in every arm. Baseline: 24.1% fewer tokens per accepted run than opusplan, on mechanical tasks (E3, 12 accepted per routed arm).
- **M3 Risk exposure (O8).** Spawns with risk vocabulary that ran on the small tier. Reported, not gated: there is no built-in floor any more, so the owner reads this as the residual risk of the orchestrator's choice and declares a floor if it is too high.
- **M4 Routing-note coverage.** Share of decisions that carry a routing note. Target 100%.
- **M5 Acceptance.** Acceptance rate per arm. It must not fall below the requested-model arm.
- **M6 Token economy per harness.** Tokens the Compactor avoided and tokens moved to a smaller model, reported in separate columns per harness, never summed and never converted to dollars.
- **M7 Limit hits avoided.** Spawns that went down while a quota window was critical. Counted, not claimed as a saving.

**Kill criteria** (proposals; the owner decides):

- **K1.** Stage 1 does not move more child tokens than the 4.1% v1 baseline over a comparable window, same definition. Stop stage 2.
- **K2.** Any task in the acceptance suite that passed on the requested model fails under v2. Revert the tier change for that rule. No exceptions.
- **K3.** A harness vendor ships native routing that applies the same declared profiles. Stop writing hook rewrites for that harness; keep the policy and the routing note.
- **K4.** No external user has asked for support or enterprise features by the time stage 1 ships. Keep the open core free; do not start a paid offer.

Adoption, user counts, revenue and maintainer hours have no source in this repository and are not tracked.

## 9. Open decisions for the owner

Status, 2026-10-27: O1 to O7 in the [MVP plan](mvp-plan.md) settle P2 (no dollars) and D7 (native snapshots only). Status, 2026-10-26: the owner accepted the roadmap decisions D1 to D13 (see [roadmap section 8](roadmap.md#8-owner-decisions)). The items below are the business plan’s own questions and stay open unless the roadmap settles them. D8 here is superseded by roadmap D4.

- **D1. Publish the 4.1% and 8 of 158 figures?** Recommendation: not yet. The diagnosis reads events from 2026-10-21 to 2026-10-27 plus legacy backups from 2026-09-21 to 2026-10-13. That does not match one "30 days" window, and 10-27 is after today. Fix the window and keep the diagnosis in a durable place first.
- **D2. Where SKU and price detail lives.** Recommendation: keep MONETIZATION at the four options. Move any detail to the private `downshift-labs` repository, as [publishing-boundaries](../../reference/publishing-boundaries.md) asks.
- **D3. First revenue option, if any.** Recommendation: support. It needs no new build. Keep hosted unbuilt until a user asks for it.
- **D4. Target user for stage 1.** Recommendation: Claude Code users on paid plans. This matches the owner's order in routing-v2 (Claude Code first).
- **D5. Evidence sharing.** Recommendation: no automatic export. Accept voluntary `downshift report` output, and run a redaction check before anything is published.
- **D6. Clean-room trail.** Recommendation: move the external-orchestrator analysis into the private `downshift-labs` repository, so P1 can be audited after the scratch location is cleared.
- **D7. Vendor usage readings. Decided (O2): native snapshots only.** Do not add undocumented vendor endpoints as a hard dependency.
- **D8. Shadow stack.** Recommendation: keep the v1 classifier only as an offline suggester for one release, then delete it with routing-v2 R12. This follows the diagnosis: the shadow code changes no route by default.

## Verification

`go test ./internal/docscheck/` checks the file name (kebab-case) and the relative links in this file. Every relative link points at a file that exists in the repository. The document has no external links.
