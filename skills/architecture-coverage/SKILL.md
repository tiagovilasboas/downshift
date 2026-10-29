---
name: architecture-coverage
description: >
  Fill the architecture coverage inventory before proposing a roadmap phase
  that touches the Downshift binary. Triggers on: roadmap, architecture, BDI,
  accounting, compaction, "the whole thing", or any plan with multiple phases.
  Only applies when the workspace is the Downshift repository.
---

# Architecture coverage — procedure

**When to load:** a roadmap phase, an architecture question, a BDI question, a
compaction plan, an accounting plan, or any request that says "the whole thing".

**This skill does not ship in the plugin.** Operators who install Downshift do
not pay this context. It is a contributor-only procedure.

## Step 1 — read the three sources (do not use `docs/architecture/`)

1. [`docs/design/architecture.md`](../../docs/design/architecture.md) —
   current import graph, domain map, known gaps.
2. [`docs/domains/README.md`](../../docs/domains/README.md) — per-domain
   boundaries and invariants.
3. [`docs/reference/capability-matrix.md`](../../docs/reference/capability-matrix.md) —
   what each harness can and cannot do today.

`docs/architecture/` files are historical. Do not cite them as current.

## Step 2 — fill the inventory

Copy this table into the plan. Mark each row `in`, `out`, or `unknown`.
A blank cell or an `unknown` cell means the plan is not ready to implement.

| Surface | Packages | Effect | Account | This phase |
|---------|----------|--------|---------|-----------|
| Routing | `core`, `classify`, `fuse`, `session`, `catalog`, `quota` | writes child model | `telemetry.Summarize` | |
| Shell | `execwrap`, `shellwrap` | rewrites command / advises retry | bytes, outside `Saved` | |
| DCI | `codeintel` | opt-in context injection | latency + bytes | |
| Telemetry | `telemetry`, `childend`, `hookport` | measures | token, no dollar | |
| Accounting | `accounting` | CLI does not import; ADR Proposed | dollar, +15% bias | |
| Adapters | `internal/adapters/*` | translates the wire | honor + usage per harness | |
| Quality | `outcome`, `benchmark` | offline | acceptance suite | |

## Step 3 — check the two accounts

The token account (`telemetry.Summarize`) and the dollar account (`accounting`,
ADR Proposed) are **not the same number**. Never merge them in a claim or plan.

## Step 4 — confirm doubt stays allow

An unconfident routing decision (`!Decision.Confident`) allows the spawn unchanged
(rules R7, R8 in `core.Check`). No phase in this plan changes that.

## What CI enforces (`internal/docscheck`)

- Every `internal/<pkg>/` and `cmd/<pkg>/` directory on disk must appear in
  `docs/contrib/coverage.md` or `AGENTS.md`.
- Every package path in `AGENTS.md` must exist on disk.
- `AGENTS.md` must be ≤ 200 lines.

A phase that adds a package without updating the inventory will fail CI.
