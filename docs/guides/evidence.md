<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Evidence ladder: what each saving figure means

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Five kinds of "saving" appear around this project, and they are not the same number. This page separates them, quotes
each figure from the file that holds it (checked against the current file on 2026-10-17), and says which rung the
project stands on today. Rule of reading: a figure on a lower rung never stands in for a higher one.

| Rung | What it is | Status today |
|---|---|---|
| A | Theoretical saving from catalog list prices | Arithmetic, always available |
| B | Saving estimated by telemetry (normalised units, catalog-priced tokens) | Available on Claude Code; an estimate per decision |
| C | Saving measured in controlled benchmarks | Partial: quality measured, cost not |
| D | Saving demonstrated on real tasks, routed versus control | One measurement, one operator, 2026-10-15 |
| E | Cost per task completed with acceptable quality | Defined and computed by `report`; no window has unlocked it yet |

## A. Theoretical saving (list prices)

Source: `internal/catalog/catalog.json` (`version` 2, `updated` 2026-10-16), USD per 1M tokens.

| Model | Input | Output | Cache read | Cache write |
|---|--:|--:|--:|--:|
| `claude-haiku-4-5` (small, the native `haiku` alias) | 1.00 | 5.00 | 0.10 | 1.25 |
| `claude-sonnet-5-5` (mid) | 2.00 | 10.00 | 0.10 | 2.50 |
| `claude-opus-5-5` (frontier) | 4.00 | 20.00 | 0.20 | 5.00 |

A Haiku child costs 75% less than Opus and 50% less than Sonnet on uncached input and output tokens. On cache reads the
gap to Opus is 50% and the gap to Sonnet is zero. A subagent's traffic is mostly cached context (the one transcript
checked in [beta-exit.md](../reference/beta-exit.md#pillar-3--billing-beforeafter-real-dollars) had 18 input, 311
output and 82,900 cache tokens), so "75%" is the ceiling of one token class, not the saving on a child. Rung A also
assumes the same tokens on both models and a task that passes on the cheaper one; both assumptions are what rungs C
to E test.

## B. Saving estimated by telemetry

Source: `internal/telemetry/telemetry.go` (`Aggregate`), `export.go`, [stats-export.md](../reference/stats-export.md).

- **Normalised units** (`norm_baseline`, `norm_routed`): every decision is 1.0 baseline unit; an applied downshift is
  charged `1 - estimated_savings` (a list-price ratio), an applied upshift `upshift_cost_ratio`, everything else 1.0.
  No tokens are involved. `--cost-per-unit` turns units into dollars by multiplication, nothing more.
- **Catalog-priced tokens** (`actual_cost_usd`, `real_saved_usd`, `real_cost_delta_usd`, `real_overspend_usd`): the
  child's recorded tokens priced at the executed model and again at the requested model. The baseline is the same
  token count on another model, a counterfactual, not a second run. `real_saved_usd` clamps each event at zero;
  read it with `real_overspend_usd`. Tokens come from the child transcript on `SubagentStop` (Claude Code only).
- **Provenance.** [billing-gap-audit.md](../evidence/billing-gap-audit.md) (2026-10-09, 7 days): `total` 186,
  `usage_records` 15, `usage_linked` 5, `actual_cost_usd` 15.432977, `real_saved_usd` 0.424866,
  `baseline_no_route` 0; native tokens and model matched the record on 5 of 5 linked children. That is provenance
  for the token counts, not for a bill. P3.4 needs 50 linked records and stands at 5 in that audit.
- **Tokens rerouted, and why "tokens saved" is not a claim.** Routing does not reduce tokens: the child reads and
  writes about the same tokens on another model, so the dollar saving is those tokens priced on the requested model
  minus the same tokens priced on the model that ran. "Tokens rerouted" counts the tokens of usage records linked to an
  applied, non-refuted rewrite, by harness and requested-to-ran model pair, split into moved to a cheaper model and
  moved to a pricier one. It is rung B evidence: the counts are observed (Claude Code only today, because it is the only
  harness that records child tokens), the dollars are estimated at list price, and the baseline stays a counterfactual.
  It carries the same biases as the dollar figures below: a refuted rewrite, an unknown requested model or a decision
  with no usage record contributes nothing. Downshift does not claim fewer tokens: no run without routing exists to
  subtract from, a cheaper model can need more tokens or a retry, and on a plan with no per-token bill (Claude Code Pro,
  Max, Team; Cursor plans) what was saved is not tokens or dollars but headroom against a quota, which no hook sees.
- **"Tokens saved" in the live views means saved from the requested model** (owner decision 2026-10-19). The session
  tree, `session --panel` (savvy-progress), the statusline, `watch`, `dsmon`, the child-end line and the `stats`
  headline show the share of child tokens that ran on a cheaper model than the one the harness asked for: the
  "cheaper" bucket above over every priced child (control, held, kept and refuted children count in the total, never
  as saved), per agent, per agent type and per session (`telemetry.TokenSavings`). Work tokens (input+output) and cache
  tokens (read+write) are counted apart, because cache reads are most of a child's tokens and carry the smallest price
  gap: Sonnet to Haiku saves nothing on them, so one share would overstate the value. These views carry no dollar;
  `downshift report` prices the tokens at report time (API lens) and states the share again in tokens (plan lens). The
  share is a count of where tokens ran, not a reduction in tokens and not a dollar saving.
  Per harness, what is observed and what blocks the rest: [value-per-harness.md](../reference/value-per-harness.md).
- **Known biases.** `baseline_source: inferred` (Claude Code settings guess) is counted apart; a refuted rewrite saves
  nothing; an unknown requested model gives zero saving by construction; a decision with no usage record gives units
  only. The headline percentage this produces (19.3% in one operator's log, per
  [subagent-routing-ab-2026-10-15.md](../evidence/subagent-routing-ab-2026-10-15.md)) is an estimate per decision at
  list price, not a measurement.

## C. Saving measured in controlled benchmarks

Sources: [benchmark/report.md](../../benchmark/report.md) (last updated 2026-10-15),
[hardening-audit-2026-10-15.md](../evidence/hardening-audit-2026-10-15.md). Datasets live in the private downshift-labs
repository; the public repo holds aggregates and two sample tasks under `internal/outcome/testdata/`.

| Benchmark | n | Held constant | Result | Cost measured? |
|---|--:|---|---|---|
| Classifier, seed regression net | 200 | prompts, labels, `ClassifyWithSemantic`, no adapt memory | 73.5% tier accuracy (95% CI 67.0–79.1%) from `236c057` (2026-10-12) onward, measured again at `2c8f3f4`; the 69.0% that `benchmark/report.md` carried until 2026-10-17 was the classifier at `be1054ac` (2026-10-06), resolved in [baseline-freeze-2026-10-17.md](../evidence/baseline-freeze-2026-10-17.md) | No |
| Classifier, burned holdout | 300 | same | 100.0%; tuned against, so not generalisation | No |
| Classifier, eval-only `fresh` / `heldout2` | 100 / 120 | same, split never tuned | 50.0% (40.0–60.0%) / 40.0% (31.7–49.2%) at `5a888e7`; 53.0% / 41.7% at `3067340`. Most misses are abstentions (router keeps the model); confident and wrong: 2 of 37 and 6 of 28 | No |
| Outcome suite, fixed model per tier | 40 tasks, 1 run per tier | prompt, hidden `check_test.go`, one model per tier, 2026-10-03 | frontier `anthropic/claude-opus-5.5` 40/40; mid `deepseek/deepseek-v4-flash` 31/40 (77.5%); small `qwen/qwen3-coder-30b-a3b-instruct` 27/40 (67.5%). TRIVIAL+SIMPLE: 20/20, 17/20, 16/20 | No: `run.json` holds tier, model, date and pass/fail only (issue #100) |
| Classifier routing on that suite | 40 | recomputed 2026-10-15 | 6 small, 22 mid, 12 frontier; 37/40 pass at the picked tier | No |
| Adapt memory, offline | 40 | same runs | per-task upper bound 40/40; shared shape memory 37/40 (0.0pp vs classifier) | No |
| Held-out suite (GAP-SUITE) | 6 tasks, 1 smoke run | Go tasks outside the development suite | small 2/6, mid 4/6, frontier 6/6, USD 0.14 total ([gap-tasks.md](../internal/gap-tasks.md)) | Total only, not per task |

Two notes. The small-versus-frontier gap on TRIVIAL+SIMPLE labels is −20.0 points on fixed-model runs, with
overlapping intervals; the suite's own rule is to revisit the downshift below −5. And `downshift eval-adapt` prints a
"cost per completed task" per policy, but it is a model: reference tokens per attempt (`--ref-in`, `--ref-out`), tier
prices you pass, escalation one tier up on failure, an optional `--fail-penalty`. It uses no observed tokens.

## D. Saving demonstrated on real tasks

Source: [subagent-routing-ab-2026-10-15.md](../evidence/subagent-routing-ab-2026-10-15.md). One operator, Claude
Code on `sonnet-5-5`, same prompts in both arms, control = same hooks with `DOWNSHIFT_NO_ROUTE=1`, acceptance = exit
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

What D supports: a saving of about 12% on mechanical delegated work, a third on pure renames, with 12/12 accepted
in both arms. What it does not: any figure for natural use (b2 and b6 gaps are not attributable to routing),
intervals (three runs per cell), quality beyond structural checks, other harnesses, more than one operator.

## E. Cost per task completed with acceptable quality

**Not measured today.** The definition exists and the command computes it, but no window has met its threshold.

`downshift report` (`cmd/downshift/report.go`, `internal/telemetry/report.go`, `tasks.go`) defines a completed task as a
child that ended `completed` and has a priced usage record; cost per completed task is that child's cost plus every
failed attempt it redid (a respawn in the same session within 10 minutes of an `error` or `interrupted` end); rework
rate is reworked spawns over spawns with an observed end; savings are control minus routed per completed task, with a
95% percentile bootstrap (2,000 resamples, fixed seed 20261013). It needs 20 completed, priced tasks per group
(`MinReportTasks`); at the default `DOWNSHIFT_CONTROL_SHARE=0.10` that is roughly 200 completed tasks.

What is missing to produce it: enough completed tasks in one log (the [pilot](pilot.md) at share 0.5); a captured
`PostToolUseFailure` payload, without which rework undercounts (GAP-CHILD-FAILURE); and a quality judgment, because
"acceptable quality" is not in the log: `completed` means the child returned, not that its work was right.
`downshift feedback <id> failed|retry` is the manual backstop. The 2026-10-15 A/B measured cost per run with an
acceptance check, which is close to E for those tasks but is not the `report` definition and had no rework term.

## Metrics table

Status: **available** (recorded and printed), **inferred** (derived from indirect evidence), **estimated** (a price
model applied to something recorded), **not available** today. Claude Code unless stated; other harnesses record
decisions but not usage or ends ([harness-matrix.md](../reference/harness-matrix.md#child-end-n5-2026-10-13)).

| Metric | Status | Where |
|---|---|---|
| **Total cost per task completed with acceptable quality** (primary) | not available | `downshift report` computes cost per completed task once 20 priced completions per group exist; quality is manual |
| Cost per run (parent session) | not available from Downshift | hooks see subagents only; the A/B used Claude Code's `total_cost_usd` by hand |
| Cost per subagent | estimated | `downshift session --tree`, `stats --detail` (`cost_subagents`, `max_subagent_cost_usd`); catalog price × transcript tokens |
| Input / output / cache tokens per child | available | usage event fields `input_tokens`, `output_tokens`, `cached_tokens`, `cache_write_tokens`; `observed_child_tokens` in `stats --export` |
| Latency, total and per step | not available | no duration field on decision, usage or end events (`latency_ms` exists only on the E5 shadow record); the A/B timed runs externally |
| Completion rate | available | `child_status` counts in `stats` ("How children ended") and `--export`; `report` `ended` / `completed_tasks` |
| Failure rate | available, undercounted | `child_status` `error` / `interrupted`; `PostToolUseFailure` payload never captured (GAP-CHILD-FAILURE) |
| Retries | inferred | "Respawned after a failure" in `stats`, `↻` in the session tree (10-minute rule, a hint); explicit via `downshift feedback <id> retry` |
| Result quality | not available | hidden tests in the A/B only; `feedback <id> success|failed` is manual |
| Coordination and delegation cost | not available | parent turns and tokens are outside the hook; GAP-COORDINATION open |
| Fixed-context cost | available per child, estimated in dollars | `cached_tokens` / `cache_write_tokens` priced at cache rates; parent context not seen; `context metrics` token fields unavailable |
| Abstention rate | available | reason codes `NO_SIGNAL`, `CLASSIFIER_UNCONFIDENT` in `stats --export` `reason_codes`; `downshift benchmark` prints abstentions per split |
| Routing decisions and guardrails fired | available | `verdict`, `corrections`, `HELD_*` codes; `stats` "Guardrails held"; `session --why`; `stats --by-harness` |
| Requested versus executed model | available / inferred | `requested_model` and `final_model` on the decision; `resolved` record with `rewrite_honored` when the harness reports the child's model; otherwise `rewrite_honored_inferred` from a later spawn |
| Baseline comparison | available (control) / estimated (counterfactual) | `report` routed versus `CONTROL` spawns; `stats` same-token counterfactual, marked `baseline_source: inferred` when guessed |

## Three saving scenarios and what each must prove

The objective is one sentence: minimise cost per task completed correctly, with safety and developer experience
preserved. It is never "pick the cheapest". The guardrails in `internal/core/selfcheck.go` (`Check`) encode that:
R1 holds an unconfident downshift, R2 any downshift of a COMPLEX task, R5 any downshift of a task that touches a risk
category (the risk floor, `Guards.RiskFloor`), R8 an unconfident upshift, R7 an unconfident frontier pick with no
known current model, R9 the same for mid. Only a deterministic lift (`deterministicLift`: risk floor, risk
abstention, graph escalation, an opted-in NB upshift, an operator tier floor) may move a task up. Doubt never spends
money, in either direction (`TestCostTruthTable`, `TestCostUnconfidentUpshiftNeverRewritesAbove` in
`internal/core/cost_safety_invariants_test.go`). A scenario below counts as proven only when its metric is measured
with those guardrails on; a saving bought by removing them is a different product.

| Scenario | Must prove | What acts in Downshift | Which metric answers it | Status today |
|---|---|---|---|---|
| 1. Same provider, smaller model (an Opus or Sonnet child to Haiku) | Cost down without relevant quality loss | `Route` in `internal/core/policy.go`: `Complexity.Tier()` picks the tier, `compareToCurrentModel` returns `VerdictDownshift` with `SavingsRatio` from list prices; `Check` runs before any adapter acts; `Plan(caps, resolver)` in `capabilities.go` turns the decision into a wire rewrite inside the session allowlist, with the catalog `native_name` (`haiku`) where the harness validates names | `downshift report` cost per completed task including rework (rung E); `downshift stats` `real_saved_usd` with `real_overspend_usd` (rung B, same-token counterfactual); acceptance plus `total_cost_usd` per run in the A/B method (rung D); the arms of [benchmark-protocol.md](benchmark-protocol.md) | **Measured, narrowly.** b5: $0.59 against $0.67 per run (−12%), 12/12 accepted in both arms, 19 of 56 spawns to Haiku; `mech-rename` −33% ([subagent-routing-ab-2026-10-15.md](../evidence/subagent-routing-ab-2026-10-15.md)). Not measured: natural use (b2 applied 0 downshifts, b6 3 of 34), quality beyond structural checks, intervals, rung E (no window has 20 priced completions per group). The quality risk is sized by the outcome suite, small 27/40 against frontier 40/40 (`benchmark/report.md`) |
| 2. Same model, lower reasoning effort | Total saving without more retries and rework | `EffortFor(tier)` in `internal/core/models.go` (small → low, mid → medium, frontier → high), set by `Route` and `RaiseTier`; `IntentFor` in `escalation.go` names the intent (TrivialIntent small/low, ReviewIntent frontier/high, PreservedIntent keeps an `explicit_only` model); the catalog's `EffortFor` maps the level to the harness string through `effort_map`, and `ConstrainEffort` / `AllowedEfforts` keep it to values verified for the exact id. Only Codex has `CanApplyEffort` (`capabilities.go`); its adapter writes effort on a confident unheld decision or alongside a model rewrite and never lowers it on doubt (`effortAllowed` in `internal/adapters/codex/codex.go`; `TestHandle_HeldDownshiftKeepsRequestedEffort`, `TestHandle_UnconfidentOKDoesNotLowerEffort`, `TestHandle_UnconfidentKeepsRequestedEffort`). Claude Code, Cursor, Antigravity and KiroCrew have no effort field; Grok is config only | Would be `report` cost per completed task and rework rate on Codex. Codex has `UsageSource: none` and an unverified end parser, so neither term is recorded. `stats` prices tokens by model only (`CostUSD` in `internal/telemetry/cost.go`); effort has no price in the catalog, so an effort-only change counts as 1.0 unit and $0 saved | **Not measured.** Emission only: a Codex replay on 2026-10-09 emitted `reasoning_effort: low` with `gpt-6-luna` ([native-quota-routing-2026-10-09.md](../evidence/native-quota-routing-2026-10-09.md)); honored is inferred for the model (2026-10-02) and not observed for effort. 15 Codex spawn inputs checked on 2026-10-09 carried no `reasoning_effort` key ([closeout-recheck-2026-10-09.md](../evidence/closeout-recheck-2026-10-09.md)), so the baseline effort is usually unknown. No retry or rework figure exists for any effort change |
| 3. Cross-provider routing (OpenRouter ids, catalog family prefixes) | Net gain across price, latency, limits, compatibility and governance | `LookupByID` in `internal/catalog/catalog.go` strips the `provider/` prefix (`normaliseModelID`) and resolves exact → alias → family prefix, so `anthropic/claude-opus-4-8` or `cc/claude-opus-4-8` gives a baseline tier (`TestLookupByID_OpenRouter*` and the other prefix-normalisation tests in `catalog_test.go`); an id outside the catalog stays unknown and the spawn is left unchanged. The target still comes from the calling harness's catalog and session list (`ModelFor`; R4 `RuleForeignModel`; `TestPlanForSession_ForeignIDDoesNotRewrite`). Only the Cursor catalog mixes providers across tiers (anthropic, cursor, google, openai, xai, zhipu); the others are single-provider, Claude Code apart from one `grok-4.6` entry. Limits: the quota gate skips exhausted models (`Session.QuotaStatus`, `CreditHeld`; `TestPlanForSession_PrefersIncludedAndSkipsExhausted`). Compatibility: `native_name`, `StrictModelName`, `AllowedEfforts`. Governance: `explicit_only`, the project tier floor | Price: `stats` `real_saved_usd`, which needs both models priced (`HasModelPrices`; OpenRouter snapshot 2026-10-12 in the catalog, `gpt-6.1-sol` priced since #190, 2026-10-19). Latency: no duration field on any event (metrics table above). Limits: `downshift quota`, `QuotaStatus` on the decision. Compatibility: rewrite honored per harness ([harness-matrix.md](../reference/harness-matrix.md)). Governance: `policy_floor` and `HELD_*` counts in `stats --export` | **Not measured.** The lookup is pinned by tests; no live gateway has been verified; no A/B arm routed across providers. The A/B's only other-provider figure, Codex without Downshift (8/8 accepted, about $0.21 per run), is a different harness, not a routed comparison. Latency has never been measured. The fixed-model outcome suite ran OpenRouter ids (`deepseek/deepseek-v4-flash` 31/40, `qwen/qwen3-coder-30b-a3b-instruct` 27/40, 2026-10-03): quality only, no cost per task |

Scenario 1 has a protocol: arms B and C of [benchmark-protocol.md](benchmark-protocol.md) and the
[pilot](pilot.md). Scenarios 2 and 3 have none yet. Scenario 2 needs Codex usage and child ends before `report` can
say anything; scenario 3 needs a per-spawn duration field and an arm whose routed and control spawns run on
different providers. Until then the honest status for both is the last column.

## Where Downshift adds value beyond native opusplan and gateways

Three things sit near Downshift. Claude Code's native `opusplan` ([claude-code.md](claude-code.md), section 1) is a
main-session policy: Opus while plan mode is on, Sonnet once the plan is approved; it makes no per-spawn decision,
and a subagent inherits whatever the session runs. A gateway (LiteLLM, Portkey and the like,
[when-to-use.md](when-to-use.md)) owns transport, keys, retries and fallbacks and sees each request after the harness
has named a model. "Shown by" names a test or an evidence file; a claim with neither is marked unproven.

| Claim | Against | Shown by | Status |
|---|---|---|---|
| Acts at delegation time, per spawn, from the task text, before the request leaves the harness | `opusplan` decides once per session mode; a gateway decides per request, after the harness chose | `PreToolUse` on `Agent\|Task` in `internal/adapters/claudecode/claudecode.go`, shared `runHookAdapter` in `cmd/downshift/hookrun.go`, `cmd/downshift/hook_e2e_test.go`; A/B b5: 19 of 56 spawns rewritten under a Sonnet session | Shown |
| Harness-honest observation: emitted, honored and used are three records (`rewrite_emitted`; `resolved` with `rewrite_honored`; `usage`), and a refuted rewrite saves nothing | A gateway sees the model named in each request, not which spawn it belongs to or whether the harness applied a rewrite | [claude-code-rewrite-honored-2026-10-06.md](../evidence/claude-code-rewrite-honored-2026-10-06.md) (two spawns); [billing-gap-audit.md](../evidence/billing-gap-audit.md) (5 of 5 linked records matched); `TestAggregate_RefutedRewriteSavesNothing`, `TestCountInferredHonored_RefutedBeatsLaterSpawn` in `internal/telemetry/refuted_inferred_test.go` | Shown on Claude Code. Codex honored is inferred (2026-10-02), Cursor unconfirmed (2026-10-10); other harnesses record decisions only ([harness-matrix.md](../reference/harness-matrix.md)) |
| Per-spawn guardrails: R1–R9, risk floor, `explicit_only`, quota gate; a held decision changes nothing | `opusplan` has none; gateway fallbacks fire on errors and quotas, not on task risk or classifier confidence | `internal/core/selfcheck_test.go`, `cost_safety_invariants_test.go`, `risk_downshift_test.go`; [hardening-audit-2026-10-15.md](../evidence/hardening-audit-2026-10-15.md) (23 regression tests fail on the base commit); A/B b6: 2 upshift and 1 downshift verdicts held | Shown |
| Project tier floors: `.downshift/policy.json` by class and by risk, raise-only, applied by the shared runner for every harness | Neither `opusplan` nor a gateway reads a per-project floor from the repository | `internal/policyfile/policyfile_test.go`, `TestHookE2E_ProjectFloorThroughSharedRunner` (`cmd/downshift/policy_floor_e2e_test.go`), `TestRunKiroCrewHook_ProjectFloorApplies`; counted as `policy_floor` in `stats --export` | Shown by tests; no dated evidence file records a floor firing on a real project |
| Control group and report: about 1 in 10 spawns left unrouted by hashing the routing id, `downshift report` compares cost per completed task with a bootstrap interval | A session policy has no control arm; whether a given gateway offers a per-task routed-versus-control comparison is outside this repository's evidence | `cmd/downshift/control_group_test.go`, `TestInControlGroup_ShareAccuracy`, `internal/telemetry/report_test.go`; [report.md](report.md) | Mechanism shown; a saving from it unproven: no window has reached 20 priced completions per group (rung E) |
| Composes with `opusplan`: a spawn in plan mode inherits Opus and is routed as usual | Replaces nothing in the main session | Code path only: `configuredModel` resolves `opusplan` by `permission_mode` ([claude-code.md](claude-code.md), section 2) | Unproven: a live plan-mode spawn has not been observed |
| Saves beyond what `opusplan` alone saves | The B−C comparison | Protocol only: [benchmark-protocol.md](benchmark-protocol.md); no `docs/evidence/benchmark-opusplan-*.md` exists | Unproven: the A/B compared routed and control under a Sonnet session, not `opusplan` arms |

What Downshift does not do, by design ([when-to-use.md](when-to-use.md), [companions.md](companions.md)): it is not
a gateway and terminates no HTTP chat or completions traffic; it holds no provider keys; it proxies no request and
adds no markup; it never installs, launches, wraps or configures a companion (`downshift context enable headroom`
writes one field in its own file; the earlier RTK `init` call was removed in #71); it does not rewrite tool results
(its compressor observes); it stores no prompt, task text or path. The only network call on the hook path is the
optional, at-most-daily refresh of public capability scores, off with `DOWNSHIFT_SCORES=off`. A companion's saving is
never counted as routing: `doctor` reports the companion so a before/after is not attributed to Downshift alone.

## Why catalog prices are not the invoice

Catalog prices are public list prices from a dated snapshot. An invoice differs by plan (Pro, Max and Team have no
per-token line at all), negotiated or batch discounts, cache pricing the catalog may lack (then cached tokens are
billed at the input rate, an upper bound), provider-side retries and the parent session's own tokens, which no hook
sees. The only invoice-grade comparison is the [billing comparison template](billing-comparison-template.md), still
unfilled; see [pilot.md](pilot.md) for the protocol that feeds it.

## Why fewer tokens can cost more

A cheaper child that fails is paid for and then paid for again on the retry, usually on a larger model, plus the
parent's turns to notice and re-delegate. The outcome suite shows the risk size: the small tier passes 27 of 40 tasks
where the frontier passes 40. That is why the primary metric is per completed task including rework, why `report` adds
redone attempts to the completing child, and why an unconfident router that keeps the requested model (most spawns in
b2 and b6) is the intended behaviour rather than a missed saving. A token count or a per-decision percentage says what
a spawn cost, not what the task cost.
