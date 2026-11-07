<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Evidence ladder: what each saving figure means

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Five kinds of "saving" appear around this project, and they are not the same number. This page separates them, quotes
each figure from the file that holds it (checked against the current file on 2026-10-17), and says which rung the
project stands on today. Rule of reading: a figure on a lower rung never stands in for a higher one.

| Rung | What it is | Status today |
|---|---|---|
| A | Theoretical saving from catalog list prices | Arithmetic, always available |
| B | Saving estimated by telemetry (normalised units, tokens by tier) | Available on Claude Code; an estimate per decision |
| C | Saving measured in controlled benchmarks | Partial: quality measured, cost not |
| D | Saving demonstrated on real tasks, routed versus control | One measurement, one operator, 2026-10-15 |
| E | Cost per task completed with acceptable quality | Not measured; `report` states tokens per completed task and rework for routed spawns, but does not emit cost or judge quality. The routed-versus-control token comparison that would have unlocked it cannot run since 2026-11-02 (control arm removed) |

## A. Theoretical saving (list prices)

Source: `internal/catalog/catalog.json` (`version` 2, `updated` 2026-10-16), USD per 1M tokens.

| Model | Input | Output | Cache read | Cache write |
|---|--:|--:|--:|--:|
| `claude-haiku-4-5` (small, the native `haiku` alias) | 1.00 | 5.00 | 0.10 | 1.25 |
| `claude-sonnet-5-5` (mid) | 2.00 | 10.00 | 0.10 | 2.50 |
| `claude-opus-5-5` (frontier) | 4.00 | 20.00 | 0.20 | 5.00 |

A Haiku child costs 75% less than Opus and 50% less than Sonnet on uncached input and output tokens. On cache reads the
gap to Opus is 50% and the gap to Sonnet is zero ($0.10 / 1M tokens for both). A subagent's traffic is mostly cached context (the one transcript
checked in [beta-exit.md](../reference/beta-exit.md#pillar-3--billing-beforeafter-real-dollars) had 18 input, 311
output and 82,900 cache tokens, and cache reads exceeded 98% of total tokens in the four-arm battery), so "75%" is the ceiling of one token class, not the saving on a child. Crucially, downshifting from Sonnet to Haiku produces **zero price reduction on prompt cache reads**. Rung A also
assumes the same tokens on both models and a task that passes on the cheaper one; both assumptions are what rungs C
to E test. Detailed source-to-claim reconciliation: [anthropic-references-benchmark-reconciliation-2026-11-07.md](../evidence/anthropic-references-benchmark-reconciliation-2026-11-07.md).

## B. Saving estimated by telemetry

Source: `internal/telemetry/aggregate.go` (`Aggregate`), `export.go`, [stats-export.md](../reference/stats-export.md).

- **Tokens by tier** (`routing_tokens`, since schema 4; it replaced `tokens_rerouted` and `tokens_saved`): the child's recorded tokens, placed by the catalog tier of the
  model that ran against the session model (below, or not), or against the requested model when the session model is unknown. The counterfactual is the same token count on
  the baseline model, not a second run. Tokens come from the child transcript on `SubagentStop` (Claude Code only).
  Older event-log records may carry dollar fields (`actual_cost_usd`, `baseline_cost_usd`); current builds do not write
  them, and current `stats` output and schema 3 and 4 exports contain no dollar amounts. The provenance below preserves
  figures from the older schema under which they were recorded.
- **Provenance.** [billing-gap-audit.md](../evidence/billing-gap-audit.md) (2026-10-09, 7 days): `total` 186,
  `usage_records` 15, `usage_linked` 5, then-measured `actual_cost_usd` 15.432977 and `real_saved_usd` 0.424866 (schema 2),
  `baseline_no_route` 0; native tokens and model matched the record on 5 of 5 linked children. That is provenance
  for the token counts, not for a bill. P3.4 needs 50 linked records and stands at 5 in that audit.
- **Routing tokens, and why "tokens saved" is not a claim.** Routing does not reduce tokens: the child reads and
  writes about the same tokens on another model. `routing_tokens` counts, per harness and never summed across harnesses,
  the billable tokens (input, output, cache write; cache read apart) of the usage records, folded per spawn, and how
  many ran on a lower tier than the session model (`parent_model`). A spawn with no usable session model is placed against
  its requested model in separate fallback fields, never added to the session ones; a spawn neither can place is
  unobserved. It is rung B evidence: the counts are observed (Claude Code only today, because it is the only
  harness that records child tokens). The current views print token counts, no dollar and no price.
  It carries these biases: an unknown model or a decision with no usage record contributes nothing to the
  below counts. Downshift does not claim fewer tokens: no run without routing exists to
  subtract from, a lower tier can need more tokens or a retry, and on a plan with no per-token bill (Claude Code Pro,
  Max, Team; Cursor plans) what was saved is not tokens or dollars but headroom against a quota, which no hook sees.
- **The live views say "below the session model", not "saved"** (owner decisions 2026-10-19, 2026-11-03). The session
  tree, `session --panel`, the statusline, `watch`, `dsmon`, the child-end line and the `stats`
  headline show the billable tokens that ran on a lower tier than the session model, per harness and per agent type
  (`telemetry.RoutingTokens`; unrouted, held, kept and refuted children count in the total). Cache reads are
  not in the billable tokens: they follow the context a child rereads, not the tier it ran on. These views carry no dollar;
  `downshift report` prints no dollar either: it shows tokens per completed task for routed spawns, and the routing tokens per harness. The
  figure is a count of where tokens ran, not a reduction in tokens and not a dollar saving. `downshift session --why` quotes no
  price or percentage for one decision: it says whether its tokens would run below the requested model, and only when the hook would make that rewrite.
  Per harness, what is observed and what blocks the rest: [value-per-harness.md](../reference/value-per-harness.md).
- **Known biases.** A model the catalog does not rank gives an unobserved spawn, never a guess; a decision with no usage record gives units
  only. The headline percentage this produces (19.3% in one operator's log, per
  [subagent-routing-ab-2026-10-15.md](../evidence/subagent-routing-ab-2026-10-15.md)) is an estimate per decision at
  list price, not a measurement.

## C. Saving measured in controlled benchmarks

Sources: [benchmark/report.md](../benchmark/report.md) (last updated 2026-10-15),
[hardening-audit-2026-10-15.md](../evidence/hardening-audit-2026-10-15.md),
[evaluation-suite-inventory.md](../evidence/evaluation-suite-inventory.md). Datasets live in the private downshift-labs
repository; the public repo holds aggregates and two sample tasks under `internal/outcome/testdata/`.

| Benchmark | n | Held constant | Result | Cost measured? |
|---|--:|---|---|---|
| Classifier, seed regression net | 200 | prompts, labels, `ClassifyWithSemantic`, no adapt memory | 73.5% tier accuracy (95% CI 67.0–79.1%) from `236c057` (2026-10-12) onward, measured again at `2c8f3f4`; the 69.0% that `docs/benchmark/report.md` carried until 2026-10-17 was the classifier at `be1054ac` (2026-10-06), resolved in [baseline-freeze-2026-10-17.md](../evidence/baseline-freeze-2026-10-17.md) | No |
| Classifier, burned holdout | 300 | same | 100.0%; tuned against, so not generalisation | No |
| Classifier, eval-only `fresh` / `heldout2` | 100 / 120 | same, split never tuned | 50.0% (40.0–60.0%) / 40.0% (31.7–49.2%) at `5a888e7`; 53.0% / 41.7% at `3067340`. Most misses are abstentions (router keeps the model); confident and wrong: 2 of 37 and 6 of 28 | No |
| Outcome suite, fixed model per tier | 40 tasks, 1 run per tier | prompt, hidden `check_test.go`, one model per tier, 2026-10-03 | frontier `anthropic/claude-opus-5.5` 40/40; mid `deepseek/deepseek-v4-flash` 31/40 (77.5%); small `qwen/qwen3-coder-30b-a3b-instruct` 27/40 (67.5%). TRIVIAL+SIMPLE: 20/20, 17/20, 16/20 | No: `run.json` holds tier, model, date and pass/fail only (issue #100) |
| Classifier routing on that suite | 40 | recomputed 2026-10-15 | 6 small, 22 mid, 12 frontier; 37/40 pass at the picked tier | No |
| Adapt memory, offline | 40 | same runs | per-task upper bound 40/40; shared shape memory 37/40 (0.0pp vs classifier) | No |
| Held-out suite (GAP-SUITE) | 6 tasks, 1 smoke run | Go tasks outside the development suite | small 2/6, mid 4/6, frontier 6/6, USD 0.14 total ([gap-tasks.md](../internal/gap-tasks.md)) | Total only, not per task |

Two notes. The small-versus-frontier gap on TRIVIAL+SIMPLE labels is −20.0 points on fixed-model runs, with
overlapping intervals; the suite's own rule is to revisit the downshift below −5.
The complete task inventory of the 40 development tasks, the 6-task GAP-SUITE held-out evaluation set, and the minimal
reproducible smoke evaluation set for immediate post-O8 changes without requiring expensive API calls are documented in
[evaluation-suite-inventory.md](../evidence/evaluation-suite-inventory.md).

## D. Saving demonstrated on real tasks

Source: [subagent-routing-ab-2026-10-15.md](../evidence/subagent-routing-ab-2026-10-15.md). One operator, Claude
Code on `sonnet-5-5`, same prompts in both arms, control = same hooks with the since-removed `DOWNSHIFT_NO_ROUTE=1` switch, acceptance = exit
code of hidden tests, cost = Claude Code's own `total_cost_usd` (list price), runs shuffled, usage-limit refusals
excluded, comparison restricted to runs with at least 3 spawns.

| Batch | Tasks | n per arm | Accepted routed / control | Avg cost routed / control | Downshifts applied |
|---|---|--:|---|---|--:|
| b2 | 4 real fixes, delegation optional | 8 | 7/8 / 7/8 | $0.65 / $0.77 | 0 (1 upshift to Opus) |
| b5 | 4 mechanical tasks, one subagent per piece | 12 | 12/12 / 12/12 | $0.59 / $0.67 (−12%) | 19 of 56 (34%) |
| b6 | 4 real fixes, at least 3 subagents | 12 | 8/12 / 9/12 | $0.75 / $0.92 | 3 of 34 (9%) |

Per task in b5: `mech-rename` $0.49 / $0.74 (−33%, all five subagents downshifted in all three runs), `mech-constant`
$0.66 / $0.77, `mech-doc-comments` $0.59 / $0.65 (interval includes zero), `mech-test-mirror` $0.62 / $0.52 (noise:
nothing downshifted in 2 of 3 runs). Output tokens in b5: 39.8k on Haiku against 117.5k all on Sonnet. In b2 the
routed arm used about half the turns (13.9 against 30.6) with no downshift applied, so that gap is agent variation.
Codex without Downshift, 8 runs: 8/8 accepted, about $0.21 per run. Median wall time 141 s; mean not reported.

### Subsequent Controlled and Real-Use Batteries (2026-10-21)

Three subsequent empirical evaluations expanded this evidence base:
1. **Four-arm mechanical battery (Claude Code, 46 runs):** In [`four-arm-mechanical-2026-10-21.md`](../evidence/four-arm-mechanical-2026-10-21.md),
   isolated routing under native `opusplan` (Arm B vs Arm C) achieved **24.1% fewer tokens per accepted run** (95% CI +2.1% to +46.7%)
   and 16.5% fewer tokens per run (+0.6% to +29.7%), with 12/12 accepted in both routed arms. Prompt cache reads accounted for >98%
   of total tokens across all arms.
2. **KiroCrew combined routing and compaction trial:** In a multi-hour session evaluating 6 control tasks, combining native policy
   routing with shell advise compaction achieved **up to ~53% fewer tokens per task** (directional real-use measurement; see
   [`roadmap.md`](../roadmap.md#now-beta) line 68 and [`anthropic-references-benchmark-reconciliation-2026-11-07.md`](../evidence/anthropic-references-benchmark-reconciliation-2026-11-07.md)).
3. **CCR Compactor benchmark:** On the 107-fixture synthetic corpus ([`bench.md`](../reference/bench.md)), output compaction
   achieved **72.0% byte reduction with CCR** (51.1% raw without CCR), reaching 65.2% on lossless test runners while preserving 100% of diagnostic failures.

What D supports: a saving of about 12% to 24% on mechanical delegated work, a third on pure renames, and up to 53% when combined
with shell compaction in KiroCrew, with 100% acceptance on mechanical tasks. What it does not: any proven figure for unconstrained natural
everyday use (where 30-day telemetry shows −1.9%, within noise), or generalization beyond mechanical disjoint tasks.

## E. Cost per task completed with acceptable quality

**Not measured today.** The definition exists and `report` states tokens per completed task for routed spawns, but that command does not
compute dollars or assess task quality. It has no unrouted side to compare with (the control arm was removed on 2026-11-02), so the
routed-versus-control token comparison under P3.7 cannot be run.

`downshift report` (`cmd/downshift/report.go`, `internal/telemetry/report.go`, `tasks.go`) defines a completed task as a
child that ended `completed` and has a usage record with tokens; tokens per completed task are that child's tokens plus every
failed attempt it redid (a respawn in the same session within 10 minutes of an `error` or `interrupted` end); rework
rate is reworked spawns over spawns with an observed end. The report prints these for routed spawns only, with no comparison and no threshold.

What is missing for a per-task cost figure:
- **Acceptance vs Completion:** A subagent `completed` status simply means the tool process exited with code 0 or finished turns. It does **not** judge functional correctness or acceptance. Small models fail 32.5% of tasks in the 40-task outcome suite ([evaluation-suite-inventory.md](../evidence/evaluation-suite-inventory.md)); if broken code passes as "completed", subsequent lead-session rework erases all savings.
- **Missing Failure Payloads (GAP-CHILD-FAILURE):** The `PostToolUseFailure` error payload is not delivered by harness hooks ([billing-gap-audit.md](../evidence/billing-gap-audit.md)), leading to potential undercounting of child failure and rework.
- **Workflow Hook Bypass (GAP-WORKFLOW-AGENTS):** Spawns from Claude Code `Workflow` scripts bypass `PreToolUse` entirely ([workflow-agents-bypass-pretooluse-2026-10-11.md](../evidence/workflow-agents-bypass-pretooluse-2026-10-11.md)), remaining unrouted unless declared in script parameters.
- **Unmeasured Parent Overhead (GAP-COORDINATION):** Orchestrator tokens spent generating briefs and reviewing child outputs are not captured by subagent hooks ([delegation-boundaries-and-overhead-2026-11-07.md](../evidence/delegation-boundaries-and-overhead-2026-11-07.md)).
- **Expanded Held-out Testing (GAP-SUITE):** Sizing quality degradation requires independent held-out evaluation beyond the current 6-task smoke set ([evaluation-suite-inventory.md](../evidence/evaluation-suite-inventory.md)).

## Metrics table

Status: **available** (recorded and printed), **inferred** (derived from indirect evidence), **estimated** (a price
model applied to something recorded), **not available** today. Claude Code unless stated; other harnesses record
decisions but not usage or ends ([harness-matrix.md](../reference/harness-matrix.md#child-end-n5-2026-10-13)).

| Metric | Status | Where |
|---|---|---|
| **Total cost per task completed with acceptable quality** (primary) | not available | `downshift report` compares tokens per completed task once 20 completions per group exist; it emits no dollar amount, and quality is manual |
| Cost per run (parent session) | not available from Downshift | hooks see subagents only; the A/B used Claude Code's `total_cost_usd` by hand |
| Tokens per subagent | observed | `downshift session --tree`, `stats --detail`; transcript tokens, placed by tier |
| Input / output / cache tokens per child | available | usage event fields `input_tokens`, `output_tokens`, `cached_tokens`, `cache_write_tokens`; `observed_child_tokens` in `stats --export` |
| Latency, total and per step | not available | no duration field on decision, usage or end events (`latency_ms` exists only on the E5 shadow record); the A/B timed runs externally |
| Completion rate | available | `child_status` counts in `stats` ("How children ended") and `--export`; `report` `ended` / `completed_tasks` |
| Failure rate | available, undercounted | `child_status` `error` / `interrupted`; `PostToolUseFailure` payload never captured (GAP-CHILD-FAILURE) |
| Retries | inferred | "Respawned after a failure" in `stats`, `↻` in the session tree (10-minute rule, a hint) |
| Result quality | not available | hidden tests in the A/B only |
| Coordination and delegation cost | not available | parent turns and tokens outside the hook; GAP-COORDINATION open; four-arm framework and breakeven boundaries in [delegation-boundaries-and-overhead-2026-11-07.md](../evidence/delegation-boundaries-and-overhead-2026-11-07.md) |
| Fixed-context cost | available per child, as tokens | `cached_tokens` / `cache_write_tokens` counted apart by class; parent context not seen; `context metrics` token fields unavailable |
| Abstention rate | available | reason codes `NO_SIGNAL`, `CLASSIFIER_UNCONFIDENT` in `stats --export` `reason_codes` |
| Routing decisions and guardrails fired | available | `verdict`, `corrections`, `HELD_*` codes; `stats` "Guardrails held"; `session --why`; `stats --by-harness` |
| Requested versus executed model | available / inferred | `requested_model` and `final_model` on the decision; `resolved` record with `rewrite_honored` when the harness reports the child's model; otherwise `rewrite_honored_inferred` from a later spawn |
| Baseline comparison | estimated (counterfactual) | `stats` same-token counterfactual, marked `baseline_source: inferred` when guessed |

## Three saving scenarios and what each must prove

> **O8 (2026-10-28).** The orchestrator now chooses the model; Downshift applies it within the declared policy. The
> built-in risk floor (R5 below), the automatic upgrade after a failure and the classifier-driven tier pick were
> removed. The measured trials in this guide ran on the pre-O8 router and are kept as the record of that behaviour;
> they do not describe the product today. Re-measure with `downshift report` before quoting a saving.

The objective is one sentence: minimise cost per task completed correctly, with safety and developer experience
preserved. It is never "pick the cheapest". The guard in `internal/routing/guard` (`Check`, called by
`internal/routing/decide`) encodes that: an unconfident decision or an unknown requested tier keeps the requested model
(R1, R6, R7, R9), and a target above the requested tier is refused (`never_widen`, R8). Since owner decision O8 there
is no built-in risk or COMPLEX downshift veto (R2, R5); a project declares a floor in `.downshift/policy.json`.
Downshift never raises a tier on its own; the only moves up are deterministic: the project floor and the opt-in
`raise_complex`. Doubt never spends money, in either direction (`TestHookE2E_DoubtHoldsUpshiftAndUnknownFrontier` in
`cmd/downshift/doubt_held_e2e_test.go`, `internal/routing/guard/guard_test.go`). A scenario below counts as proven only when its metric is measured
with those guardrails on; a saving bought by removing them is a different product.

| Scenario | Must prove | What acts in Downshift | Which metric answers it | Status today |
|---|---|---|---|---|
| 1. Same provider, smaller model (an Opus or Sonnet child to Haiku) | Cost down without relevant quality loss | `Route` in `internal/core/policy.go`: `Complexity.Tier()` picks the tier, `compareToCurrentModel` returns `VerdictDownshift` with `SavingsRatio` from list prices; `Check` runs before any adapter acts; `Plan(caps, resolver)` in `capabilities.go` turns the decision into a wire rewrite inside the session allowlist, with the catalog `native_name` (`haiku`) where the harness validates names | Current signals: routed tokens per completed task and rework in `downshift report`; tokens by tier in `downshift stats`. Neither emits dollar amounts. The A/B method separately records acceptance and `total_cost_usd` per run (rung D); see [benchmark-protocol.md](benchmark-protocol.md) for its planned arms | **Measured, narrowly.** b5: $0.59 against $0.67 per run (−12%), 12/12 accepted in both arms, 19 of 56 spawns to Haiku; `mech-rename` −33% ([subagent-routing-ab-2026-10-15.md](../evidence/subagent-routing-ab-2026-10-15.md)). These are historical list-price estimates from the 2026-10-15 trial, not current `stats` or `report` outputs. Not measured: natural use (b2 applied 0 downshifts, b6 3 of 34), quality beyond structural checks, intervals for the historical per-run figures, rung E (no window has 20 completed tasks per group). The quality risk is sized by the outcome suite, small 27/40 against frontier 40/40 (`docs/benchmark/report.md`) |
| 2. Same model, lower reasoning effort | Total saving without more retries and rework | `EffortFor(tier)` in `internal/core/models.go` (small → low, mid → medium, frontier → high), set by `Route` and `RaiseTier`; the catalog's `EffortFor` maps the level to the harness string through `effort_map`, and `ConstrainEffort` / `AllowedEfforts` keep it to values verified for the exact id. Only Codex has `CanApplyEffort` (`capabilities.go`); its adapter writes effort on a confident unheld decision or alongside a model rewrite and never lowers it on doubt (`effortAllowed` in `internal/adapters/codex/codex.go`; `TestHandle_HeldDownshiftKeepsRequestedEffort`, `TestHandle_UnconfidentOKDoesNotLowerEffort`, `TestHandle_UnconfidentKeepsRequestedEffort`). Claude Code, Cursor, Antigravity and KiroCrew have no effort field; Grok is config only | Would be routed tokens per completed task and rework in `report`. Codex has `UsageSource: none` and an unverified end parser, so neither token usage nor completion is currently observed there. Effort has no price in the catalog, and current `stats`/`report` do not estimate dollar savings for an effort-only change | **Not measured.** Emission only: a Codex replay on 2026-10-09 emitted `reasoning_effort: low` with `gpt-6-luna` ([native-quota-routing-2026-10-09.md](../evidence/native-quota-routing-2026-10-09.md)); honored is inferred for the model (2026-10-02) and not observed for effort. 15 Codex spawn inputs checked on 2026-10-09 carried no `reasoning_effort` key ([closeout-recheck-2026-10-09.md](../evidence/closeout-recheck-2026-10-09.md)), so the baseline effort is usually unknown. No retry or rework figure exists for any effort change |
| 3. Cross-provider routing (OpenRouter ids, catalog family prefixes) | Net gain across price, latency, limits, compatibility and governance | `LookupByID` in `internal/catalog/catalog.go` strips the `provider/` prefix (`normaliseModelID`) and resolves exact → alias → family prefix, so `anthropic/claude-opus-4-8` or `cc/claude-opus-4-8` gives a baseline tier (`TestLookupByID_OpenRouterExactMatch`, `TestLookupByID_OpenRouterFamilyMatch` and the other prefix-normalisation tests in `catalog_test.go`); an id outside the catalog stays unknown and the spawn is left unchanged. The target still comes from the calling harness's catalog and session list (`ModelFor`; R4 `RuleForeignModel`; `TestPlanForSession_ForeignIDDoesNotRewrite`). Only the Cursor catalog mixes providers across tiers (anthropic, cursor, google, openai, xai, zhipu); the others are single-provider, Claude Code apart from one `grok-4.6` entry. Limits: the quota gate skips exhausted models (`Session.QuotaStatus`, `CreditHeld`; `TestPlanForSession_PrefersIncludedAndSkipsExhausted`). Compatibility: `native_name`, `StrictModelName`, `AllowedEfforts`. Governance: `explicit_only`, the project tier floor | Price: catalog list-price entries can be inspected, but current `stats` and `report` expose no dollar comparison; `routing_tokens` records model-tier movement, not price savings. OpenRouter snapshot 2026-10-12; `gpt-6.1-sol` has catalog prices since #190 (2026-10-19). Latency: no duration field on any event (metrics table above). Limits: `downshift quota`, `QuotaStatus` on the decision. Compatibility: rewrite honored per harness ([harness-matrix.md](../reference/harness-matrix.md)). Governance: `policy_floor` and `HELD_*` counts in `stats --export` | **Not measured.** The lookup is pinned by tests; no live gateway has been verified; no A/B arm routed across providers. The A/B's only other-provider figure, Codex without Downshift (8/8 accepted, about $0.21 per run), is a different harness, not a routed comparison. Latency has never been measured. The fixed-model outcome suite ran OpenRouter ids (`deepseek/deepseek-v4-flash` 31/40, `qwen/qwen3-coder-30b-a3b-instruct` 27/40, 2026-10-03): quality only, no cost per task |

Scenario 1 has a protocol: arms B and C of [benchmark-protocol.md](benchmark-protocol.md) and the
[pilot](pilot.md). Scenarios 2 and 3 have none yet. Scenario 2 needs Codex usage and child ends before `report` can
say anything; scenario 3 needs a per-spawn duration field and an arm whose spawns run on
different providers. Until then the honest status for both is the last column.

## Where Downshift adds value beyond native opusplan and gateways

Three things sit near Downshift. Claude Code's native `opusplan` ([claude-code.md](claude-code.md), section 1) is a
main-session policy: Opus while plan mode is on, Sonnet once the plan is approved; it makes no per-spawn decision,
and a subagent inherits whatever the session runs. A gateway (LiteLLM, Portkey and the like,
[when-to-use.md](when-to-use.md)) owns transport, keys, retries and fallbacks and sees each request after the harness
has named a model. Academic and commercial router products (RouteLLM, RouterBench, FrugalGPT) invoke external embeddings or
meta-LLM calls on single prompts, incurring 100–300+ ms latency and routing token fees; see
[router-comparison-routellm-2026-11-07.md](../evidence/router-comparison-routellm-2026-11-07.md) for the benchmark comparison.
"Shown by" names a test or an evidence file; a claim with neither is marked unproven.

| Claim | Against | Shown by | Status |
|---|---|---|---|
| Acts at delegation time, per spawn, on the orchestrator's choice (pre-O8: from the task text), before the request leaves the harness | `opusplan` decides once per session mode; a gateway decides per request, after the harness chose | `PreToolUse` on `Agent\|Task` in `internal/adapters/claudecode/claudecode.go`, shared `runHookAdapter` in `cmd/downshift/hookrun.go`, `cmd/downshift/hook_e2e_test.go`; A/B b5: 19 of 56 spawns rewritten under a Sonnet session | Shown |
| Harness-honest observation: emitted, honored and used are three records (`rewrite_emitted`; `resolved` with `rewrite_honored`; `usage`), and a refuted rewrite saves nothing | A gateway sees the model named in each request, not which spawn it belongs to or whether the harness applied a rewrite | [claude-code-rewrite-honored-2026-10-06.md](../evidence/claude-code-rewrite-honored-2026-10-06.md) (two spawns); [billing-gap-audit.md](../evidence/billing-gap-audit.md) (5 of 5 linked records matched); `TestAggregate_RefutedRewriteSavesNothing`, `TestCountInferredHonored_RefutedBeatsLaterSpawn` in `internal/telemetry/refuted_inferred_test.go` | Shown on Claude Code. Codex honored is inferred (2026-10-02), Cursor unconfirmed (2026-10-10); other harnesses record decisions only ([harness-matrix.md](../reference/harness-matrix.md)) |
| Per-spawn guardrails: R1–R9, risk floor, `explicit_only`, quota gate; a held decision changes nothing | `opusplan` has none; gateway fallbacks fire on errors and quotas, not on task risk or classifier confidence | `internal/routing/guard/guard_test.go`, `internal/routing/decide/decide_test.go`, `TestHookE2E_DoubtHoldsUpshiftAndUnknownFrontier`, `TestHookE2E_ClassStepNeverMovesRiskOrComplexDown`, `risk_downshift_test.go`; [hardening-audit-2026-10-15.md](../evidence/hardening-audit-2026-10-15.md) (23 regression tests fail on the base commit); A/B b6: 2 upshift and 1 downshift verdicts held | Shown |
| Project tier floors: `.downshift/policy.json` by class and by risk, raise-only, applied by the shared runner for every harness | Neither `opusplan` nor a gateway reads a per-project floor from the repository | `internal/policyfile/policyfile_test.go`, `TestHookE2E_ProjectFloorThroughSharedRunner` (`cmd/downshift/policy_floor_e2e_test.go`), `TestRunKiroCrewHook_ProjectFloorApplies`; counted as `policy_floor` in `stats --export` | Shown by tests; no dated evidence file records a floor firing on a real project |
| Report: `downshift report` states tokens per completed task and rework for routed spawns, and the policy key that turns each routing verdict off | Not compared: what a session policy or a gateway reports per task is outside this repository's evidence | `internal/telemetry/report_test.go`; [report.md](report.md) | Mechanism shown; it has no unrouted side, so it shows no saving |
| Composes with `opusplan`: a spawn in plan mode inherits Opus and is routed as usual | Replaces nothing in the main session | Code path only: `configuredModel` resolves `opusplan` by `permission_mode` ([claude-code.md](claude-code.md), section 2) | Unproven: a live plan-mode spawn has not been observed |
| Saves beyond what `opusplan` alone saves | The B−C comparison | Protocol only: [benchmark-protocol.md](benchmark-protocol.md) and [post-O8 smoke protocol](../evidence/post-o8-benchmark-protocol-2026-11-07.md); no `docs/evidence/benchmark-opusplan-*.md` exists | Unproven: the A/B compared routed and control under a Sonnet session, not `opusplan` arms |

What Downshift does not do, by design ([when-to-use.md](when-to-use.md), [companions.md](companions.md)): it is not
a gateway and terminates no HTTP chat or completions traffic; it holds no provider keys; it proxies no request and
adds no markup; it never installs, launches, wraps or configures a companion (`downshift context enable headroom`
writes one field in its own file; an earlier `init` call was removed in #71); it does not rewrite tool results
(its compressor observes); it stores no prompt, task text or path. A companion's saving is
never counted as routing: `doctor` reports the companion so a before/after is not attributed to Downshift alone.

## Where the saving can come from

Nothing in this section is measured. It is the arithmetic of the ceiling, to read before comparing any percentage
above with another tool's.

**Routing has a structural ceiling.** It acts only on subagent spawns, only on the share of those a guardrail lets
through, and only by the price gap between two models. The saving on total spend is at most:

    subagent share of spend  x  price gap  x  routable share

Worked example, **hypothetical, not an observation**: if subagents are 30% of a session's spend, the price gap on the
tokens that move is 50% (Sonnet to Haiku on uncached work tokens; zero on cache reads), and 35% of spawns are routable
(b5 applied 19 of 56, b6 3 of 34), the ceiling is 0.30 x 0.50 x 0.35, about 5%. With a routable share of 10%, as in
b6, it falls under 2%. This is why b5 shows 12% on a task set built from delegation while natural use is estimated
near 3%: the first inflates the subagent share and the routable share, the second does not. The three factors are the
quantities worth logging, not their product.

**Output cutting acts elsewhere and multiplies.** A tool that shrinks command and tool output (a compression proxy
such as Headroom, an output wrapper such as `downshift compact`, or a third-party output-cutting tool) reduces the tokens the main session reads and
then re-reads from cache on every later turn: the part of spend routing cannot touch. The two levers act on different
tokens, so their savings compose roughly as (1 - r)(1 - o), not as a sum. Hypothetical: a 5% routing saving on top of
a 20% output saving takes spend to 0.95 x 0.80, a 24% total saving. The composition breaks if cutting changes
behaviour (more retries, more turns), which is part of what the arms of
[benchmark-output-cutting.md](benchmark-output-cutting.md) test.

**Status.** Directionally validated on 2026-10-21: Arm D of the four-arm battery ([`four-arm-mechanical-2026-10-21.md`](../evidence/four-arm-mechanical-2026-10-21.md))
combined routing with compactor wrapping (26.8% fewer tokens per accepted run), and KiroCrew real-use testing
demonstrated up to ~53% total token savings combining policy routing and shell advice compaction across 6 control tasks.
In both trials, output cutting and subagent routing acted as complementary multipliers, but broad everyday gains
remain unproven.

## Why catalog prices are not the invoice

Catalog prices are public list prices from a dated snapshot. An invoice differs fundamentally by pricing structure:
- **Subscription plans (Pro, Max, Team; Cursor Pro/Ultra):** Flat monthly fees with rolling 5-hour and weekly rate limits.
  There is no per-token line item or dollar invoice. What Downshift saves on subscriptions is **quota headroom**, extending
  active coding duration before throttling occurs.
- **Pay-as-you-go API billing:** Invoices incorporate provider-side minimums, cache-write overhead, retry spikes, and the
  parent session's own accumulating context window, which no client subagent hook can observe.
Downshift does not measure money: it reports tokens, and the production figures are tokens per completed task and rework for routed spawns (see [report.md](report.md)). The planned seven-day routed-versus-control comparison (issue #103, [beta-exit.md](../reference/beta-exit.md) P3.7) cannot run since 2026-11-02, when the control arm was removed.

## Why fewer tokens can cost more

A cheaper child that fails is paid for and then paid for again on the retry, usually on a larger model, plus the
parent's turns to notice, diagnose, and re-delegate. The outcome suite shows the risk size: the small tier passes 27 of 40 tasks
where the frontier passes 40 ([evaluation-suite-inventory.md](../evidence/evaluation-suite-inventory.md)). That is why the primary metric is per completed task including rework, why `report` adds
redone attempts to the completing child, and why an unconfident router that keeps the requested model (most spawns in
b2 and b6) is the intended behaviour rather than a missed saving. A token count or a per-decision percentage says what
a spawn cost, not what the task cost. Detailed reconciliation: [anthropic-references-benchmark-reconciliation-2026-11-07.md](../evidence/anthropic-references-benchmark-reconciliation-2026-11-07.md).
