<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Hardening audit, beta.14: defects fixed and what did not change (2026-10-15)

What this page answers: which defects an audit of the routing, adapter, telemetry and hook code found, what each fix changed, and
whether the classifier benchmark moved. The sources are the commits `f9b531e..429bf42`, the CHANGELOG section
`[0.1.0-beta.14]`, and the regression tests those commits added. This page summarises them; it does not replace them. Aggregates
only: no prompts, task text or paths from the private datasets are published here.

## What was audited and how

- **Scope.** Commits `f9b531e..429bf42` on `main`: 12 commits, 47 files, +1611/−59 lines. Nine are code commits (defect fixes, and one
  benchmark report change); three are documentation.
- **Auditors.** Independent auditors worked in isolated clones, so findings did not depend on one reading of the code. Their working
  notes are not in the repository. The commit messages, the tests and this page are the record.
- **Failing test before each fix.** The method was that every defect needed a failing regression test before its fix. Fix and test
  land in the same commit, so the history alone does not show the order. This audit therefore re-ran the new tests against the base
  commit `f9b531e`. Behavioural regression tests fail there for every row marked "fails on base" below (23 tests). Telemetry and
  benchmark tests do not compile on the base because they call symbols the fixes add, so they show no behavioural failure there and
  are marked "compile-red". The other tests that compile on the base are guards: they pass there too, and protect behaviour the fixes must keep.
- **Benchmark before and after.** The classifier benchmark was run with identical commands on the base build and the HEAD build,
  for the five splits. The output is identical apart from three new summary lines (confident coverage, accuracy when confident,
  abstentions). No split moved, so the benchmark shows no sign that the fixes were tuned on an evaluation split.

## Defects fixed

Fix commits are short shas. "Fails on base" means the named test fails at `f9b531e`.

| Area | Defect | Effect | Fix | Regression test |
|---|---|---|---|---|
| Routing safety | The risk floor read only the 32 KB classification head; Route's risk list read the full prompt | A risk signal past the cap let a trivial head write the small model on a known session | 2350b30 | `TestRiskSafety_SignalPastClassifyCapKeepsTaskOffSmall` (fails on base) |
| Routing safety | Upshift chose the session model by list position | On an unordered session list an upshift could write haiku over sonnet for a risky COMPLEX task; the risk rules do not fire on an upshift | fd02286 | `TestUpshiftTier_UpshiftNeverWritesLowerCatalogTier`, `TestUpshiftTier_ClaudeCodeUpshiftWritesSmallerModel`, `TestUpshiftTier_CursorUpshiftWritesSmallerModel` (all fail on base) |
| Routing safety | Score routing (opt-in) could pick a high-scoring small model for a mid-tier decision | A small model could be written for a decision the catalog tier put at mid | fd02286 | `TestScoreTier_ScoreRoutingNeverBelowDecisionTier` (fails on base) |
| Routing safety | Claude Code: a downshift on a baseline guessed from settings could write the mid tier | A model the session may already be below (for example `--model haiku`) became a hidden upshift | 664bbb2 | `TestHandle_SettingsBaselineHoldsMidTierDownshift` (fails on base); guard `TestHandle_SettingsBaselineStillWritesSmallTier` |
| Routing safety | Cursor: a downshift on the parent's `model_id` could write the mid tier | A child can run on Composer or a fast variant, so a mid target could be an upshift | 2eda5e5 | `TestHandle_ParentModelIDHoldsMidTierDownshift` (fails on base); guard `TestHandle_ParentModelIDStillWritesSmallTier` |
| Routing safety | Codex: a decision held by R1 or R8 still raised `reasoning_effort` | A held change reached the spawn | 664bbb2 | `TestHandle_HeldUpshiftLeavesSpawnUnchanged` (fails on base) |
| Routing safety | Codex: an unconfident decision raised `reasoning_effort` without a model rewrite | Effort was spent on doubt | 9d5a99c | `TestHandle_UnconfidentKeepsRequestedEffort` (fails on base) |
| Risk detection | `drop the users table` was not destructive (named table) | A destructive request was not treated as risky | 2350b30 | `TestRiskSafety_DropNamedTableIsDestructive` (fails on base) |
| Risk detection | Four Portuguese phrasings of covered risks were missed | The same risks were missed when written in PT-BR | 2350b30 | `TestRiskSafety_PortugueseParity` (fails on base) |
| Risk detection | Lock, atomic and race changes rode a confident rename to the small tier | New `concurrency` category, so these prompts are not trivial | 2350b30 | `TestRiskSafety_ConcurrencyIsNotTrivial` (fails on base) |
| Risk detection | Patterns too broad (bare `race` and `atomic`, `cobra` as CLI, course certificates, `base de produtos`) and too narrow (`sync.WaitGroup` never matched after identifier splitting; SQL `truncate` and `DELETE FROM`, PT table drops, key rotation and thread-safe not covered) | False positives raised cost; missed patterns were missed risks | 2eda5e5 | `TestRiskPatterns_StillRisky` (fails on base); `TestRiskPatterns_NoFalsePositives` passes on base, so it guards the narrowing and does not prove it |
| Adapters | Antigravity: an earlier child's rewrite made a held or unchanged last child log as `rewrite_emitted`; `to_model` was the catalog pick, not the written model | A record paired one child's task with another child's tier | 9d5a99c | `TestMultiChild_HeldLastChildIsNotReportedAsTheRewrite`, `TestMultiChild_DecisionModelMatchesWrittenModel`, `TestAntigravity_HeldLastChildNotLoggedAsRewrite` (all fail on base) |
| Adapters | KiroCrew: the respawn message named the decision tier, not the target's | The message could ask to trim context for a mid-tier model | 9d5a99c | `TestHandle_BlockMessageTierMatchesTarget` (fails on base) |
| Telemetry honesty | A refuted rewrite (`rewrite_honored=false` from a linked `resolved` record) still counted as applied, credited a saving and could be inferred honored from a later spawn | Savings and honored counts were overstated | 4087e44 | `TestAggregate_RefutedRewriteSavesNothing`, `TestCountInferredHonored_RefutedBeatsLaterSpawn` (compile-red) |
| Telemetry honesty | Usage priced on a baseline guessed from settings carried `baseline_source=requested` | An inferred saving looked requested | 4087e44 | `TestBuildUsageEvent_InferredBaselineSource` (fails on base); `TestPrintStats_MarksInferredBaseline` (compile-red) |
| Telemetry honesty | A refuted rewrite was priced on the model it asked for; the model mix, statusline, session tree and frontier-on-doubt count included the unrun model | Each view disagreed with the record | 0d8172b | `TestHonesty_RefutedRewritePricedOnRewriteTarget` (fails on base); `TestHonesty_ModelMixCountsRefutedRewriteTarget`, `TestHonesty_FrontierOnDoubtCountsRefutedRewrite`, `TestHonesty_StatuslineTopModelUsesRefutedTarget`, `TestHonesty_SessionTreeRowModelPrefersRefutedUsageModel`, `TestHonesty_RealSavedCreditsUsageOfRefutedRewrite` (compile-red) |
| Telemetry honesty | Inferred baselines were not marked in the session tree, child-end line or export; a spawn with neither an observed nor a requested model was priced in the control arm only | A saving that rests on a guess looked measured; the two report arms were priced differently | 0d8172b | `TestHonesty_ReportArmsPricedAsymmetrically` (fails on base); `TestHonesty_SessionTreeHidesInferredBaseline`, `TestHonesty_ChildEndLineHidesInferredBaseline`, `TestHonesty_ExportOmitsInferredBaselineCount` (compile-red) |
| Post hooks | The context-sensor lock (`flock` `LOCK_EX`) had no timeout and runs on every PostToolUse | A stuck holder stalled every hook until the harness timeout; the lock now gives up after 500 ms | c1c9938 | `TestPostHooks_PostToolUseBoundedWhenSensorLockHeld` (fails on base, after 3.2 s) |
| Post hooks | SubagentStop opened the transcript before checking that it is a regular file | A FIFO path blocked the hook; it now checks with Lstat first | c1c9938 | `TestPostHooks_SubagentStopFIFOTranscriptBounded` (fails on base, after 3.0 s) |

Added with the post-hook fix, as guards: `TestPostHooks_PostHooksFailOpen`, `TestPostHooks_PostHooksTelemetryPrivacy`,
`TestPostHooks_PostHooksNeutralInNoRouteAndControl`, `TestPostHooks_LinkedRecordsPrivacy`.

## Benchmark before and after

Classifier benchmark, `downshift benchmark <split>`, run on `f9b531e` and on `429bf42` with the same command. Figures are from the
HEAD build; the base build prints the same first lines.

| Split | Items | Complexity accuracy | Tier routing accuracy | Confident: items, tier right, accuracy when confident | Abstained (no signal) |
|---|--:|--:|--:|---|---|
| seed (`tasks.json`) | 200 | 73.5% | 73.5% | 122, 116 right, 95.1% | 78 (40) |
| holdout | 300 | 100.0% | 100.0% | 288, 288 right, 100.0% | 12 (0) |
| fresh | 100 | 52.0% | 53.0% | 37, 35 right, 94.6% | 63 (58) |
| heldout2 | 120 | 40.8% | 41.7% | 28, 22 right, 78.6% | 92 (89) |
| blind-vitrine | 60 | 46.7% | 46.7% | 23, 15 right, 65.2% | 37 (28) |

- **Identical before and after.** For each split the base and HEAD outputs differ only by the three new summary lines. The
  per-item classifier output is the same.
- **Gates.** `downshift benchmark --gate` exits 0 on the seed and holdout splits.
- **Abstention split.** An abstention is an unconfident answer, and the router keeps the requested model for it. The report now
  counts it apart from errors (`a38bd71`, `TestAbstentionSplit_SeparatesAbstentionFromError`, compile-red on base). Accuracy
  figures and gates did not change. Confident answers that are wrong: 2 of 37 (fresh), 6 of 28 (heldout2), 8 of 23 (blind-vitrine).
- **Classifier-level unsafe rates.** These are counted over all items, including abstentions, so they are not what routing does.
  FRONTIER to MID: 44% of frontier items on fresh (11 of 25), 60% on heldout2 (18 of 30), 35% on blind-vitrine (7 of 20); 0% on the
  seed and holdout. FRONTIER to SMALL was 0% on every split.
- **Earlier published figures.** `benchmark/report.md` publishes fresh 50.0% and heldout2 40.0% at classifier commit `5a888e7`.
  Both base and HEAD in this range give 52.0% and 40.8%, so that change came before this range. This page does not attribute it.
- **Outcome suite.** `downshift eval-outcome --verify` (explicit `--tasks` and `--runs` paths in the labs repository) reports
  40 tasks verified: the stub fails, the reference passes, and the recorded results reproduce. It checks reproduction, not pass rates.

## Performance note

Measured in-process with `Route(prompt, "claude-code", "claude-sonnet-5-5")` (no resolver), Go test in `internal/core`, five runs per
case, medians, one macOS machine. The adversarial input repeats `fooBarBazQux ` (camelCase filler); the typical input repeats one
ordinary sentence.

| Input | `f9b531e` (base) | `429bf42` (HEAD) |
|---|--:|--:|
| short prompt (about 50 bytes) | 0.25 ms | 0.10 ms |
| typical 1 MiB | 0.93 s | 0.23 s |
| adversarial 1 MiB (camelCase filler) | 3.18 s | 0.16 s |

Intermediate commits on the adversarial input: 2350b30 about 3.7 s and fd02286 about 3.4 s; 2eda5e5 (one memoised risk scan per
prompt, narrower patterns) brought it to about 0.15 s. In this range Route got faster on large prompts, not slower. Short prompts
are below 0.3 ms at every commit.

Limits. The figure is `Route` only. It excludes process start, reading the hook payload (up to 1 MiB), JSON parsing and hook I/O.
The regex work runs in-process and is not cut off by any hook budget, and `Route` has no internal time limit. One adversarial family
was measured, not a search for the worst input.

## On main after beta.14

Not in the beta.14 release; listed in the CHANGELOG under Unreleased.

- **Transcript wait once** (292ce5a, found by the beta.14 smoke test of 52 stdin hook checks across five harnesses, all passing):
  with an empty transcript the 400 ms wait for the session model ran twice per Claude Code call; it now runs once
  (`TestSessionModelFromTranscript_WaitsOncePerPath`).
- **Guessed-baseline holds explained** (292ce5a): a downshift held because its baseline was guessed now carries
  `HELD_GUESSED_BASELINE` instead of a bare `allow`.
- **Risk scan prefilter** (3067340): each risk category runs its regex only when the case-folded text holds a literal every match
  of that category needs, and identifier splitting no longer uses a regex. A differential test against the previous scan (about
  30k prompts, long prompts around the 32 KB cap, every rune up to U+1FFFF for case folding) keeps the categories identical, and
  the benchmark output is identical. With the catalog resolver and a cold memo per call, `Route` on 1 MiB went from about 2.9 s to
  78 ms (adversarial) and from 1.5 s to 275 ms (benign sentence filler); this is a different harness from the table above, so the
  two sets of figures are not comparable row by row.

## Remaining gaps and design questions

- **Upshift is not the cheapest.** An upshift picks the highest catalog tier at or above the current model's tier (later list
  position first within a tier). That is safe, since it never goes down, but it can spend more than the needed tier. Whether it
  should pick the cheapest model at the needed tier is open.
- **A confident decision with no baseline may write any tier.** This is the designed behaviour for confident calls. The rules for
  unconfident calls with no baseline are in [harness-matrix.md](../reference/harness-matrix.md).
- **Claude Code upshift on a settings-inferred baseline is still written.** The small-tier-only limit applies to downshifts on a guessed
  baseline, not to upshifts.
- **Export writes a zero for cost that was never measured.** `real_saved_usd` is always present, so 0 reads like a measured zero. Check
  `real_cost_events` first.
- **`context-sensor.json` is never pruned.** Entries are added per session hash and removed by nothing.
- **Confident wrong answers remain.** See the benchmark section (8 of 23 on blind-vitrine).
- **The benchmark scores the classifier only.** Router, guardrails and quota gates are not in these numbers.
- **Recorded outcome runs are below the threshold.** The outcome suite's recorded runs (2026-10-03, before this range) give small 27 of
  40 (67.5%) against frontier 40 of 40. On the TRIVIAL and SIMPLE labels, small is 80.0% against 100.0% for frontier (20 tasks, with
  overlapping 95% intervals). The suite's README says to revisit the downshift when the gap is below -5 points, and the TRIVIAL and
  SIMPLE gap is -20.0. The fixes in this range were not re-run against these tasks.

## Not verified

- No live harness session was run for these fixes. The tests use fake hook payloads.
- Whether Cursor 3.2x fills `model_id` on a Task `preToolUse` payload. The Cursor baseline fix assumes the field may be set.
- Real-traffic false-positive rates of the new risk patterns. The false-positive test is a fixed list of prompts.
- The auditors' working process is not in the repository (see the method section).
- Performance on input families other than the two measured.

## Reproducing

```bash
git log --reverse f9b531e..429bf42
go test ./...
go build -o ds ./cmd/downshift
./ds benchmark eval/benchmark/tasks.json        # repeat per split; add --gate for the gates
./ds eval-outcome --verify --tasks eval/outcomes/testdata --runs eval/outcomes/runs
```

The benchmark splits and outcome suite are in the private downshift-labs repository. The public summary is
[benchmark/report.md](../../benchmark/report.md). The release notes are in [CHANGELOG.md](../../CHANGELOG.md). The earlier
routed-versus-control measurement from the same day is in [subagent-routing-ab-2026-10-15.md](subagent-routing-ab-2026-10-15.md).
