# Architecture coverage inventory

Fill this table before proposing any roadmap phase that touches the binary.
A blank cell invalidates the plan. **Do not use `docs/architecture/` as a source** —
those files are historical. Read [`docs/design/architecture.md`](../design/architecture.md),
[`docs/domains/README.md`](../domains/README.md), and
[`docs/reference/capability-matrix.md`](../reference/capability-matrix.md).

## The seven surfaces

| Surface | Packages | Effect | Account |
|---------|----------|--------|---------|
| Routing | `core`, `classify`, `fuse`, `session`, `catalog`, `quota` | writes the child's model | `telemetry.Aggregate` |
| Shell | `execwrap`, `shellwrap` | rewrites the command, advises retry, or passes through | bytes, outside `Saved` |
| Telemetry | `telemetry`, `childend`, `hookport` | measures | token, no dollar |
| Accounting | `accounting` | CLI does not import; ADR Proposed | dollar, measured +15% bias |
| Adapters | `internal/adapters/*` | translates the wire | honor and usage per harness |
| Quality | `outcome`, `benchmark` | offline | acceptance suite |

**Two separate savings accounts.** The token account (`telemetry.Aggregate`) and the
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
| `internal/accounting` | Accounting | `Compute` and the numbers every surface shows; `cmd/downshift` report and session panel import it |
| `internal/adapters` | Adapters | Subdirs: `claudecode`, `cursor`, `codex`, `antigravity`, `kirocrew`, `opencode` |
| `internal/archtest` | Quality | Import-graph enforcement tests |
| `internal/benchmark` | Quality | Offline classifier accuracy |
| `internal/cascade` | Routing | Episodic failure tracking (dead-ends) for the loop runner and `exec`. The decider no longer reads it: the automatic upgrade was removed (O8) |
| `internal/catalog` | Routing | Embedded `catalog.json`, user override, Resolver, `TierView` (the tier lookup telemetry reads through `fromcore`) |
| `internal/childend` | Telemetry | Child-end recorder |
| `internal/classify` | Routing | Regex signals, language guard, risk categories (telemetry and declared floors; no built-in floor since O8) |
| `internal/compressor` | Shell | Output compressor (families, corpus, mutation-closure tests) |
| `internal/console` | UI | Full-screen terminal console: home, command list, output pane; runs every command as a child of the binary |
| `internal/console/surface` | UI | Pure decision: console or plain text for an invocation; doubt is plain |
| `internal/contextopt` | Shell | Config for compaction and headroom |
| `internal/core` | Routing | `Route`, `Check` (R1-R9), `PlanSession`, `HarnessCapabilities` |
| `internal/cursorbridge` | Adapters | Cursor native session bridge |
| `internal/discover` | Shell | `downshift discover`: transcript scan for commands that ran bare (counts and families only) |
| `internal/discovery` | Routing | `models discover/list/check/pull` |
| `internal/docscheck` | Quality | CI gates for markdown layout and package inventory |
| `internal/execwrap` | Shell | Allowlist policy, family classification, compactor runner |
| `internal/desktopview` | Shell | agent-desktop repeat reads: keeps the last compact view per window and session (0600, 10-minute TTL) and sends the line delta |
| `internal/fuse` | Routing | Raise-only semantic boost |
| `internal/gateway` | Routing | Gateway companion abstractions and adapter for 9router |
| `internal/hookctx` | Adapters | Hook deadline context |
| `internal/hookport` | Telemetry | Honor and usage ports |
| `internal/hookutil` | Adapters | Field helpers for hook payloads |
| `internal/clispec` | CLI | Command registry as data (names, summaries, flags, hidden, deprecated) and the `DOWNSHIFT_*` variables the code reads; read by help, `docs-manifest` and the docs guards |
| `internal/installer` | Setup | Tests for the root `install.sh` (offline, with fake curl and gh); no production code |
| `internal/limits` | Adapters | Centralized harness limitations registry |
| `internal/loop` | Loop | Autonomous engineering loop runner (Plan -> Implement -> Sensor -> Cascade -> Accept) |
| `internal/models` | Routing | `models` subcommand helpers |
| `internal/outcome` | Quality | Offline eval runner |
| `internal/operatorexport` | Telemetry | Operator export (roadmap P3.2): counts and token totals only, built from the `report` aggregation; `Check` is the allowlist sensor that fails on any string field or value it does not allow |
| `internal/orchestration` | Delegation | Multi-agent coordination ported from Ruflo's claude-flow: file-based swarm state, agent registry, instance pool, topology manager (star/mesh/hierarchical), and coordinator. Stdlib-only (no external imports) |
| `internal/paths` | — | Cross-platform path helpers |
| `internal/mcp` | Delegation | The MCP server. `tools`: the port, plain-function tools with no protocol knowledge. `transport`: the only package that imports the official Go SDK; edge-only, enforced by `TestInternalPackagesUseOnlyTheStandardLibrary` and `TestExternalModulesStayAtTheEdge` |
| `internal/workers` | Delegation | The queue behind `delegate`, `collect` and `cancel`: starts workers up to a limit, a time budget per job, deliveries acknowledged by sequence. `Launcher` is the port; the default refuses every agent until a read-only launch is verified (spec R27) |
| `internal/playbook` | Delegation | Named working rules a delegated worker receives (General plus an area set): shipped markdown, the person's files under the state dir, name folding, `Block` for the MCP server. No routing, no enforcement; imports only `paths` |
| `internal/plan` | Delegation | `downshift plan`: validates the parts the lead LLM wrote (agent, points, dependencies), fills tier, playbook, wave and route, renders markdown or JSON. No LLM call, no state read (the command passes the gateway switch in); imports `agenttier` and `playbook` |
| `internal/policyfile` | Routing | Project tier floor; registers via hook |
| `internal/postrun` | Shell | Post-run compaction: a finished tool's result (shell output, search matches and file lists) compacted where the harness can replace it (`ReplacesToolOutput`); reuses `execwrap.CompactCaptured` and the balanced cut. Harness-free; adapters parse and answer through `hookport.ToolOutput` |
| `internal/quota` | Routing | File I/O for quota; registers hooks |
| `internal/quotamodel` | Routing | Pure quota types, no I/O |
| `internal/routing` | Routing | Routing v2 domain (`docs/design/routing-v2.md` section 7); imports only `core`, `paths`, `routing/*` (archtest) |
| `internal/routing/contract` | Routing | v2 types only: `Request`, `Hint`, `Tier`, `Verdict`, `Reason`, `Note`, `Reading`; no logic |
| `internal/routing/policy` | Routing | Declared policy: profiles, ordered rules, quota gates, critical threshold, both-critical enum, quota TTL. Load and validate (a bad file is refused, never partly applied); project over user; `CheckProfile`. No decision; imports only `core`, `paths`, `routing/contract` (archtest) |
| `internal/routing/headroom` | Routing | Strain per provider from quota readings: worst fresh window, rate limited above any percentage, past TTL is unknown; pure, no I/O |
| `internal/semantic` | Routing | Semantic boost (BM25 / embedding-free) |
| `internal/sensor` | Telemetry | Tool-output sensor (observe-only PostToolUse) |
| `internal/session` | Routing | `SessionList`, loaders |
| `internal/setup` | Setup | `downshift setup` after argument parsing, moved out of `cmd/downshift` in steps ([setup-extraction.md](../design/setup-extraction.md)). Root: target table, `Options`, `Plan`, linked-path and binary-path guards, bridge files, backup naming, hook merge, backed-up write, summary, MCP file planners, the other planners, MCP registration, plugin install, undo, binary prune and `Run`. `configfile`: order-keeping JSON object, raw-member JSON edit, unified preview, atomic write, ownership marker; standard library only |
| `internal/shellwrap` | Shell | Shell wrap domain: `RewriteAdapter`, `AdviseAdapter`, `Handle` |
| `internal/mainwork` | Telemetry | Main-session read/search/shell calls: tool name, category, tier, harness; no text. `Observe`, `Summarize` (report row per harness) |
| `internal/telemetry` | Telemetry | Prompt-free JSONL; `Summarize` (the one token savings definition) |
| `internal/telemetry/fromcore` | Telemetry | Edge from the router to the log: `FromDecision`, `ReasonCodes`, `Install` (wires `catalog.TierViewOf` and the harness list); the only telemetry package that imports `core` |
| `internal/testenv` | Quality | Test helpers (no test files) |
| `internal/testrace` | Quality | Race detector helpers (no test files) |
| `cmd/downshift` | — | CLI, `harnessRegistry`, `runHookAdapter`, setup, doctor |
| `cmd/dsmon` | Telemetry | `events.jsonl` monitor (not the hook) |
| `cmd/dsmon-hook` | Telemetry | dsmon hook integration |
| `cmd/devcheck` | Repository gate | `make check-full` and `make check-fast`: gofmt and go vet (host, windows, linux) run at once, then `go test -count=1 ./...`; not shipped |

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

## Phase 1 (#810) surface inventory

Generic compaction chain for output of no known shape (`internal/compressor/generic*.go`,
families in `internal/execwrap`).

| Surface | In/out | Note |
|---------|--------|------|
| Routing | out | no change to model choice |
| Shell | in | the compactor chain in `execwrap` and `compressor` |
| Telemetry | out | no new event field |
| Accounting | out | compaction savings stay in the bytes account, outside `Saved` |
| Adapters | out | no adapter change |
| Quality | in | unit tests for the chain; no outcome suite change |
