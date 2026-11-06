Superseded in part by O8 (2026-10-28): see docs/internal/plans/mvp-plan.md

> **Note (2026-11-01).** The naive-Bayes opinions were removed: the upshift opinion on 2026-10-31 and the TRIVIAL downshift opinion in #708. `internal/classify` has no naive-Bayes code today (checked at `bf56237b`), so the "Naive Bayes" parts below describe an earlier design, not the current classifier. The live reference is [`docs/design/architecture.md`](../design/architecture.md).

# System Design

Downshift is a token-saving layer for AI coding agents. It sits between the
agent harness and the underlying models, optimizing both output (compaction)
and model selection (routing).

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Agent Harness                           │
│              (Claude Code, Codex, Cursor, KiroCrew)         │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      Downshift                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐   │
│  │  Compactor   │  │   Router     │  │   Telemetry      │   │
│  │  (output)    │  │  (models)    │  │   (local)        │   │
│  └──────────────┘  └──────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                    Model Providers                          │
│              (Anthropic, OpenAI, etc.)                      │
└─────────────────────────────────────────────────────────────┘
```

## Core Modules

### Compactor (`internal/compressor/`)

Transforms verbose command output into compact summaries:

- Pattern-based compression for 70+ command families
- Exit code preservation
- Failure detail retention
- Learning system for new patterns

### Router (`internal/core/`, `internal/classify/`)

Routes subagent tasks to right-sized models:

- Naive Bayes classifier for complexity
- Risk floor for critical domains
- Adaptive memory from outcomes

### Telemetry (`internal/telemetry/`)

Local-only observability:

- JSONL event storage
- No network calls
- Privacy-preserving (no prompts logged)
- Aggregation and reporting

## Design Principles

1. **Zero LLM overhead** — All decisions are deterministic
2. **Fail-open** — Errors leave commands/spawns unchanged
3. **Local-first** — No data leaves the machine
4. **Conservative routing** — Doubt never saves money

## See Also

- [Compressor Pipeline](compressor.md)
- [Routing Engine](routing.md)
