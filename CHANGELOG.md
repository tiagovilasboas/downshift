<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **Catalog:** Claude Haiku 5.5 (`claude-haiku-5-5`, $0.10/$0.50 per 1M tokens for
  prompts up to 100k; $0.50/$2.50 above) is the Claude Code small tier (KiroCrew keeps `claude-haiku-4-5`: the Kiro account list offers no 5.x model).
  Older `claude-haiku-4-5` ids in `session-models.json` still match through the
  `claude-haiku` family (not an alias: KiroCrew owns that canonical id). The small-vs-frontier price gap is now ~97% (was ~75%) at list price for prompts up to 100k tokens; above that Haiku 5.5 costs $0.50/$2.50 and the gap is ~87%.

### Added
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
