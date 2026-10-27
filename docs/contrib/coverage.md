# Architecture coverage inventory

Fill this table before proposing any roadmap phase that touches the binary.
A blank cell invalidates the plan. **Do not use `docs/architecture/` as a source** —
those files are historical. Read [`docs/design/architecture.md`](../design/architecture.md),
[`docs/domains/README.md`](../domains/README.md), and
[`docs/reference/capability-matrix.md`](../reference/capability-matrix.md).

## The seven surfaces

| Surface | Packages | Effect | Account |
|---------|----------|--------|---------|
| Routing | `core`, `classify`, `fuse`, `session`, `catalog`, `quota` | writes the child's model | `telemetry.Summarize` |
| Shell | `execwrap`, `shellwrap` | rewrites the command, advises retry, or passes through | bytes, outside `Saved` |
| DCI | `codeintel` | opt-in, injects context | latency and bytes |
| Telemetry | `telemetry`, `childend`, `hookport` | measures | token, no dollar |
| Accounting | `accounting` | CLI does not import; ADR Proposed | dollar, measured +15% bias |
| Adapters | `internal/adapters/*` | translates the wire | honor and usage per harness |
| Quality | `outcome`, `benchmark` | offline | acceptance suite |

**Two separate savings accounts.** The token account (`telemetry.Summarize`) and the
dollar account (`accounting`, ADR Proposed, +15% bias) are not the same number.
Never merge them in a plan or a claim.

**Doubt stays allow.** An unconfident routing decision allows the spawn unchanged (R7, R8).
No phase changes this.

## How to fill it for a proposed phase

Copy the table above into the phase description. For each surface mark:

- `in` — the phase touches this surface
- `out` — the phase does not touch this surface
- `unknown` — the phase may touch this surface; investigate before shipping

A phase with any `unknown` cell is not ready to implement.

## Package inventory

Every `internal/` and `cmd/` directory is listed here. The CI test
`TestAllPackagesInventoried` fails when a directory exists but is absent from this
file or from `AGENTS.md`.

| Package | Surface | Notes |
|---------|---------|-------|
| `internal/accounting` | Accounting | Exists; ADR Proposed; `cmd/downshift` does not import it |
| `internal/adapters` | Adapters | Subdirs: `claudecode`, `cursor`, `codex`, `antigravity`, `kirocrew`, `opencode` |
| `internal/api` | — | Dashboard HTTP server (`downshift dashboard`) |
| `internal/archtest` | Quality | Import-graph enforcement tests |
| `internal/benchmark` | Quality | Offline classifier accuracy |
| `internal/catalog` | Routing | Embedded `catalog.json`, user override, Resolver |
| `internal/childend` | Telemetry | Child-end recorder |
| `internal/classify` | Routing | Regex signals, language guard, risk floor |
| `internal/codeintel` | DCI | Opt-in context injection |
| `internal/compressor` | Shell | Output compressor (families, corpus, mutation-closure tests) |
| `internal/contextopt` | Shell | Config for compaction and headroom |
| `internal/core` | Routing | `Route`, `Check` (R1-R9), `PlanSession`, `HarnessCapabilities` |
| `internal/cursorbridge` | Adapters | Cursor native session bridge |
| `internal/discovery` | Routing | `models discover/list/check/pull` |
| `internal/docscheck` | Quality | CI gates for markdown layout and package inventory |
| `internal/execwrap` | Shell | Allowlist policy, family classification, compactor runner |
| `internal/fuse` | Routing | Raise-only semantic boost |
| `internal/gateway` | — | OmniRoute and OpenRouter HTTP gateway |
| `internal/hookctx` | Adapters | Hook deadline context |
| `internal/hookport` | Telemetry | Honor and usage ports |
| `internal/hookutil` | Adapters | Field helpers for hook payloads |
| `internal/lifecycleobserver` | Telemetry | Process lifecycle events |
| `internal/limits` | Adapters | Centralized harness limitations registry |
| `internal/models` | Routing | `models` subcommand helpers |
| `internal/nbtier` | Routing | Naive-Bayes tier types |
| `internal/outcome` | Quality | Offline eval runner |
| `internal/paths` | — | Cross-platform path helpers |
| `internal/policyfile` | Routing | Project tier floor; registers via hook |
| `internal/quota` | Routing | File I/O for quota; registers hooks |
| `internal/quotamodel` | Routing | Pure quota types, no I/O |
| `internal/semantic` | Routing | Semantic boost (BM25 / embedding-free) |
| `internal/sensor` | Telemetry | Tool-output sensor (observe-only PostToolUse) |
| `internal/session` | Routing | `SessionList`, loaders |
| `internal/shellwrap` | Shell | Shell wrap domain: `RewriteAdapter`, `AdviseAdapter`, `Handle` |
| `internal/telemetry` | Telemetry | Prompt-free JSONL; `Summarize` (the one token savings definition) |
| `internal/testenv` | Quality | Test helpers (no test files) |
| `internal/testrace` | Quality | Race detector helpers (no test files) |
| `cmd/downshift` | — | CLI, `harnessRegistry`, `runHookAdapter`, setup, doctor |
| `cmd/dsmon` | Telemetry | `events.jsonl` monitor (not the hook) |
| `cmd/dsmon-hook` | Telemetry | dsmon hook integration |

`internal/docscheck` enforces both directions:

1. Every `internal/<pkg>/` and `cmd/<pkg>/` directory that exists must appear in
   this file (the new surface table or the package map in `AGENTS.md`).
   A package that exists but is not inventoried fails the test.
2. Every package path named in `AGENTS.md` must exist on disk
   (`TestCurrentDocsNameExistingPackages`, already live).
3. `AGENTS.md` must be ≤ 200 lines (`TestAgentsMdUnderLineCap`, added in G3).

These three checks together close the gap: the old sensor only caught "doc names a
package that doesn't exist"; the new sensor also catches "package exists but no doc
names it".
