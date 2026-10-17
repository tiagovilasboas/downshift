# Graphify integration: library and optional Route path

Downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

`internal/graphify` is a library. **Default (no env var):** `graphify.FetcherFromEnv()`
is nil, replaced by `noopFetcher`, so `classifyTask` / `core.Route` never
escalate via the graph — regex scoring plus a monotonic semantic boost only
([beta-exit P4.3](../reference/beta-exit.md)). **With `DOWNSHIFT_GRAPHIFY_CMD` set:**
`classifyTask` calls `graphifyHint` on unconfident, non-COMPLEX prompts; a
positive hint raises complexity to COMPLEX inside `Route`. That counts as a
deterministic lift for the R7/R8/R9 guardrails. The command fetcher is opt-in,
fail-open and bounded (3 s timeout; hook budget). Adapters do not call
`graphify.Hint` themselves; the shared classifier does when the env var is set.
A native Graphify **MCP fetcher** for Kiro Crew remains optional and deferred
([P4.8](../reference/beta-exit.md)).

`TestHint_NilFetcher_OfflineMode` in `internal/graphify/graphify_test.go` passes
a file path and a symbol with a nil fetcher and expects `ShouldEscalate == false`.
Without `DOWNSHIFT_GRAPHIFY_CMD`, that offline behaviour is what the hook path uses.

The library can talk to the [Graphify](https://graphify.dev) knowledge graph
through Kiro Crew MCP tools (`query_graph`, `get_node`, `god_nodes`) only
after a caller supplies a fetcher.

## What the library does

`core.Route` scores task prompts with text signals. It does not know whether
a mentioned file is a God Node with 186 edges or a tiny utility with 3, and
it does not ask the graph.

When a `GraphFetcher` is available (env command or an injected fetcher in tests),
`Hint` can return `ShouldEscalate` if the prompt mentions a file path or a
PascalCase symbol and the fetcher finds a node over the edge threshold or in a
high-risk community. In production hooks, `classifyTask` applies that hint only
when `DOWNSHIFT_GRAPHIFY_CMD` is set; otherwise the fetcher stays nil and the
hint is never applied.

```
prompt → extractCandidates() → GraphFetcher.FetchNode()  (nil → noop, no I/O)
                                              │
                                              ↓
                         edges ≥ 20  OR  high-risk community
                                              │
                                              ↓
                         EscalationHint.ShouldEscalate
                         (classifyTask applies when env fetcher is configured)
```

## Escalation criteria (defaults)

| Criterion | Value | Source |
|---|---|---|
| Edge threshold | ≥ 20 edges | `god_nodes` data (Controller=186, CreateSaleService=33) |
| High-risk communities | "Client & Subscription State", "User & Consent", "Repository Interfaces", "ERP Controllers" | `query_graph` observation 2026-09-30 |

## Zero-latency by design

These traces apply only when a non-nil fetcher runs (typically
`DOWNSHIFT_GRAPHIFY_CMD`). A nil fetcher still extracts candidates locally but
does not escalate.

**No graph call is made when the prompt contains no file path or symbol.**
A rename or git commit prompt never touches the network.

```
"rename userId"          → 0 candidates → no fetch → no escalation (fast)
"fix CreateSaleService"  → 1 symbol     → FetchNode → community match → ShouldEscalate
"refactor src/app/..."   → 1 file path  → FetchNode → 33 edges → ShouldEscalate
```

## Integration points

### 1. Shared classifier (`classifyTask` / `core.Route`)

`classifyTask` already calls `graphifyHint` → `graphify.Hint(..., graphify.FetcherFromEnv())`.
Do not duplicate that call in adapters. Standalone use (tests, future tools):

```go
import "github.com/tiagovilasboas/downshift/internal/graphify"

hint := graphify.Hint(prompt, graphify.DefaultCriteria(), myFetcher)
if hint.ShouldEscalate {
    // Tests or tools; production hooks rely on FetcherFromEnv() inside classifyTask.
}
```

### 2. GraphFetcher implementations

The package is interface-driven. Tests use `stubFetcher`. `KiroCrewFetcher` is not in this repo and is not on the hook.

| Impl | Used in | How |
|---|---|---|
| `stubFetcher` | `graphify_test.go` | Returns deterministic `NodeInfo` from a map |
| `KiroCrewFetcher` | Not shipped | Would call graphify MCP `get_node` / `query_graph` |

The unshipped fetcher would call:

```json
{
  "tool": "graphify::get_node",
  "args": { "label": "CreateSubscriptionStructureService" }
}
```

A caller would map that result to `NodeInfo{Label, Community, Edges, Found}`.

### 3. Hook path

Adapters never call `graphify` directly; `runHookAdapter` → `Plan` → `Route` →
`classifyTask` does. Graph escalation on the hook is **off** unless
`DOWNSHIFT_GRAPHIFY_CMD` names a helper. `downshift graphify-check` is not
implemented; P4.8 MCP wiring is still deferred.

## Active graph data (2026-09-30)

From `graph_stats`:
- **4934 nodes**, **9028 edges**, **183 communities**
- 98% extracted confidence

Top God Nodes (highest blast radius):

| Symbol | Edges | Community |
|---|---|---|
| Controller | 186 | Community 69 |
| moment() | 61 | (none recorded) |
| CreateSubscriptionStructureService | 40 | Client & Subscription State |
| SalesPublishPostGraduateCosmosService | 37 | Client & Subscription State |
| CreateSaleService | 33 | Client & Subscription State |

## What's NOT implemented yet

These items are not a request to put graph escalation on the hook:

- `KiroCrewFetcher`: a production MCP bridge does not ship in this repo
- `downshift graphify-check` CLI subcommand
- Graph data caching (TTL-based, to avoid repeated MCP calls per session)
- PR impact integration (`get_pr_impact` in Guardian hook)
- Graph escalation without configuration (default nil fetcher never escalates; set `DOWNSHIFT_GRAPHIFY_CMD` to enable the live classifyTask path)
