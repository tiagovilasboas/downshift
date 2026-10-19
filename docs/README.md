# Documentation index

Downshift is organized around **Domain Architecture Layers** following Clean Architecture, Single Responsibility Principle (SRP), and strict domain boundaries. Every domain encapsulates its own logic, telemetry, and contracts without leaky cross-imports or circular dependencies.

Docs are grouped into 5 decoupled domain layers, detailed in [domains/](domains/), plus cross-cutting start guides and governance:
- **[Domain 1: Subagent Model Routing & Guardrails](domains/subagent-routing.md)** (Core policy, classifier, catalog, quota gate, session discovery)
- **[Domain 2: Code Intelligence & Context Layer (DCI)](domains/code-intelligence.md)** (Multi-language parsing, reverse call-graph blast radius, BM25 ranking, prompt injection delimiters)
- **[Domain 3: Output Compaction & Context Optimization](domains/context-optimization.md)** (CLI compaction, sensor, headroom companion)
- **[Domain 4: Observability, Evidence & Verification](domains/observability-telemetry.md)** (Telemetry, control group A/B, verification report, capability matrix)
- **[Domain 5: Harness Adapters & Packaging](domains/harness-adapters-packaging.md)** (Claude Code, Cursor, Codex, OpenCode, plugins, mods)

Markdown file names under `docs/` are lowercase kebab-case; only names a tool requires (`README.md`, `SKILL.md`, `AGENTS.md`) keep their exact casing.

---

## Domain Architecture Layers at a glance

| Domain Layer | Clean Architecture / SRP Role | Key Responsibilities & Invariants | Core Packages | Status | Specifications & Primary Docs |
|---|---|---|---|---|---|
| **Domain 1: Subagent Model Routing & Guardrails** | Core Domain / Business Logic | Deterministic subagent routing, signal scoring, hash boost, R1-R9 guardrails, project tier floor, session discovery & quota gates. Bounded reads, fail open. | `internal/core`, `internal/classify`, `internal/fuse`, `internal/session`, `internal/quotamodel`, `internal/catalog` | Implemented; honored on Claude Code | [domains/subagent-routing.md](domains/subagent-routing.md), [design/architecture.md](design/architecture.md) |
| **Domain 2: Code Intelligence & Context Layer (DCI)** | Autonomous Subsystem | Zero-runtime AST symbol & call extraction across 7+ languages, incremental inverted index, reverse call-graph blast radius (`InvertedIndex.Callers`), BM25 ranking (Production-First + Co-Reference boost), conversational abstention, structural packaging with prompt injection delimiters. Zero imports with core/classify. | `internal/codeintel` | Implemented (v0.1 – v0.4) | [domains/code-intelligence.md](domains/code-intelligence.md), [design/code-intelligence.md](design/code-intelligence.md) |
| **Domain 3: Output Compaction & Context Optimization** | Supporting Domain / Optimization | Native AST/JSON context compressor (`safe` mode CLI), observe-only tool output sensor (PostToolUse byte accounting), Headroom companion detection & telemetry (`context enable headroom`), session context policy (`opusplan`, plan mode). | `internal/contextopt`, `internal/compressor`, `internal/sensor`, `providers/headroom` | Implemented, observe-only | [domains/context-optimization.md](domains/context-optimization.md), [design/context-optimization.md](design/context-optimization.md) |
| **Domain 4: Observability, Evidence & Verification** | Telemetry, Auditing & Evaluation | Prompt-free event logging (`events.jsonl`), 10% unrouted control group A/B, catalog-priced cost & savings reports (`stats`, `report`), verification reports (`verification: passed|failed|none`), capability matrix, evidence ladder (rungs A-E). | `internal/telemetry`, `internal/childend`, `internal/hookport`, `internal/benchmark`, `internal/outcome` | Implemented; estimates, not an invoice | [domains/observability-telemetry.md](domains/observability-telemetry.md), [reference/capability-matrix.md](reference/capability-matrix.md) |
| **Domain 5: Harness Adapters & Packaging** | Interface Adapters & Delivery | Harness-specific edge I/O (Claude Code, Cursor, Codex, OpenCode, KiroCrew, Antigravity), fail-open runner (`runHookAdapter`), 5 packaging layers (Memory, Skills, Hooks, Agents, Plugin), Claude Code plugin, mods (`cache-tax`, `downshift-router`). Zero policy in adapters. | `internal/adapters/*`, `cmd/downshift`, `plugins`, `mods` | Completed to Experimental per harness | [domains/harness-adapters-packaging.md](domains/harness-adapters-packaging.md), [reference/harness-matrix.md](reference/harness-matrix.md) |

---

## Start here

| Doc | For |
|-----|-----|
| **[guides/install.md](guides/install.md)** | Install binary, hooks per harness, troubleshooting (humans + agents) |
| **[guides/public-beta.md](guides/public-beta.md)** | Public beta: what is proven, blockers, first contribution, roadmap |
| [reference/capability-matrix.md](reference/capability-matrix.md) | Every capability with status (implemented, not validated everywhere, experimental, planned), evidence and limits |
| [design/architecture.md](design/architecture.md) | End-to-end execution flow, domain layer boundaries, what Downshift is and is not |
| [guides/claude-code.md](guides/claude-code.md) | Primary harness: opusplan, baseline inference, plan mode, emitted vs honored vs used |
| [guides/evidence.md](guides/evidence.md) | Evidence ladder: list price, telemetry estimate, controlled benchmark, real tasks, cost per completed task |
| [guides/benchmark-protocol.md](guides/benchmark-protocol.md) | Reproducible comparison: fixed Opus vs native opusplan vs opusplan + Downshift |
| [guides/benchmark-output-cutting.md](guides/benchmark-output-cutting.md) | Protocol: routing vs output cutting (Headroom, `downshift exec`) vs RTK, with a pre-registered cut criterion |
| [guides/when-to-use.md](guides/when-to-use.md) | Downshift vs gateways and direct SDK; Claude Code is the primary harness |
| [guides/config.md](guides/config.md) | State dir, env vars, project tier floor, migration |
| [reference/beta-exit.md](reference/beta-exit.md) | Beta graduation criteria |
| [reference/prices.md](reference/prices.md) | Where list prices come from, the weekly sync PR, staleness warning, user override |
| [../ROADMAP.md](../ROADMAP.md) | Public roadmap |
| [../LICENSE](../LICENSE) | Apache 2.0 |
| [../MONETIZATION.md](../MONETIZATION.md) | Optional hosted/support |
| [../GOVERNANCE.md](../GOVERNANCE.md) | Releases and decisions |
| [pt/README.md](pt/README.md) | Documentação em português |

---

## Domain 1: Subagent Model Routing & Guardrails

The subagent routing domain provides deterministic model selection for delegated tasks. When a coding agent delegates to a subagent (`Agent`, `Task`, `spawn_agent`, `invoke_subagent`), Downshift evaluates task signals, applies a monotonic semantic boost, enforces safety guardrails (R1–R9: holding unconfident downshifts, maintaining risk floors), validates session model allowlists and quota availability, and emits updated tool inputs (`model` and `reasoning_effort`). The hot path runs in-process with zero network, zero LLMs, and zero prompt storage, failing open on any timeout.

| Doc | Type | Topic |
|-----|------|-------|
| [domains/subagent-routing.md](domains/subagent-routing.md) | Domain Hub | Domain specification: deterministic routing, BDI deliberation, guardrails, catalog resolution |
| [design/architecture.md](design/architecture.md) | Design | End-to-end flow, BDI decision lens, guardrails, and boundary invariants |
| [guides/config.md](guides/config.md) | Guide | Configuration, state directory, env vars, project tier floor (`.downshift/policy.json`) |
| [guides/subagents.md](guides/subagents.md) | Guide | When a subagent is routable: omit `model`; example mechanical worker agent |
| [guides/quota.md](guides/quota.md) | Guide | Observed usage quota, native snapshot evaluation, holding exhausted tiers |
| [guides/daily-truth.md](guides/daily-truth.md) | Guide | Daily operational truth: routing a subagent delegation, not a session |
| [guides/session-view.md](guides/session-view.md) | Guide | `downshift session`: subagent tree, decision rationale, session pause |
| [reference/session-models.md](reference/session-models.md) | Reference | Session candidate allowlists, Claude usage hooks, fallback resolution |
| [reference/session-discovery.md](reference/session-discovery.md) | Reference | `models discover`: harness-native model discovery and ranking |
| [reference/complexity-weights.md](reference/complexity-weights.md) | Reference | Signal weights, tier thresholds, and per-harness model mapping |
| [reference/scores.md](reference/scores.md) | Reference | Public capability scores: sources, cache, refresh, shadow selection |
| [examples/session-models.example.json](examples/session-models.example.json) | Example | Sample session model configurations across harnesses |
| [../README.md#the-llms-it-knows](../README.md#the-llms-it-knows) | Reference | Embedded catalog, families per harness, `explicit_only` handling |
| [design/minilm-semantic.md](design/minilm-semantic.md) | Design | Semantic boost: in-process word-hash embedder (raise-only gate) |
| [design/classifier-shadow.md](design/classifier-shadow.md) | Design | Shadow classifier candidate evaluation without affecting live routing |
| [design/router-generalization.md](design/router-generalization.md) | Design | Eval methodology and holdout discipline |
| [design/capability-router-v2.md](design/capability-router-v2.md) | Design | Router v2 architecture (experimental) |
| [design/decision-intelligence.md](design/decision-intelligence.md) | Design | Advisory decision intelligence layer |
| [design/langgraph-orchestration.md](design/langgraph-orchestration.md) | Design | Optional orchestration planner |
| [img/downshift-bdi.svg](img/downshift-bdi.svg) | Diagram | Editable BDI decision lens map (beliefs, desires, intentions) |
| [img/downshift-execution.svg](img/downshift-execution.svg) | Diagram | Complete numbered execution flow with ownership boundaries |
| [contrib/classifier.md](contrib/classifier.md) | Contrib | Classifier tuning: signal weights, margin thresholds, shadow testing |
| [contrib/prompt-corpus.md](contrib/prompt-corpus.md) | Contrib | Prompt corpus and evaluation fixture splits |
| [evidence/native-quota-routing-2026-10-09.md](evidence/native-quota-routing-2026-10-09.md) | Evidence | Native quota mark routing verification |
| [evidence/hash-v2-audit-2026-10-17.md](evidence/hash-v2-audit-2026-10-17.md) | Evidence | In-process hash embedder inventory and raise-only gate audit |

---

## Domain 2: Code Intelligence & Context Layer (DCI)

Downshift Code Intelligence (DCI) is a zero-runtime, deterministic, privacy-first code intelligence subsystem integrated into Downshift. Strictly decoupled from model routing (`internal/codeintel` never imports `internal/core` or `internal/classify`), DCI extracts structural declarations, outgoing calls, and imports across 7+ languages (Go AST, TypeScript/JavaScript, Python, Rust, PHP, Java, C#), builds an incremental inverted index with a reverse call-graph (`InvertedIndex.Callers`), scores relevance using BM25 with Production-First weighting and call-graph co-reference boosting, filters conversational chatter via an abstention engine, and packages compact structural sketches with downstream blast radius annotations. Context is framed with `<downshift-code-context>` security delimiters to prevent prompt injection and auto-injected into agent harness prompts via multi-harness hooks.

| Doc | Type | Topic |
|-----|------|-------|
| [domains/code-intelligence.md](domains/code-intelligence.md) | Domain Hub | Domain specification: zero-runtime AST, reverse call-graph, BM25 ranking, prompt defense |
| [design/code-intelligence.md](design/code-intelligence.md) | Specification | Complete DCI v0.1–v0.4 architecture, AST parsing, reverse call-graph blast radius, BM25 ranking, prompt injection defense, and security gate |
| [evidence/dci-v04-blast-radius.md](evidence/dci-v04-blast-radius.md) | Evidence | DCI v0.4 reverse call-graph & multi-language calls runtime verification |
| [evidence/dci-mvp-v01.md](evidence/dci-mvp-v01.md) | Evidence | DCI MVP cold/warm benchmarks, latency budgets (cold <= 2.5s, warm p95 <= 300ms), and zero-runtime verification |

---

## Domain 3: Output Compaction & Context Optimization

The context optimization domain manages context window consumption without intercepting or altering model generation in flight. The native compressor provides AST, JSON, text, and test output compaction in `safe` mode as an explicit CLI step (`downshift context compress`). The PostToolUse hook sensor is strictly observe-only: it records tool output bytes and potential token reductions without rewriting tool outputs seen by the model. Companion tools like Headroom (a compression proxy) are detected on PATH and recorded via `downshift context enable headroom`, but never installed, launched, or wrapped. Context policy also configures initial session posture (`opusplan`, plan mode; You should know is opt-in).

| Doc | Type | Topic |
|-----|------|-------|
| [domains/context-optimization.md](domains/context-optimization.md) | Domain Hub | Domain specification: native compressor, observe-only sensor, companion integration |
| [design/context-optimization.md](design/context-optimization.md) | Design | Native context compressor, observe-only tool output sensor, companion contracts, and failure handling |
| [guides/companions.md](guides/companions.md) | Guide | Companions: Headroom proxy detection, recommended Claude Code mods (You should know, Token Weather, Replay Theater, reflect-mod) |
| [../skills/cost-reducer/SKILL.md](../skills/cost-reducer/SKILL.md) | Skill | Agent skill: cut session tokens in fixed order (routing, fixed context, tool output, policy, misroutes) |
| [evidence/report-e2e/status-compaction.json](evidence/report-e2e/status-compaction.json) | Evidence | Context compaction status telemetry record |
| [evidence/report-e2e/compaction-block.png](evidence/report-e2e/compaction-block.png) | Evidence | Visual evidence of compaction blocks |

---

## Domain 4: Observability, Evidence & Verification

The observability and verification domain delivers empirical proof of routing behavior, costs, and quality. Telemetry persists prompt-free routing events (`events.jsonl`) locally, with 10% of subagent spawns randomly routed to an unrouted control arm to measure real baselines. Cost estimation uses catalog list prices (`downshift stats`), while `downshift report` computes cost per completed task and bootstrap confidence intervals. The verification report tracks executed validation commands (`verification: passed|failed|none`) to evaluate child success without trusting self-reported outcomes. Capabilities are cataloged against the 5-rung evidence ladder (A to E).

| Doc | Type | Topic |
|-----|------|-------|
| [domains/observability-telemetry.md](domains/observability-telemetry.md) | Domain Hub | Domain specification: prompt-free telemetry, empirical control group, rework tracking |
| [reference/capability-matrix.md](reference/capability-matrix.md) | Reference | Capability matrix with status, evidence, limits (audit date 2026-10-17) |
| [reference/stats-export.md](reference/stats-export.md) | Reference | Sanitized telemetry export format for safe sharing |
| [reference/beta-exit.md](reference/beta-exit.md) | Reference | Beta graduation criteria and open blockers |
| [guides/report.md](guides/report.md) | Guide | `downshift report`: routed vs control group, savings with bootstrap CI, cost per task |
| [guides/simulation.md](guides/simulation.md) | Guide | Offline simulation gate: synthetic spawns through the real hook path, accounting invariants, ESTIMATE tables (no model call) |
| [guides/verification.md](guides/verification.md) | Guide | `verification-report`: observed child checks and estimated cost by tier, sensor limits |
| [guides/evidence.md](guides/evidence.md) | Guide | Evidence ladder: list price, telemetry estimate, controlled benchmark, real tasks, cost per task |
| [guides/benchmark-protocol.md](guides/benchmark-protocol.md) | Guide | Protocol for reproducible three-arm comparison (Opus vs opusplan vs opusplan+Downshift) |
| [guides/benchmark-output-cutting.md](guides/benchmark-output-cutting.md) | Guide | Protocol, not yet run: routing, output cutting and RTK arms, cut criterion |
| [guides/pilot.md](guides/pilot.md) | Guide | Two-week team pilot methodology: routed vs control |
| [guides/billing-comparison-template.md](guides/billing-comparison-template.md) | Guide | Template to compare provider billing with `downshift stats --export` |
| [design/engineering-loop.md](design/engineering-loop.md) | Design | Feedback loop: telemetry, outcome evals, reviewed feedback, and offline tuning |
| [evidence/subagent-routing-ab-2026-10-15.md](evidence/subagent-routing-ab-2026-10-15.md) | Evidence | Controlled A/B experiment on Claude Code: routed vs control arm |
| [evidence/baseline-freeze-2026-10-17.md](evidence/baseline-freeze-2026-10-17.md) | Evidence | Baseline freeze manifest: frozen eval sets, sha256, reference numbers |
| [evidence/e5-int8-evaluation-2026-10-17.md](evidence/e5-int8-evaluation-2026-10-17.md) | Evidence | Multilingual E5 candidate evaluation: baseline measured, fuse stays off |
| [evidence/final-gap-qa.md](evidence/final-gap-qa.md) | Evidence | Final gap analysis and QA closeout |
| [evidence/closeout-recheck-qa.md](evidence/closeout-recheck-qa.md) | Evidence | Closeout recheck and verification audit |
| [evidence/closeout-recheck-2026-10-09.md](evidence/closeout-recheck-2026-10-09.md) | Evidence | Prior closeout verification audit |
| [evidence/hardening-audit-2026-10-15.md](evidence/hardening-audit-2026-10-15.md) | Evidence | Hardening audit: defect triage, regression tests, benchmark impact |
| [evidence/billing-gap-audit.md](evidence/billing-gap-audit.md) | Evidence | Audit of linked Claude Code usage records to native tokens/models |
| [evidence/](evidence/) | Evidence | Dated runtime evidence directory (rewrite honored, audits) |
| [../benchmark/README.md](../benchmark/README.md) | Benchmark | Benchmark dataset splits and evaluation harness |

---

## Domain 5: Harness Adapters & Packaging

The harness adapters and packaging domain connects Downshift's core intelligence to diverse coding environments. Edge adapters parse harness-specific JSON hook payloads (Claude Code, Cursor, Codex, OpenCode, KiroCrew, Antigravity) into normalized structs, declare supported capabilities, invoke the core routing or DCI engine, and format wire responses. The runner enforces fail-open guarantees (1 MiB stdin cap, per-hook deadline, panic recovery). Packaging is organized into 5 layers (Memory, Skills, Hooks, Agents, Plugin), providing uniform capabilities across harnesses while leveraging platform-native features like Claude Code plugins and opt-in mods.

| Doc | Type | Topic |
|-----|------|-------|
| [domains/harness-adapters-packaging.md](domains/harness-adapters-packaging.md) | Domain Hub | Domain specification: multi-harness adapters, 5 packaging layers, plugin & mods |
| [guides/claude-code.md](guides/claude-code.md) | Guide | Primary harness guide: opusplan, baseline inference, plan mode, emitted vs honored vs used |
| [guides/install.md](guides/install.md) | Guide | Installation and harness hook configuration for all supported tools |
| [guides/when-to-use.md](guides/when-to-use.md) | Guide | When to use Downshift vs gateways and direct SDKs |
| [guides/kirocrew-routing-guide.md](guides/kirocrew-routing-guide.md) | Guide | KiroCrew subagent routing via native policy hook |
| [guides/probe.md](guides/probe.md) | Guide | `downshift probe`: inspecting real hook payloads and field names |
| [reference/harness-matrix.md](reference/harness-matrix.md) | Reference | Plan compatibility matrix (harness x plan x rewrite honored status) |
| [design/architecture.md#packaging-layers-what-an-agent-harness-loads](design/architecture.md#packaging-layers-what-an-agent-harness-loads) | Design | The 5 packaging layers (Memory, Skills, Hooks, Agents, Plugin) |
| [design/mod-router.md](design/mod-router.md) | Design | Experimental router mod: in-process routing from `agent.spawn` |
| [contrib/layering.md](contrib/layering.md) | Contrib | Layering rule: every feature serves every harness (adapters only parse and emit) |
| [contrib/agent-reference.md](contrib/agent-reference.md) | Contrib | On-demand architectural detail for agents |
| [../skills/README.md](../skills/README.md) | Skills | Agent skills overview: jobs, measured context cost, delegation overhead |
| [../skills/downshift-setup/SKILL.md](../skills/downshift-setup/SKILL.md) | Skill | Agent skill: install or repair hooks on any harness |
| [../skills/downshift-audit/SKILL.md](../skills/downshift-audit/SKILL.md) | Skill | Agent skill: audit hook wiring, stats, and fixed context size |
| [../.claude-plugin/plugin.json](../.claude-plugin/plugin.json) | Packaging | Claude Code plugin definition |
| [../mods/README.md](../mods/README.md) | Packaging | Claude Code mods: cache-tax, savvy-progress, filetree |
| [../mods/downshift-router/README.md](../mods/downshift-router/README.md) | Packaging | Experimental in-process routing mod |
| [../examples/README.md](../examples/README.md) | Examples | Sample hook JSON payloads across harnesses |
| [evidence/claude-code-rewrite-honored-2026-10-06.md](evidence/claude-code-rewrite-honored-2026-10-06.md) | Evidence | Observed rewrite honor on Claude Code |
| [evidence/cursor-rewrite-honored-2026-10-10.md](evidence/cursor-rewrite-honored-2026-10-10.md) | Evidence | Cursor rewrite evaluation (discarded on Free/legacy Pro) |
| [evidence/cursor-gap-diagnostic.md](evidence/cursor-gap-diagnostic.md) | Evidence | Cursor integration diagnostic and plan differences |
| [evidence/codex-rewrite-honored-2026-10-02.md](evidence/codex-rewrite-honored-2026-10-02.md) | Evidence | Inferred rewrite honor on Codex |
| [evidence/multilang-codex-2026-10-16.md](evidence/multilang-codex-2026-10-16.md) | Evidence | Multilingual checks through Codex adapter |
| [evidence/benchmark-opencode-2026-10-18.md](evidence/benchmark-opencode-2026-10-18.md) | Evidence | OpenCode integration benchmark |
| [evidence/kirocrew-native-hook-2026-10-10.md](evidence/kirocrew-native-hook-2026-10-10.md) | Evidence | KiroCrew native hook policy mode verification |
| [evidence/antigravity-executor-ack.md](evidence/antigravity-executor-ack.md) | Evidence | Antigravity runtime execution acknowledgment |
| [evidence/antigravity-runtime-source.md](evidence/antigravity-runtime-source.md) | Evidence | Antigravity runtime source inspection |
| [evidence/plugin-mods-smoke-2026-10-17.md](evidence/plugin-mods-smoke-2026-10-17.md) | Evidence | Plugin and mods smoke testing under headless Claude Code |

---

## Governance & Cross-Cutting

| Doc | Topic |
|-----|-------|
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | Contribution guidelines, misroutes, layout, and testing |
| [../ROADMAP.md](../ROADMAP.md) | Public roadmap |
| [../GOVERNANCE.md](../GOVERNANCE.md) | Governance, releases, and decision-making |
| [../LICENSE](../LICENSE) | Apache 2.0 license |
| [../MONETIZATION.md](../MONETIZATION.md) | Commercial terms and optional hosted/support |
| [reference/relicense.md](reference/relicense.md) | Relicense to Apache 2.0 record (2026-10-06) |
| [reference/publishing-boundaries.md](reference/publishing-boundaries.md) | Public vs internal vs private documentation boundaries |
| [reference/content-classification.md](reference/content-classification.md) | Content classification map (adoption -> secret) |
| [pt/README.md](pt/README.md) | Documentação em português |

---

## Brand

| Doc | Topic |
|-----|-------|
| [brand/rename.md](brand/rename.md) | Harness → Downshift rename history |
| [brand/readme-hero.md](brand/readme-hero.md) | README hero drafts and messaging |

---

## Internal (maintainers)

Not required for install. See [reference/publishing-boundaries.md](reference/publishing-boundaries.md).

| Doc | Topic |
|-----|-------|
| [internal/README.md](internal/README.md) | Maintainer documentation index |
| [internal/launch-waves.md](internal/launch-waves.md) | Launch execution checklist |
| [internal/backlog.md](internal/backlog.md) | Maintainer backlog (stub) |
| [internal/next-steps.md](internal/next-steps.md) | Maintainer checklist (stub) |
| [internal/gap-tasks.md](internal/gap-tasks.md) | Runtime and evidence task queue |
| [internal/tasks/](internal/tasks/) | Dated task waves |
| [internal/engineering-loop-maintainer.md](internal/engineering-loop-maintainer.md) | Promotion / dsmon appendix |
