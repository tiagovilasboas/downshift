<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **Docs:** README repositioned as **Downshift** (deterministic model router hero),
  plus [docs/WHEN-TO-USE.md](docs/WHEN-TO-USE.md) (vs LiteLLM, OpenRouter, direct SDK).
  `llms.txt` and `docs/pt/README.md` aligned. GitHub repo name remains
  `harness-downshift` until wave 4 rename.
- **Licence:** project relicensed from BUSL 1.1 to **Apache License 2.0**
  (2026-10-06). Archived BUSL text: `LICENSE-BSL-1.1-ARCHIVE.md`. Contributor
  terms: contributions are under Apache 2.0 (see CONTRIBUTING.md).

### Fixed
- Claude Code: the hook wrote the full catalog id (`claude-haiku-4-5`) into `updatedInput.model`, but the Task/Agent schema accepts only family names, so the harness rejected the rewrite and blocked the spawn. The adapter now writes the catalog's `native_name` and allows the spawn unchanged when an entry has none.

### Added
- `downshift claude-code-subagent-stop` (SubagentStop hook): prices a finished Claude Code subagent from its own transcript (tokens, API-reported model, real cost) and links it to the routing decision through a hashed agent id. Fail-open, idempotent.
- Claude Code PostToolUse records an `outcome: "resolved"` observation from `tool_response.resolvedModel` (the model the harness chose for the child), linked to the decision and compared through the catalog; `stats` counts a match as honored. The launch payload of an async subagent carries no token usage, so the `usage` path (P3.4) stays idle until a harness reports tokens.
- Catalog field `native_name` (family-level, inherited across versions and by `models pull` overrides) and `core.Model.Native`.
- `RewritePlan.WriteName` and `HarnessCapabilities.StrictModelName`: every adapter writes the plan's name (native name, else id) and a strict harness never writes a target without one. Claude Code is the only strict harness today.
- Opt-in classifier shadow: `DOWNSHIFT_SHADOW_WEIGHTS` records a candidate beside the production recommendation, and `downshift shadow-report` compares those observations with explicit reviewed labels. Guide: `docs/CLASSIFIER-SHADOW.md`.

### Changed
- `downshift feedback <id> success` no longer writes an implicit minimum-tier label. Supervised training still requires `--required-tier`.

## [0.1.0-beta.5] — 2026-10-03

### Added
- Telemetry field `rewrite_honored: bool` to distinguish emission from honor.
- Smoke test for adapter golden payloads (E2E verification that hook fires).
- Evidence protocol for manual testing (Codex rewrite honored on real session).
- `downshift stats --days=N --export` for multi-user aggregation without prompts.
- Billing comparison template (`docs/billing-comparison-template.md`).
- Harness matrix (`docs/HARNESS-MATRIX.md`) tracking adapter status per harness.

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

See [docs/BETA-EXIT.md](docs/BETA-EXIT.md) for detailed tracking.

---

**License:** Apache License 2.0 (from 2026-10-06). Prior: BUSL 1.1 — see LICENSE-BSL-1.1-ARCHIVE.md
