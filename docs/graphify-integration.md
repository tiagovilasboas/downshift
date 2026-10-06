# Graphify integration: library, not hook escalation

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

`internal/graphify` is a library. `core.Route` does not escalate via the graph.
The hook path stays legacy regex scoring plus a monotonic semantic boost.
Escalation happens only when a caller injects a `GraphFetcher` and applies
`Hint` itself. The adapters do not do that.

A nil fetcher never escalated. `TestHint_NilFetcher_OfflineMode` in
`internal/graphify/graphify_test.go` passes a file path and a symbol with a
nil fetcher and expects `ShouldEscalate == false`. A nil fetcher is replaced
by `noopFetcher`, whose `FetchNode` returns `Found: false`, so edge and
community rules do not fire. A leftover nil call inside `core.Route`, while
it remains, still cannot change the tier.

The library can talk to the [Graphify](https://graphify.dev) knowledge graph
through Kiro Crew MCP tools (`query_graph`, `get_node`, `god_nodes`) only
after a caller supplies a fetcher.

## What the library does

`core.Route` scores task prompts with text signals. It does not know whether
a mentioned file is a God Node with 186 edges or a tiny utility with 3, and
it does not ask the graph.

If a caller injects a `GraphFetcher`, `Hint` can return `ShouldEscalate` when
the prompt mentions a file path or a PascalCase symbol and the fetcher finds
a node over the edge threshold or in a high-risk community. Applying that
hint is the caller's decision. `core.Route` does not apply it.

```
caller prompt → extractCandidates() → injected GraphFetcher.FetchNode()
                                              │
                                              ↓
                         edges ≥ 20  OR  high-risk community
                                              │
                                              ↓
                         EscalationHint.ShouldEscalate
                         (core.Route does not read this)
```

## Escalation criteria (defaults)

| Criterion | Value | Source |
|---|---|---|
| Edge threshold | ≥ 20 edges | `god_nodes` data (Controller=186, CreateSaleService=33) |
| High-risk communities | "Client & Subscription State", "User & Consent", "Repository Interfaces", "ERP Controllers" | `query_graph` observation 2026-09-30 |

## Zero-latency by design

These traces apply only when a caller injected a `GraphFetcher`. `core.Route` does not run them. A nil fetcher extracts candidates and does not escalate.

**No graph call is made when the prompt contains no file path or symbol.**
A rename or git commit prompt never touches the network.

```
"rename userId"          → 0 candidates → no fetch → no escalation (fast)
"fix CreateSaleService"  → 1 symbol     → FetchNode → community match → ShouldEscalate
"refactor src/app/..."   → 1 file path  → FetchNode → 33 edges → ShouldEscalate
```

## Integration points

### 1. Caller-owned hint (not `core.Route`)

`graphify.Hint` escalates only when the caller passes a `GraphFetcher`.
Do not add this call to `core.Route`.

```go
import "github.com/tiagovilasboas/downshift/internal/graphify"

hint := graphify.Hint(prompt, graphify.DefaultCriteria(), myFetcher)
if hint.ShouldEscalate {
    // Caller-owned. core.Route does not apply this hint and does not
    // escalate via the graph.
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

### 3. Not the hook

The production hook does not call `graphify.Hint` with a real fetcher.
`downshift graphify-check` is not implemented. Do not treat that gap as a
work order to escalate inside `core.Route`.

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
- Graph escalation inside `core.Route` (intentionally absent: a nil fetcher never escalated)
