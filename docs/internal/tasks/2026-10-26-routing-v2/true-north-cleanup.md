<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# True north, part 1: what we delete before v2 starts (2026-10-26)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Branch `downshift/true-north`. Owner decision of 2026-10-26: rebuild model routing from scratch, keep the
Compactor and DCI, and leave no leftovers of the v1 experiments. Design: [`../../../design/routing-v2.md`](../../../design/routing-v2.md).
Inspiration: Anthropic's agent cookbooks, mainly *Plan big, execute small* (design §11).

Evidence: `go list -test -f '{{.ImportPath}} {{.Imports}}' ./...` on `origin/main` 800a8e4e, reverse
imports per package, line counts with tests. Nothing is deleted yet; this is the list to approve.

## Keep (they do not depend on the routing decider)

| Surface | Packages | Proof |
|---|---|---|
| Compactor | `execwrap`, `compressor`, `shellwrap` | `execwrap` imports only `compressor`, `paths`; `compressor` imports nothing internal. `shellwrap` reads only `core.CapsFor` / `HarnessCapabilities` (capability data, kept) |
| DCI | `codeintel`, `contextopt`, `sensor`, `benchmark` | `codeintel` imports only `paths`; `contextopt` and `sensor` import `compressor`, `paths`, `core` caps |
| Hook plumbing | `adapters/*`, `hookport`, `hookutil`, `hookctx`, `childend`, `lifecycleobserver`, `paths`, `testenv`, `testrace` | v2 reuses the wire layer as is (design §3) |
| Data | `catalog`, `quota`, `quotamodel`, `limits`, `models`, `discovery`, `policyfile` | headroom (R3) and candidates (R0) read them |
| Measure | `telemetry`, `outcome`, `api`, `accounting` (paused plan) | outcome is the rigor-matched control the cookbook asks for |
| Guards | `archtest`, `docscheck` | invariants; rules about deleted packages are removed with them |

## Wave A: delete now (dead or experiment-only; nothing kept imports them)

| Package / file | Lines (src+test) | Who imports it today | Also delete |
|---|--:|---|---|
| `internal/routingv2` (shadow v2) | 4764 | nobody | `docs/design/capability-router-v2.md` |
| `internal/scoreindex` | 1871 | `cmd/downshift` config.go, scores.go | `scores*.go`, `scorerefresh*.go` |
| `internal/adapt` | 1398 | `cmd` adapt_eval.go, `routeadapt`, `routingv2/training` | `adapt_eval.go`, `eval-adapt` command, `docs/advanced/adaptive-memory.md` |
| `internal/routeadapt` | 74 | `cmd` main.go | — |
| `internal/nbopinion` | 55 | `cmd` main.go | — |
| `internal/decisionintelligence` | 385 | `cmd` hookrun.go | `docs/design/decision-intelligence.md` |
| `internal/replay` | 873 | `cmd` cmd_replay.go | `replay` command, `docs/advanced/replay.md` |
| `cmd/downshift`: `shadow*.go`, `verification*.go`, `cmd_feedback*.go`, `train` | — | main.go switch | `shadow-report`, `verification-report`, `feedback`, `train` commands; `docs/advanced/shadow-mode.md`, `docs/design/classifier-shadow.md` |
| `orchestration/` (Python LangGraph planner) | — | nothing in the binary | `docs/design/langgraph-orchestration.md`, its dependabot entry |
| `tools/hashv2`, `tools/multilang` | — | `multilang` imports `semantic` | `tools/README.md` NB lines |

About 9.4k Go lines plus docs. Every row is checked by `make check` after the cut; a row that breaks a
kept package moves to wave B.

## Wave B: delete with the v1 decider (same branch, after wave A is green)

`internal/classify` (3520), `internal/fuse` (146), `internal/semantic` (743), `internal/session` (850),
`internal/nbtier` (318), the decider part of `internal/core` (`Route`, rules R1–R9, NB files), `cmd`
`cmd_try.go` and the `semantic` lines of config.go / doctor.go.

Until R2–R6 land, `core.Route` becomes the last step of the v2 precedence: **keep the requested model**.
That is the honest start-over state: hooks still fire and fail open, the Compactor and DCI are untouched,
and no spawn changes model until a profile or hint is declared. The risk scanner from `classify` moves into
`routing/guard` (R5) before `classify` goes.

Coupling to cut first: `telemetry` imports `core`, `classify`, `semantic`, `fuse`, `session` for decision
types. Telemetry gets its own enum types (the routing note, R11) so it stops importing the decider.
`catalog` imports `gateway` for slug lookup; decide whether gateway pricing stays (D-T2).

## Decisions for the owner

- D-T1. `gateway` (OpenRouter pricing shadow, 676 lines): delete with wave A, or keep for `report` pricing.
- D-T2. `accounting`: keep paused as is (recommended) or delete with the plan.
- D-T3. `docs/architecture/*` and `docs/archive/*` (historical): delete, or keep under archive with a banner.
- D-T4. Release note: wave B makes routing a no-op until v2 lands; ship from this branch only after R6.

## Order on this branch

1. Wave A, one commit per row, `make check` green after each.
2. Telemetry decoupling (enum note types), then wave B.
3. R0, R2–R6 from the v2 roadmap on top of the clean base.
