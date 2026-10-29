# ADR: Isolated Accounting Layer

**Date:** 2026-10-21
**Status:** Proposed
**Note (2026-10-29):** the savvy-progress mod this page names was removed, and the dollar account is out (owner decision O1).
**Author:** Tiago Vilas Boas

## Context

The Downshift codebase currently has accounting/metrics logic scattered across multiple packages:

| Package | Files | Responsibility |
|---------|-------|----------------|
| `internal/telemetry` | 88 files | Model routing decisions, token savings, sessions |
| `internal/execwrap` | 20 files | Compactor metrics (bytes saved, commands wrapped) |
| `internal/benchmark` | 9 files | Classifier accuracy, outcome eval |
| `cmd/downshift` | 56+ files | CLI commands that aggregate from both sources |

**Consumers of accounting data:**
1. `downshift stats` / `downshift report` — CLI reports
2. `downshift session --panel` — JSON consumed by savvy-progress mod
3. savvy-progress mod — Progress bar and agents panel in Claude Code

**Problem:** The mod (`mods/savvy-progress/hooks/register.tsx`) calls `downshift session --panel` and expects a specific JSON schema. Any change to telemetry or execwrap can break the mod. There's no single source of truth for "what Downshift saved".

## Decision

Create a new `internal/accounting` package as the single source of truth for all savings metrics.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        CONSUMERS                                │
├─────────────────────────────────────────────────────────────────┤
│  savvy-progress mod    │   CLI reports     │   External tools   │
│  (progress bar/agents) │   (stats/report)  │   (JSON export)    │
└────────────┬───────────┴────────┬──────────┴────────┬───────────┘
             │                    │                   │
             ▼                    ▼                   ▼
┌─────────────────────────────────────────────────────────────────┐
│                   internal/accounting                           │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  SessionSummary                                           │  │
│  │  ├── Routing: switched, held, tokens saved                │  │
│  │  ├── Compactor: commands, bytes saved, tokens estimated   │  │
│  │  ├── ByAgentType: per-type breakdown                      │  │
│  │  └── Spawns: per-spawn details with tool_use key          │  │
│  └───────────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  GlobalSummary                                            │  │
│  │  ├── All-time totals                                      │  │
│  │  └── Windowed (7d, 30d) breakdowns                        │  │
│  └───────────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  ComparisonReport                                         │  │
│  │  ├── Routed vs Control groups                             │  │
│  │  └── Savings with confidence intervals                    │  │
│  └───────────────────────────────────────────────────────────┘  │
└────────────┬───────────────────┬────────────────────┬───────────┘
             │                   │                    │
             ▼                   ▼                    ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│ internal/telemetry│ │ internal/execwrap │ │ internal/benchmark│
│ (routing events)  │ │ (compactor events)│ │ (eval outcomes)   │
└──────────────────┘  └──────────────────┘  └──────────────────┘
```

### Key Types

```go
// internal/accounting/types.go

// SessionSummary is the complete picture of what Downshift did in one session.
// This is the ONLY struct the CLI and mods should consume.
type SessionSummary struct {
    SessionID string `json:"session"`
    
    // Routing metrics (from telemetry)
    Routing RoutingMetrics `json:"routing"`
    
    // Compactor metrics (from execwrap)
    Compactor CompactorMetrics `json:"compactor,omitempty"`
    
    // Per-spawn breakdown for the agents panel
    Spawns []SpawnSummary `json:"spawns"`
    
    // Per-agent-type breakdown
    ByAgentType []AgentTypeSummary `json:"by_agent_type,omitempty"`
}

type RoutingMetrics struct {
    Switched    int   `json:"switched"`
    Held        int   `json:"held"`
    Observed    int   `json:"observed"`
    
    // Tokens saved from the requested model
    WorkTokens  int64 `json:"work_tokens"`
    WorkSaved   int64 `json:"work_saved"`
    CacheTokens int64 `json:"cache_tokens"`
    CacheSaved  int64 `json:"cache_saved"`
    
    // Computed: "saved 62% of work · 58% of cache"
    Line string `json:"line"`
}

type CompactorMetrics struct {
    Commands             int64 `json:"commands"`
    Compacted            int64 `json:"compacted"`
    RawBytes             int64 `json:"raw_bytes"`
    SavedBytes           int64 `json:"saved_bytes"`
    EstimatedTokensSaved int64 `json:"estimated_tokens_saved"`
}

type SpawnSummary struct {
    ToolUseKey    string `json:"tool_use"`
    State         string `json:"state"`
    FromModel     string `json:"from_model"`
    ToModel       string `json:"to_model"`
    ObservedModel string `json:"observed_model,omitempty"`
    Label         string `json:"label"`
    
    // This spawn's token savings
    Tokens      RoutingMetrics `json:"tokens"`
    TokensLabel string         `json:"tokens_label"`
}
```

### API

```go
// internal/accounting/session.go

// BuildSessionSummary aggregates telemetry + compactor for one session.
func BuildSessionSummary(sessionHash string) (SessionSummary, error)

// BuildGlobalSummary aggregates across all sessions.
func BuildGlobalSummary(opts SummaryOptions) (GlobalSummary, error)

// BuildComparisonReport builds the routed-vs-control report.
func BuildComparisonReport(days int) (ComparisonReport, error)
```

### Migration Path

1. **Phase 1:** Create `internal/accounting` with types and builders that delegate to telemetry/execwrap
2. **Phase 2:** Update `cmd/downshift/session.go` to use `accounting.BuildSessionSummary()`
3. **Phase 3:** Update `cmd/downshift/report.go` to use `accounting.BuildComparisonReport()`
4. **Phase 4:** Update `cmd/downshift/cmd_stats.go` to use accounting
5. **Phase 5:** Document the JSON schema for mod authors

### Schema Stability

The `SessionSummary` struct becomes the **contract** with external consumers:
- Additions are allowed (new fields)
- Removals require deprecation notice
- Renames are breaking changes

The mod's `downshift session --panel` call gets this struct as JSON. No more reaching into telemetry internals.

## Consequences

**Positive:**
- Single source of truth for all savings metrics
- Clear contract for mod authors
- Easier to add new metrics (DCI, etc.) without breaking mods
- Better testability — mock one interface

**Negative:**
- One more package to maintain
- Migration effort to update CLI commands

**Neutral:**
- The mod continues to call `downshift session --panel`; the JSON schema is now documented

## Alternatives Considered

1. **Keep current structure, document JSON** — Rejected: coupling remains, any refactor breaks mods
2. **Move all metrics into telemetry** — Rejected: execwrap is about shell wrapping, not routing
3. **Create a metrics gRPC/HTTP service** — Overkill for CLI tool
