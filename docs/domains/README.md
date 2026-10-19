# Domain Architecture Layers

Downshift is structured into **5 decoupled Domain Architecture Layers**, enforcing Clean Architecture, the Single Responsibility Principle (SRP), and the Dependency Inversion Principle. Core domain policies stay strictly decoupled from harness-specific adapters, autonomous subsystems (such as Code Intelligence) never import routing logic, and diagnostic sensors observe execution without transforming live outputs.

---

## Domain Architecture Layers Overview

| Domain Layer | Specification | Clean Architecture Role | Core Scope & Responsibilities | Core Go Packages |
|---|---|---|---|---|
| **Domain 1: Subagent Model Routing & Guardrails** | [subagent-routing.md](subagent-routing.md) | Core Domain / Business Policy | Deterministic subagent routing, signal classification, semantic boost, R1-R9 guardrails, project tier floor, session discovery, and quota gate | `internal/core`, `internal/classify`, `internal/fuse`, `internal/session`, `internal/quotamodel`, `internal/catalog`, `internal/discovery` |
| **Domain 2: Code Intelligence & Context Layer (DCI)** | [code-intelligence.md](code-intelligence.md) | Autonomous Subsystem | Zero-runtime AST symbol extraction (7+ languages), incremental inverted index, reverse call-graph blast radius, BM25 ranking (Production-First + Co-Reference boost), conversational abstention, and structural context packaging with prompt injection defense | `internal/codeintel` |
| **Domain 3: Output Compaction & Context Optimization** | [context-optimization.md](context-optimization.md) | Supporting Domain / Optimization | Native AST/JSON context compressor (`safe` mode CLI), observe-only tool output sensor (PostToolUse byte accounting), Headroom companion detection & telemetry, and session context policies (`opusplan`) | `internal/contextopt`, `internal/compressor`, `internal/sensor`, `providers/headroom` |
| **Domain 4: Observability, Evidence & Verification** | [observability-telemetry.md](observability-telemetry.md) | Telemetry, Auditing & Evaluation | Prompt-free event logging (`events.jsonl`), 10% unrouted control group A/B, catalog-priced cost & savings reports (`stats`, `report`; one definition in `telemetry.Summarize`), verification reports (`verification: passed|failed|none`), capability matrix, and evidence ladder | `internal/telemetry`, `internal/childend`, `internal/hookport`, `internal/benchmark`, `internal/outcome` |
| **Domain 5: Harness Adapters & Packaging** | [harness-adapters-packaging.md](harness-adapters-packaging.md) | Interface Adapters & Delivery | Harness-specific edge I/O (Claude Code, Cursor, Codex, OpenCode, KiroCrew, Antigravity), fail-open runner (`runHookAdapter`), 5 packaging layers (Memory, Skills, Hooks, Agents, Plugin), Claude Code plugin, and mods | `internal/adapters/*`, `cmd/downshift`, `hooks`, `mods` |

---

## Architectural Invariants & Clean Boundaries

1. **Dependency Inversion**: Outer layers (Adapters, CLI commands) depend inward on domain interfaces (`core`, `codeintel`, `telemetry`). Inner domain packages never depend on edge adapters or external harness details.
2. **Subsystem Isolation**: Code Intelligence (`internal/codeintel`) maintains complete domain isolation from routing: `internal/codeintel` never imports `internal/core` or `internal/classify`, and vice versa.
3. **Zero Policy in Adapters**: Harness adapters (`internal/adapters/*`) only parse incoming wire JSON payloads, declare capabilities, invoke core domain services, and format wire responses. Routing policy is never duplicated across adapters.
4. **Fail-Open Execution**: If any component encounters a syntax error, unhandled condition, or timeout (bounded: 1 MiB stdin, 2 s read, 1.5 s deadline; a panic while loading the catalog is recovered too), it recovers safely and emits a fail-open response, allowing the agent to proceed without disruption.
5. **Epistemic Honesty**: Downshift differentiates catalog list-price estimates from native provider invoices, and only asserts savings backed by empirical telemetry or controlled A/B verification.

---

## Further Reading

- [Overall Documentation Index](../README.md): Complete catalog of guides, reference specs, design notes, evidence audits, and maintainer tasks.
- [System Architecture & Execution Flow](../design/architecture.md): Deep-dive into BDI deliberation, hook lifecycle, and module boundaries.
- [Harness Compatibility Matrix](../reference/harness-matrix.md): Status, observed capabilities, and rewrite honor across supported harnesses.
