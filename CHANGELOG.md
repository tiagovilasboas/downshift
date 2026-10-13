<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **Silent overspend on upshift.** The "doubt must not spend money" gate in `ShouldRewriteModel` had no production caller, so an unconfident UPSHIFT on a known session list was written (a Haiku parent on `"help"` went to Opus). Guardrail R8 (`R8_UNCONFIDENT_UPSHIFT`) now holds it in `Check`; R7 (`R7_UNCONFIDENT_UNKNOWN_FRONTIER`) holds an unconfident frontier recommendation when no current model is known. Deterministic lifts (risk floor, graph escalation, opted-in NB upshift) stay writable. See `docs/session-models.md`.
- **Semantic boost over-classified to COMPLEX.** The hash-embedder boost lifted prompts with no regex signal to the frontier tier (the word "hook", web research, 8 of 40 trivial eval tasks). The boost is now ignored for no-signal prompts and can never produce COMPLEX without regex evidence. On the recorded outcome runs the pinned classifier floor moves from 38/40 to 37/40; the pin is explicit in `internal/adapt/outcome_eval_test.go`.
- Hard Portuguese work no longer sits on MEDIUM: PT-BR mirror of the COMPLEX vocabulary (arquitetura, migração, redesenho, auditoria de segurança, sem downtime) and of the TRIVIAL/SIMPLE vocabulary, plus PT-BR risk terms (pagamento, reembolso, estorno, segredo, criptografia).

### Added

- **Session view (U3).** `downshift session [--tree|--why|--pause|--resume] [--session=<id>] [--json]` shows the session's subagents as an aligned tree (model, status, cost, short reason; a loss reads `over $x`, a redo `↻`), explains the latest decision in plain words, and pauses routing for one session (control-group behaviour, reason `PAUSED`, 12 h expiry). Claude Code also gets the `/downshift` command (`.claude/commands/downshift.md`). New capability `HarnessCapabilities.SessionEnv`. See `docs/session-view.md`.
- **Public capability scores (shadow by default).** `downshift models scores` (`--refresh`, `--status`, `--quiet`, `--json`) keeps a local `scores.json` of public scores for the enabled session models of every harness: Epoch AI ECI (CC BY 4.0) and the Artificial Analysis index via OpenRouter (local cache only, never in `catalog.json`). Install, doctor and a hook trigger refresh it at most once a day, in a detached process that never blocks a hook. `DOWNSHIFT_SCORES=off` disables all network. The capability pick is recorded as `score_would`, `score_differs` and `score_would_cost_ratio`; it changes the routed model only with `DOWNSHIFT_SCORE_ROUTING=1` (reason code `SCORE_ROUTED`). See `docs/scores.md`.
- **Inferred session (Claude Code).** With no hook list, native export, discovery cache or operator file, the session list is inferred from the current model: the catalog tier ladder below it, ending at the model itself. Downshift can descend; upshift never passes the current model. New capability `HarnessCapabilities.InferSession` (on for Claude Code only); `DOWNSHIFT_INFER_SESSION=off` disables it. `RewritePlan.Session` carries the planned list to the write guard.
- **Semantic-boost provenance.** `Decision.SemanticLift` and the reason code `SEMANTIC_BOOST` mark a tier raised by the embedder, not a rule. `downshift stats` prints `Frontier decisions N (on doubt, no deterministic lift: M)`, the semantic-boost share, and a WARNING when M is above zero.
- `docs/harness-matrix.md`: what each harness exposes for session discovery and quota (hook payload, model listing, quota source, session-start hook).
- Tests: 178-prompt EN/PT-BR corpus, guardrail truth table, cost sweep, adversarial semantic cases, 63 PT-BR cases, provenance tests.

### Changed

- Guardrail R9 (`R9_UNCONFIDENT_UNKNOWN_MID`): with no known current model and an unconfident classification, a mid-tier recommendation is no longer written (it could be an unnecessary upshift). Confident results and deterministic lifts (risk floor, graph escalation, NB upshift, operator tier floor) still apply. Previously an unrecognised trivial prompt fell to MEDIUM and was written as Sonnet.

- `downshift help` now shows only the commands a user needs, grouped by task; `downshift help --all` lists every command, including maintainer tools. No command was renamed or removed.

### Added

- Decision trace on every routing event: `routing_policy` (`deterministic-v1`) and `reason_codes`, sorted stable codes for the signals and guardrails behind the decision (for example `RISK_FLOOR`, `HELD_UNCONFIDENT_UNKNOWN_MID`, `TIER_FLOOR`, `NO_SIGNAL`). Prompt-free, recorded by the shared runner for every harness, counted in the export. No probability is logged; older events parse unchanged. See `docs/stats-export.md`.
- `downshift stats --by-harness`: Markdown review table of spawns, tier, effort, held-by-guardrail and linked tokens per harness (prompt-free).
- Prompt corpus harness (`internal/core/prompt_corpus_test.go`) reading hand-labelled files from `internal/core/testdata/corpus/`.

- Test that `Route` is a fixed point on its own output (hook coexistence).
- `docs/evidence/vendor-hook-docs-2026-10-11.md`: vendor hook docs compared with the adapters.
- Classifier signals for mechanical edits and commands (EN + PT-BR), refined with a development/control split; see docs/contrib/prompt-corpus.md.
- `adapt.CountPredictor` (shrinkage adapted from BerriAI/litellm, MIT; see NOTICE) and evaluator policy `adapt_counts_loo`. Shadow-only.
- `telemetry.RespawnAfterFailure` and a "Respawned after a failure" line in `stats` (hint only, prompt-free, harness-agnostic). Anchoring on the newest event is inspired by the LiteLLM stall detector (MIT); the code is independent.
- First held-out outcome tasks and a smoke run recorded (see gap-tasks GAP-SUITE).
- Evidence note: Workflow-created agents bypass PreToolUse (GAP-WORKFLOW-AGENTS).
- Test that an empty hook stdin fails open in the shared runner.

### Changed
- **Architecture:** session and quota planning is one shared step (`Decision.PlanSession`); harness-name branches in `quota` and native sessions are capability fields; one harness registry in `cmd/downshift`. No behaviour change for existing harnesses.
- **CLI:** `downshift honor|usage <harness>` no longer appear in `help` (still work; documented in `docs/contrib/agent-reference.md`).
- **Catalog:** Claude Haiku 5.5 (`claude-haiku-5-5`, $0.10/$0.50 per 1M tokens for
  prompts up to 100k; $0.50/$2.50 above) is the Claude Code small tier (KiroCrew keeps `claude-haiku-4-5`: the Kiro account list offers no 5.x model).
  Older `claude-haiku-4-5` ids in `session-models.json` still match through the
  `claude-haiku` family (not an alias: KiroCrew owns that canonical id). The small-vs-frontier price gap is now ~97% (was ~75%) at list price for prompts up to 100k tokens; above that Haiku 5.5 costs $0.50/$2.50 and the gap is ~87%.

### Added
- **Project tier floor:** optional `.downshift/policy.json` raises the minimum tier per project, class or risk category. Raise-only and fail-open; applied once in the core, so every adapter inherits it. `stats` and the export count decisions it raised (`policy_floor`). See `docs/config.md`.
- **How a child ended:** `child_ended` events record a closed status (`completed`, `launched`, `interrupted`, `error`, `stopped`, `unknown`) for Claude Code Agent/Task children, shown in `stats` and the export (`child_status`). The error text is never stored; the event never counts as a decision. Needs `claude-code-post-tool-use` registered under `PostToolUseFailure`; field names are from the vendor docs, not yet captured from a real session.
- **Cost per session and subagent** in `stats` and the export (`cost_sessions`, `cost_subagents`, `max_session_cost_usd`, `max_subagent_cost_usd`); report only, no budget enforcement.
- **`baseline_source`** on cost events (`requested` today).
- **`doctor`** reports model-override variables that a harness declares (`HarnessCapabilities.ModelOverrideEnv`), without claiming the override is verified.
- **`eval-adapt`** adds leave-one-task-out, cost per completed task, `--price-*`, `--ref-in/--ref-out` and `--fail-penalty` (evaluation only; routing unchanged).
- **Skill** `skills/downshift-audit`: read-only audit of hook wiring, stats and fixed context size.
- **Docs:** layering rule (`docs/contrib/layering.md`), agent reference, Portuguese telemetry chapter updated.
- **Session discovery:** `downshift models discover` asks each harness which models the
  account can use (Codex `models_cache.json`, `kiro-cli chat --list-models`, `grok models`,
  the Anthropic Models API with a key) and writes a ranked cache that hooks read after the
  hook payload and `session-models.json`. Newest model per family, ranked by catalog tier
  and price, never by name. Hooks stay offline. See `docs/session-discovery.md`.
- **CLI:** `downshift verification-report [--events=<file>]` prints, per tier, how many
  usage events were verified (`passed`/`failed`/`none`), how many predate the field,
  and the priced cost. Read-only. Coverage counts a missing field as unrecorded, never none.
- **Telemetry:** `verification` (`passed`/`failed`/`none`) on Claude Code `usage`
  events. `SubagentStop` reads the subagent transcript for a recognised test or
  lint command and its `is_error` result; the command and output are never stored.
  It is a local signal, not a quality guarantee: a subagent that runs no check
  reads as `none`.

### Changed
- **Docs:** Slim [README.md](README.md) (hero + quickstart + index). Full install,
  per-harness hooks, dsmon, plan compatibility, and troubleshooting moved to
  [docs/install.md](docs/install.md). [AGENTS.md](AGENTS.md) and [llms.txt](llms.txt)
  point agents at INSTALL. Added [examples/run-all.sh](examples/run-all.sh) adapter smoke.

## [0.1.0-beta.7] — 2026-10-06

### Changed
- **Benchmarks & training ops:** Public repo no longer ships benchmark JSON,
  outcome suite, or `tools/minilm` / `tools/baseline`. Curated metrics:
  [benchmark/REPORT.md](benchmark/REPORT.md). Maintainer eval and regression gates
  run in private **downshift-labs** ([#57](https://github.com/tiagovilasboas/downshift/pull/57)).
- **README / docs:** Messaging aligned with labs split (REPORT, not in-repo outcome
  CI); [BETA-EXIT](docs/beta-exit.md) P4.5 note; `Makefile report` no longer
  overwrites README ([#58](https://github.com/tiagovilasboas/downshift/pull/58) hygiene).
- **Docs:** Maintainer shadow/engineering/v2 long form stubbed in OSS; full copies
  in downshift-labs. [classifier-shadow.md](docs/classifier-shadow.md) shortened.
- **CI:** Public `ci` / `ci-full` run `go test` and adapter smoke only (sample
  fixture). Removed obsolete outcome-scope scripts.

### Added
- **Docs:** [content-classification.md](docs/content-classification.md) (A/B/C/D
  inventory, code vs docs policy for competitive content).
- **Docs:** [internal/doc-audit.md](docs/internal/doc-audit.md) (use vs inside vs
  MOAT flags for README, docs, benchmarks, examples).
- **Docs:** [benchmark/EVAL-PRIVATE.md](benchmark/EVAL-PRIVATE.md) policy for
  public vs maintainer eval.
- **Docs:** [docs/brand/social-preview.md](docs/brand/social-preview.md) (W1-5 upload steps).

## [0.1.0-beta.6] — 2026-10-06

### Changed
- **Docs:** Maintainer backlog/checklist stubs; competitive sensitivity matrix in
  [publishing-boundaries.md](docs/publishing-boundaries.md); public
  [engineering-loop.md](docs/engineering-loop.md) trimmed (detail in
  `docs/internal/engineering-loop-maintainer.md`).
- **Docs:** [publishing-boundaries.md](docs/publishing-boundaries.md) (public vs
  `docs/internal/` vs private). Launch waves and test coverage moved under
  `docs/internal/` with stubs at old paths.
- **Docs (wave 2):** Public [capability-router-v2.md](docs/capability-router-v2.md) condensed;
  full design moved to [docs/internal/capability-router-v2-full.md](docs/internal/capability-router-v2-full.md).
  Contributor classifier guide: [docs/contrib/classifier.md](docs/contrib/classifier.md).
  ROADMAP and CONTRIBUTING aligned with Downshift branding.
  `examples/` hook fixtures; GitHub issue template for beta stats exports (P2.3).
- **Rename:** GitHub repository and Go module are `tiagovilasboas/downshift`
  (`go install github.com/tiagovilasboas/downshift/cmd/downshift@latest`). State
  directory: `~/.downshift` with legacy fallback `~/.harness-downshift`; `downshift doctor`.
  See [docs/config.md](docs/config.md).
- **Docs:** README repositioned as **Downshift** (deterministic model router hero),
  plus [docs/when-to-use.md](docs/when-to-use.md) (vs LiteLLM, OpenRouter, direct SDK).
  `llms.txt` and `docs/pt/README.md` aligned.
- **Licence:** project relicensed from BUSL 1.1 to **Apache License 2.0**
  (2026-10-06). Archived BUSL text: `LICENSE-BSL-1.1-ARCHIVE.md`. Contributor
  terms: contributions are under Apache 2.0 (see CONTRIBUTING.md).

### Fixed
- Claude Code: a spawn that requested a model explicitly was never rewritten. The Task schema only accepts the native names (`haiku`/`sonnet`/`opus`), the hook did not recognise them as catalog models and held the spawn as foreign. `native_name` now resolves like an alias, so explicit requests are routed like the full id.
- Claude Code: the hook wrote the full catalog id (`claude-haiku-4-5`) into `updatedInput.model`, but the Task/Agent schema accepts only family names, so the harness rejected the rewrite and blocked the spawn. The adapter now writes the catalog's `native_name` and allows the spawn unchanged when an entry has none.

### Added
- `downshift claude-code-subagent-stop` (SubagentStop hook): prices a finished Claude Code subagent from its own transcript (tokens, API-reported model, real cost) and links it to the routing decision through a hashed agent id. Fail-open, idempotent.
- Claude Code PostToolUse records an `outcome: "resolved"` observation from `tool_response.resolvedModel` (the model the harness chose for the child), linked to the decision and compared through the catalog; `stats` counts a match as honored. The launch payload of an async subagent carries no token usage, so the `usage` path (P3.4) stays idle until a harness reports tokens.
- Catalog field `native_name` (family-level, inherited across versions and by `models pull` overrides) and `core.Model.Native`.
- `RewritePlan.WriteName` and `HarnessCapabilities.StrictModelName`: every adapter writes the plan's name (native name, else id) and a strict harness never writes a target without one. Claude Code is the only strict harness today.
- Opt-in classifier shadow: `DOWNSHIFT_SHADOW_WEIGHTS` records a candidate beside the production recommendation, and `downshift shadow-report` compares those observations with explicit reviewed labels. Guide: `docs/classifier-shadow.md`.

### Changed
- `downshift feedback <id> success` no longer writes an implicit minimum-tier label. Supervised training still requires `--required-tier`.

## [0.1.0-beta.5] — 2026-10-03

### Added
- Telemetry field `rewrite_honored: bool` to distinguish emission from honor.
- Smoke test for adapter golden payloads (E2E verification that hook fires).
- Evidence protocol for manual testing (Codex rewrite honored on real session).
- `downshift stats --days=N --export` for multi-user aggregation without prompts.
- Billing comparison template (`docs/billing-comparison-template.md`).
- Harness matrix (`docs/harness-matrix.md`) tracking adapter status per harness.

### Changed
- Improved guardrail R1 to preserve explicit-only models when already assigned.
- README reorganized for clarity on harness-level limitations vs router readiness.
- Classifier now includes MiniLM semantic scoring (enabled by default).

### Fixed
- Route caching respects per-session model allowlists.
- PostToolUse hook integration for usage event linkage (Claude Code path complete).

## [0.1.0-beta.4] — 2026-10-03

### Added
- `downshift benchmark --report` and `--gate` for offline evaluation.
- CapabilityRouter v2 pipeline: 13-signal extractor, deterministic safety floor, risk-weighted softmax.
- Graphify library for knowledge graph integration (command-fetcher mode).
- `DOWNSHIFT_MINILM=0` to opt out of semantic scoring.

### Changed
- Expanded curated benchmark dataset from 30 to 200 tasks.
- Holdout split (300 tasks) for regression gating, not generalization proof.
- Catalog v2 with version-agnostic family matching.

### Fixed
- OpenRouter model ID normalization (strips `provider/` prefix).

## [0.1.0-beta.3] — 2026-10-03

### Added
- Support for Antigravity (flash_lite, flash, pro).
- Deterministic classifier covering 40+ documented prompts.
- Event schema with cost normalization (CostUSD field).
- Multi-process JSONL lock and rotation policy.

### Changed
- Adapters now return `Task` struct with explicit fields for model/effort rewrite.
- Catalog entries support `routing:"explicit_only"` to prevent auto-downshift.

## [0.1.0-beta.2] — 2026-09-30

### Added
- Initial Claude Code, Cursor, and Codex adapters.
- Embedding in binary via `go:embed` for zero external model list dependencies.
- `downshift try` for local testing.
- `downshift models list|check|pull` for catalog inspection and override.

### Changed
- Classifier signals extracted and tunable in `internal/core/signals.go`.
- Hook output format: correlation ID + decision summary + stderr.

## [0.1.0-beta.1] — 2026-09-21

### Added
- Initial release: harness-downshift router core.
- PreToolUse hook integration for Claude Code.
- Deterministic complexity classifier.
- Tier assignment and model selection.
- Install script and basic CLI.

---

## Development

### Versioning

- **Beta** (v0.1.0-betaX): Router built, adapters tested, harnesses evolving. API may change.
- **Release Candidate** (v0.1.0-rcX): Beta exit criteria met, production-ready.
- **Stable** (v1.0.0+): API stable, harnesses broadly supported, real multi-user evidence.

### Before v1.0.0

Beta exit requires:
1. **Real-spawn verification** (P1): Hook rewrite honored on Claude Code (paid) and Cursor (Pro/Ultra).
2. **Multi-user evidence** (P2): ≥2 external contributors or non-maintainer sessions.
3. **Billing proof** (P3): ≥50 usage events with real provider costs.
4. **Benchmark generalization** (P4): New test set, blind, no tuning leakage.
5. **Harness coverage** (P5): Support matrix up-to-date with no stale TBDs.

See [docs/beta-exit.md](docs/beta-exit.md) for detailed tracking.

---

**License:** Apache License 2.0 (from 2026-10-06). Prior: BUSL 1.1 — see LICENSE-BSL-1.1-ARCHIVE.md
