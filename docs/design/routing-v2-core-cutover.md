<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Routing v2: cutting the v1 decider out of `internal/core`

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: Proposed, 2026-10-27

Scope: Phase 1, units 2 to 4, of the [roadmap](../internal/plans/roadmap.md). Design context:
[routing-v2.md](routing-v2.md) §7, [true-north-cleanup.md](../internal/tasks/2026-10-26-routing-v2/true-north-cleanup.md)
(wave B), [core-engine-ports.md](core-engine-ports.md), [layering.md](../contrib/layering.md).
All numbers were measured on branch `downshift/true-north` at `e79021ac`.

## 1. Problem

The roadmap says the v1 decider goes away and `core.Route` keeps the requested model until R6.
But `internal/core` is the hub. `go list -f '{{.Imports}}' ./...` shows **20 packages** that import it:
`cmd/downshift`, the 6 adapters, `benchmark`, `catalog`, `childend`, `contextopt`, `discovery`,
`hookport`, `hookutil`, `models`, `outcome`, `policyfile`, `quota`, `shellwrap`, `telemetry`.
14 of the 59 production files in `cmd/downshift` import it. Core itself imports `classify`, `fuse`,
`paths`, `quotamodel` and `session`. Core production code is 24 files and 2,686 lines. Core tests are
74 files with 272 `Test` functions.

Most importers use only **types**. The v1 decision is reached through exactly 7 production calls:
`claudecode.go:272`, `codex.go:209` and `cursor.go:146` (`core.RouteAgent`); `kirocrew.go:180`,
`opencode.go:201` and `antigravity.go:139` (`core.Route`); and `cmd/downshift/try.go:83`. Every adapter
then calls `Decision.PlanSession`, which goes to `PlanForSession` (`session_select.go:71`). That function
already returns "no rewrite" when `d.Checked && d.SafeVerdict == VerdictOK` (`session_select.go:108`).
So the cut is a seam at `Route`. It needs no adapter rewrite.

## 2. What in `internal/core` is a shared type and what is the v1 decision

Exported symbols per file come from a grep of the declarations. Who uses them comes from
`grep -oE 'core\.[A-Z]\w*'` over the production files of each importer.

| File (lines) | Content | Used outside core by | Action |
|---|---|---|---|
| `capabilities.go` (237) | `HarnessCapabilities`, `*Caps`, `AllHarnessCaps`, `RewritePlan` (:207), usage sources | all adapters, cmd, quota, shellwrap, contextopt, childend, telemetry | **keep** (R0 adds the candidate set here) |
| `models.go` (142) | `Tier`, `Model`, `Effort`, `EffortFor`, `ConstrainEffort` | catalog, models, adapters, outcome, telemetry | **keep** |
| `resolver.go` (80) | `Resolver`, `ExactIDResolver`, `FamilyEffortResolver` | catalog, models, hookport, childend, telemetry, adapters | **keep** |
| `quota_hooks.go` (39) | quota load and parse hooks | quota | **keep** |
| `session.go` (99), `session_native.go` (142), `session_inferred.go` (22), `spawn_session.go` (34) | aliases to `session`, native lists, `isBaselineModel`, `PlanSession` | adapters, cmd, discovery | **keep** |
| `session_select.go` (337) | `PlanForSession`, `CanWriteSessionID`, `HarnessOwnsID`, target selection | adapters | **keep**. This is channel planning: v2 hands it a target tier |
| `session_resolver.go` (286) | session resolver (keep); `RouteForSession` (:139) | adapters, discovery | **keep**, delete `RouteForSession` |
| `env.go` (237) | `Config`, `SetConfig`, `ConfigFromEnv` | cmd | **keep**. Drop the NB and economy fields (:77-:121); add `Routing` |
| `policy.go` (387) | `Decision` (:31), `Verdict` (:161), `Baseline`, `Session`, `Guards`, `Shadow`; `TierOf` (:13), `RouteAgent` (:201), `classifyTask` (:299), `RaiseTier` (:266), `compareToCurrentModel` (:326), `Summary` | everywhere (`Decision`); adapters (`Route*`) | **split**: types stay; `Route`/`RouteAgent` become a facade over `Decider`; the body goes into `v1Decider` and is then deleted; `compareToCurrentModel` moves to `routing/decide` |
| `selfcheck.go` (187) | rules R1-R9, `Check`, `Report` | telemetry (`reason_codes.go:61-68`) | **delete** after telemetry has its own enums |
| `escalation.go` (108) | `EscalationIntent`, `IntentFor` | `try.go:131` | **delete** with `try.go` |
| `agent_type_hint.go` (45) | v1 agent-type tier | none | **delete** (R4 replaces it) |
| `classify_facade.go` (35) | aliases to `classify`, `RiskCategories`, `RiskCategoryNames` | benchmark, outcome, cmd, policyfile | **shrink** to the risk calls, then **delete** at R5 |
| `semantic_fuse.go` (10) | `ClassifyWithSemantic` | benchmark, outcome | **delete** |
| `nb_upshift.go` (49), `nb_downshift.go` (43) | NB opinion hooks | none: `NBSuggestHook` and `NBTrivialHook` are never set in production (`nbopinion` left in wave A) | **delete** (dead today) |
| `memory_adjust.go` (14) | `MemoryAdjustHook` | none: never set (`routeadapt` left in wave A) | **delete** (dead today) |
| `mode.go` (60) | economy mode | none outside core | **delete** |
| `gateway_shadow.go` (66), `gateway_live.go` (11) | gateway shadow resolvers | `catalog.go:781`, `telemetry/token_direction.go:116` | **delete** (gateway removed in 8c1afdfa) |
| `tier_floor.go` (16) | `TierFloorHook` | `policyfile.go:24` | **delete** in Phase 1. R2 owns floors (see §6, risk 2) |

Result: 11 files (1,655 lines) stay as the type and channel layer; `policy.go` and `classify_facade.go` shrink; 11 files (609 lines) are deleted,
plus the body of `policy.go`. Core then imports `paths`, `quotamodel`, `session` and, until R5,
`classify` (risk only).

## 3. Tests that depend on the v1 decider

**Method (probe, not a gate).** I made a copy of the tree outside the worktree and renamed
`RouteAgent` to `routeAgentV1`. A new `RouteAgent` wraps it and returns a keep-requested decision:
`Verdict` and `SafeVerdict` are OK, `Checked` is true, `Model` is the current model, and
`Corrections` is nil. Then I ran `go test -count=1 -json ./...` on the original and on the patch, and
took the difference. The original fails only `TestSafeExecGit` and `TestDCIImpact`, because the copy
has no `.git`. The run covers 1,858 top-level tests.
**121 test functions fail only under keep-requested** (plus 8 package-level failures).

| Package | Fail under keep | (a) contract | (b) v1 policy | (c) v2 golden seed |
|---|---|---|---|---|
| `internal/core` | 34 of 271 | 4 | 21 | 9 |
| `cmd/downshift` | 26 of 363 | 17 | 6 | 3 |
| `adapters/claudecode` | 14 of 79 | 7 | 3 | 4 |
| `adapters/cursor` | 12 of 26 | 3 | 2 | 7 |
| `adapters/antigravity` | 10 of 21 | 6 | 3 | 1 |
| `adapters/codex` | 8 of 27 | 4 | 1 | 3 |
| `adapters/kirocrew` | 8 of 18 | 5 | 0 | 3 |
| `adapters/opencode` | 7 of 18 | 5 | 1 | 1 |
| `internal/telemetry` | 2 of 226 | 2 | 0 | 0 |
| **Total** | **121** | **53** | **37** | **31** |

The split goes by test name, and I spot-checked it. It is a first pass, not a verdict.

- **(a) Contract (53).** These tests use a v1 rewrite only as the trigger. What they check is the
  wire: sibling fields kept (`TestHandleQuotaGovernsRewriteAndPreservesSiblings`), allow-listed
  envelope keys, multi-child reporting, honor observation, fixtures, conformance, spawn and child
  lines, pause, and control group. They stay valid with any decider once a **fixed stub decider**
  drives them.
- **(b) v1 policy (37).** `TestRoute_*`, `TestShiftMatrix`, `TestDecisionSummary_*`, `TestRunTry_*`,
  and "trivial goes to Flash Lite" style tests. They die.
- **(c) v2 golden seed (31).** Held, risk, explicit_only, unknown-baseline, family-in-session, strict
  native name and floor scenarios, such as `TestRiskFloor_BlocksDownshiftAllowsUpshift`,
  `TestHandle_FamilySlugRewritesOnlyWhenResolvedIDIsInSession` and `TestPlan_StrictHarnessWritesNativeName`.
  Their scenarios become rows in the R6 golden tables.

**Tests that die by deletion but pass under keep:** `classify` 36, `fuse` 5, `semantic` 20, `nbtier` 5,
and 106 core tests in 39 v1-only test files (121 tests, 15 of them already counted above). Total: **172**.
**Risk tests that move with the scanner:** 6 in `classify` (`risk_scan_equiv_test.go` and the other
risk-named ones) and 14 in core (`risk_test.go`, `risk_vocab_test.go`, `risk_patterns_test.go`).

**Category totals:** (a) about 1,600 kept: every untouched test, the 53 contract rewrites, and
`session`'s 9 tests, which all pass under keep. (b) 209 die (37 + 172). (c) 31 seed the golden tables.
20 risk tests move.

## 4. Proposed cut: small steps, `make check` green after each

**The seam.** It copies the `SetConfig` pattern that already exists (`env.go`; the runner calls it at
`hookrun.go:198`):

```go
type Request struct{ Prompt, Harness, AgentType, CurrentModelID string; Resolver Resolver }
type Decider interface{ Decide(Request) Decision }
func SetDecider(d Decider) (restore func())   // called once at the edge, like SetConfig
func RouteAgent(p, h, a, cur string, r ...Resolver) Decision // facade: activeDecider.Decide(...)
```

`internal/routing/decide` will import `catalog`, and `catalog` imports `core`. So core cannot import
the v2 decider. Core owns only the interface; `cmd/downshift` injects the implementation. Adapters keep
calling `core.Route`/`RouteAgent`. That follows the layering rule: adapters hold no policy, and the
decision is chosen in the shared runner.

| Step | Points | Files touched | Tests affected | Risk | Branch |
|---|---|---|---|---|---|
| S1 Seam | 2 | `core/policy.go`, new `core/decider.go`, `cmd/downshift/hookrun.go` | 0 change. Add 1 shared-layer test: a stub decision reaches each adapter's output through `runHookAdapter` | low: pure refactor, behaviour identical | main-eligible (no behaviour change); default is to keep it on true-north |
| S2 Stub decider for contract tests | 3 per PR, one PR per adapter or cmd | adapter `*_test.go`, new test-only `core/decidertest` | the 53 (a) tests move from prompt text to a fixed decision | low; it checks the contract survives | main-eligible |
| S3 Keep-requested behind a flag | 2 | `core/decider.go`, `env.go` (`Config.Routing`, `DOWNSHIFT_ROUTING=keep`), `session_select.go:89` | new golden: with the flag, all 6 adapters emit an unchanged spawn (no model, no effort) | medium: `ApplyEffort` (`session_select.go:89`) still writes effort when `d.Model.ID != ""`; keep must clear it | main-eligible while v1 stays the default |
| S4 Telemetry enums (unit 2) | 3 | `telemetry/{telemetry,reason_codes,token_direction,completion,tokens_rerouted,liveline,model_canonical,aggregate}.go`, `hookrun.go` | 15 telemetry test files that import core; 2 (a) tests | medium: event schema and stats must not change; needs a fixture diff | true-north (parallel with S1 to S3) |
| S5 Flip the default to keep | 2 | `decider.go`, the (b) and (c) test files | delete 37 (b); rewrite 31 (c) to assert "unchanged" and list them as R6 golden rows | high: routing is a no-op (D5) | **true-north only** |
| S6a Delete the v1 decider in core | 3 | the 11 files in §2, the `policy.go` body, `try.go`, `cmd_try.go`, `harnesses.go:36` | 106 core tests in 39 files, plus the try tests | low after S5 | true-north only |
| S6b Delete `fuse`, `semantic`, `nbtier` | 3 | those packages; `cmd/downshift/{config,doctor,context,outcome}.go`; `outcome/report.go:26`; `benchmark`; `archtest/imports_test.go:52-56,117` | 30 tests in those packages; archtest rules updated in the same PR | medium: `outcome` and `benchmark` measure the v1 classifier, so the owner must decide to delete them or re-point them at v2 | true-north only |
| S6c Shrink `classify` to `risk.go` | 2 | `internal/classify/*` except `risk.go`; core risk tests move into `classify` | 36 die; 20 move | low | true-north only |
| S7 Docs (unit 5) | 1 | `architecture.md`, `coverage.md`, `layering.md` (layers table, NB row) | `docscheck` | low | with S6 |

**Order:** S1, then S2 and S3, then S5, then S6a, S6b and S6c in that order. S4 runs in parallel from
the start. S5 depends on S3 (it needs the flag) and on S4 (`reason_codes.go` must stop naming the
rule ids before `selfcheck.go` goes). S2 must be done before S5, or the 53 contract tests are lost
together with the policy tests.

Every step stays under 400 lines of diff, except the deletions. Deletions are not counted against the
cap, but S6 is split three ways to keep review readable.

## 5. Where the risk scanner lives until R5, and what telemetry needs (unit 2)

**Risk scanner.** `internal/classify/risk.go` is 319 lines. `go list -f '{{.Imports}}' ./internal/classify`
shows standard library only, so it can stand alone. Until R5 it stays in `internal/classify` with
`risk_scan_equiv_test.go` and the 20 risk tests (S6c). Core keeps two calls, `RiskCategories` and
`RiskCategoryNames`, so the keep decider still fills `Decision.Risk` for telemetry, and
`policyfile` keeps validating category names. R5 moves the file to `internal/routing/guard`. That
deletes the facade and the package, and switches `policyfile` to `guard`.

**Telemetry.** `go list -deps ./internal/telemetry` pulls in `core`, `classify`, `semantic`, `fuse`
and `session`. All of these arrive through `core`; the only direct `downshift` imports are `codeintel`,
`core` and `paths`. Per file:

| File | Uses | Replacement |
|---|---|---|
| `telemetry.go:434` `FromDecision(core.Decision)`, `:459` `VerdictUpshift`, `:485` tiers | the whole Decision | `telemetry.DecisionFacts`: plain strings and bools that the runner fills (`hookrun.go:151` `recordedDecisions`) |
| `reason_codes.go:61-75` | `Decision` and 8 v1 rule ids | telemetry-owned `ReasonCode` strings. The runner maps them now; R11 (`routing/note`) maps them later |
| `token_direction.go:98-116` | `Resolver`, `Model`, gateway resolvers | a local tier-lookup interface; delete the gateway lines |
| `completion.go`, `tokens_rerouted.go` | `AllHarnessCaps`, `UsageNone`, `HarnessCapabilities` | a `map[harness]usageSource` passed in by cmd |
| `liveline.go` | `StaleQuotaHold` | import `session` directly (`session` does not import core) or take the value as an argument |
| `model_canonical.go` | `ExactIDResolver` | a local interface with the same method set (structural typing) |
| `aggregate.go` | `TierFrontier` | the string `"frontier"` |

Gate: an `archtest` rule that `telemetry` must not import `core`, added in the S4 PR.

## 6. Risks and what was not verified

1. **Biggest risk: losing contract coverage.** The 53 (a) tests check the wire only because v1 happens
   to rewrite. If S5 lands before S2, the sibling-field, multi-child and honor guarantees lose their
   only proof until R6. S2 must come first, and the tests must not be deleted in bulk.
2. **Project floors stop working.** Under keep-requested, the `policyfile` floor
   (`TierFloorHook`, `policyfile.go:24`) has no effect. A user with a floor gets no rewrite until R2
   and R6. Accepted by D5 and D10, but it belongs in the release note.
3. **Conflict with roadmap unit 3:** that unit lists `internal/session` for deletion. `session` is the
   allowlist source for `PlanForSession` and R0. It is not v1 policy, and its 9 tests pass under keep.
   This proposal keeps it. The owner should amend unit 3.
4. **Effort writes.** `PlanForSession` can set `ApplyEffort` even with no model change
   (`session_select.go:89`). Whether keep-requested may still write `reasoning_effort` is an owner
   decision. S3 defaults to "no".
5. **A package-level decider is global state.** `core-engine-ports.md` criticises hook variables.
   `SetDecider` with a restore function is the same pattern as `SetConfig`. It is set only at the
   edge, and tests restore it. A ports refactor stays out of scope.
6. `docs/contrib/layering.md` still names `internal/nbopinion` as landed; the package is gone.
   S7 fixes it.

**Not verified:** the a/b/c split is by test name, with spot checks, not a review of each test.
Table entries in package-level vars and assertions inside helpers may be miscounted. The probe was not
run with `-race` or on Windows. `make check` was not run (doc only). The line counts per step are
estimates. I did not check whether `benchmark` or `outcome` are reachable from a shipped command.

Verification of this document: `go test -count=1 ./internal/docscheck/`.
