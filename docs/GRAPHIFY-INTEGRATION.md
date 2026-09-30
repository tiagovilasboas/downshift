# Graphify Integration — Codebase-Graph-Aware Escalation

The `internal/graphify` package connects the Downshift classifier to the
[Graphify](https://graphify.dev) knowledge graph via the Kiro Crew MCP tools
(`query_graph`, `get_node`, `god_nodes`).

## What it does

The standard Downshift classifier scores task prompts using **text signals**
(regex patterns). It is deterministic and fast but cannot know whether the
file mentioned in a prompt is a God Node with 186 edges or a tiny utility
with 3.

The graphify adapter adds a second layer: if the prompt mentions a **file path
or a PascalCase symbol**, it queries the graph and can **escalate to COMPLEX**
before the text-based tier is applied.

```
prompt → extractCandidates() → GraphFetcher.FetchNode() → EscalationHint
                                                              │
                    ┌─────────────────────────────────────────┘
                    ↓
         edges ≥ 20  OR  community in [payment, KYC, subscription]
                    │
                    ↓
          Route() overrides Complexity → COMPLEX
```

## Escalation criteria (defaults)

| Criterion | Value | Source |
|---|---|---|
| Edge threshold | ≥ 20 edges | `god_nodes` data (Controller=186, CreateSaleService=33) |
| High-risk communities | "Client & Subscription State", "User & Consent", "Repository Interfaces", "ERP Controllers" | `query_graph` observation 2026-09-30 |

## Zero-latency by design

**No graph call is made when the prompt contains no file path or symbol.**
A rename or git commit prompt never touches the network.

```
"rename userId"          → 0 candidates → no fetch → no escalation (fast)
"fix CreateSaleService"  → 1 symbol     → FetchNode → Community match → COMPLEX
"refactor src/app/..."   → 1 file path  → FetchNode → 33 edges → COMPLEX
```

## Integration points

### 1. Policy layer (Go — no MCP)

Use `graphify.Hint(prompt, criteria, fetcher)` **before** `core.Route()`:

```go
import "github.com/tiagovilasboas/harness-downshift/internal/graphify"

hint := graphify.Hint(prompt, graphify.DefaultCriteria(), myFetcher)
if hint.ShouldEscalate {
    // Force Complex tier regardless of text-only classification.
    decision := core.Route(prompt+" [graphify-escalated]", harness, currentModelID, resolver)
    // Or: use a wrapper that accepts a complexity override.
}
```

### 2. GraphFetcher implementations

The package is interface-driven. Two implementations are expected:

| Impl | Used in | How |
|---|---|---|
| `stubFetcher` | Tests | Returns deterministic `NodeInfo` from a map |
| `KiroCrewFetcher` | Kiro Crew hooks | Calls graphify MCP `get_node` / `query_graph` via the MCP bridge |

The `KiroCrewFetcher` (not included here — lives in the adapter layer) calls:

```json
{
  "tool": "graphify::get_node",
  "args": { "label": "CreateSubscriptionStructureService" }
}
```

And maps the result to `NodeInfo{Label, Community, Edges, Found}`.

### 3. Kiro hook (`preToolUse` or `promptSubmit`)

The hook can call `downshift graphify-check --prompt="..."` (new subcommand,
not yet implemented) which runs `Hint()` with the `KiroCrewFetcher` and exits
with code 2 if escalation is needed.

## Active graph data (2026-09-30)

From `graph_stats`:
- **4934 nodes**, **9028 edges**, **183 communities**
- 98% extracted confidence

Top God Nodes (highest blast radius):

| Symbol | Edges | Community |
|---|---|---|
| Controller | 186 | Community 69 |
| moment() | 61 | — |
| CreateSubscriptionStructureService | 40 | Client & Subscription State |
| SalesPublishPostGraduateCosmosService | 37 | Client & Subscription State |
| CreateSaleService | 33 | Client & Subscription State |

## What's NOT implemented yet

- `KiroCrewFetcher` — the production MCP bridge (adapter layer task)
- `downshift graphify-check` CLI subcommand
- Graph data caching (TTL-based, to avoid repeated MCP calls per session)
- PR impact integration (`get_pr_impact` in Guardian hook)
