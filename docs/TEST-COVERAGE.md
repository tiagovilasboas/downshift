<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: BUSL-1.1 -->

# Test Coverage Report

Generated: 2026-10-06  
Total packages: 34  
Run with: `go test ./... -cover`

## Perfect Coverage (100%)

| Package | Lines | Notes |
|---------|-------|-------|
| `internal/hookutil` | 31 | Utilities for JSON field extraction |
| `internal/routecache` | ~50 | Route decision caching |
| `internal/routingv2/safety` | ~100 | Safety guards for escalation |
| `internal/routingv2/policy` | ~100 | Tier & rewrite policies |

## High Coverage (90-99%)

| Package | Coverage | Lines | Notes |
|---------|----------|-------|-------|
| `internal/routingv2/domain` | 96.3% | ~200 | Decision & route types |
| `internal/routingv2/router` | 95.5% | ~150 | Main routing pipeline |
| `internal/nbtier` | 95.9% | ~100 | Naive Bayes tier classifier |
| `internal/hookctx` | 96.8% | ~150 | Hook execution context |
| `internal/orchestration` | 94.1% | ~100 | Agent orchestration |
| `internal/routingv2/extractor` | 91.9% | ~150 | Signal extraction |
| `internal/routingv2/matcher` | 91.4% | ~150 | Pattern matching for signals |
| `internal/benchmark` | 89.4% | ~400 | Benchmark runner & gates |
| `internal/catalog` | 89.5% | ~400 | Model catalog |
| `internal/models` | 89.6% | ~200 | Model list/check/pull CLI |
| `internal/adapters/antigravity` | 90.0% | ~150 | Antigravity hook adapter |
| `internal/routingv2/training` | 90.7% | ~300 | Offline training pipeline |

## Good Coverage (80-89%)

| Package | Coverage | Notes |
|---------|----------|-------|
| `internal/telemetry` | 87.6% | Event recording & cost tracking |
| `internal/decisionintelligence` | 87.0% | Decision analysis |
| `internal/core` | 86.0% | Routing core logic |
| `internal/lifecycleobserver` | 86.2% | Codex lifecycle POC |
| `internal/outcome` | 85.2% | Outcome evaluation |
| `internal/graphify` | 81.2% | Knowledge graph integration |
| `internal/adapters/codex` | 82.4% | Codex spawn adapter |
| `cmd/dsmon-hook` | 83.0% | Monitor hook binary |

## Moderate Coverage (70-79%)

| Package | Coverage | Notes |
|---------|----------|-------|
| `internal/adapters/cursor` | 72.5% | Cursor subagent adapter |
| `internal/adapters/kirocrew` | 78.9% | KiroCrew agent adapter |
| `internal/semantic` | 67.7% | MiniLM embedding & augmentation |
| `internal/routingv2/classifier` | 69.8% | Classifier implementation |

## Needs Attention (50-69%)

| Package | Coverage | Notes |
|---------|----------|-------|
| `cmd/downshift` | 53.7% | CLI main; many subcommands |
| `internal/server` | 57.8% | HTTP dashboard |
| `internal/adapters/claudecode` | 39.1% | Claude Code hook (main adapter) |

## UI / Generated (0-10%)

| Package | Coverage | Notes |
|---------|----------|-------|
| `cmd/dsmon` | 8.0% | Terminal widget (UI) |
| `cmd/dsmon-server` | 0.0% | Server wrapper (setup only) |

---

## Summary

- **Core routing** (routingv2, core, catalog): **91% average** ✅
- **Adapters**: **72% average** (claudecode 39% is the outlier; others 78-90%)
- **CLI** (cmd/downshift): **54%** (many subcommands need more tests)
- **Infrastructure** (telemetry, server, hookutil): **82% average**

## Improvements Made in This Session

| Package | Before | After | Δ |
|---------|--------|-------|---|
| `internal/adapters/claudecode` | 32% | 39% | +7% |
| `internal/server` | 47% | 58% | +11% |
| `internal/hookutil` | 50% | 100% | +50% |

## Recommended Next Steps

1. **claudecode adapter** (39%): Add tests for `HoldForeign`, `PreserveExplicit` paths
2. **cmd/downshift** (54%): Test `runStats`, `runFeedback`, `runTrain` subcommands
3. **server** (58%): Test rare error paths (file read failures, malformed JSON)
4. **semantic** (68%): Test `loadStore()`, `storeFor()` on missing files
5. **main.go refactor**: Split into `hook.go`, `commands.go` to improve maintainability

---

**Note:** Coverage alone does not guarantee correctness. High-coverage packages have been reviewed for edge-case handling; low-coverage areas need more rigorous testing before relying on them in production.
