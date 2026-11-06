<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Roadmap: from 8 to 9

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Written 2026-11-05, after v0.7.2 and
[tiagovilasboas/downshift#857](https://github.com/tiagovilasboas/downshift/pull/857)
(9router only, MCP `plan` tool).

## Goal

The 8/10 review named five gaps. Each stream below closes one of them, and the
roadmap is done when every exit criterion holds.

| Gap | Stream | Exit criterion |
|-----|--------|----------------|
| Planner, workers and gateway are shipped but off for a fresh install | [WK](workers-planner.md) | A fresh install delegates read-only workers with no env set, `plan` steps never fail with workers off, and `report` shows planner and worker use |
| A drop in compactor saving fails nothing | [BN](bench.md) | Cutting a family's saving or dropping a lossless signal line fails `go test`, on Linux, macOS and Windows |
| Docs name things the code lacks, and nothing fails | [DR](docs-drift.md) | Every documented command, flag, env var, MCP tool, endpoint and cited test is checked against the code in CI, and the site against the release manifest |
| Site CSP pins 575 script hashes for 1 in use; site CI checks almost nothing | [ST](site-and-pane.md) | CSP pins exactly what is used, and a required site CI job fails on drift |
| Pane never visually verified | [UI](site-and-pane.md) | A test fails on raw text outside a card or a layout shift; release QA includes screenshots |

The grade is the owner's call, not a metric. These criteria are the evidence
offered for it.

## Totals

| Stream | Tasks | Points | Repo |
|--------|-------|--------|------|
| WK: workers by default | 18 | 46 | downshift |
| BN: bench guard | 8 | 22 | downshift |
| DR: docs drift | 12 | 31 | downshift, plus DR-11 in site |
| ST: site | 8 | 16 (less if ST-5 picks A) | downshift-site |
| UI: pane | 4 | 12 | downshift |
| QF: quick fixes | 3 | 5 | both |
| **Total** | **53** | **132** | |

Every task is 5 points or less. BN-7 is split into PRs of at most 3 points.

## Quick fixes

These land first, before any guard, so the guards start green.

| ID | Fix | Pts |
|----|-----|-----|
| QF-1 | Fix the live doc drift: remove or implement `DOWNSHIFT_LOG_LEVEL` (`cli.md:486`); list all 13 MCP tools (`cli.md:411-418`) and fix "eleven" (`README.md:218`); mark `--mcp` deprecated (`cli.md:409`); check `DOWNSHIFT_FORCE_COMPACT` (`cli.md:295`) and `TestTokenBudget` (`CHANGELOG.md:42`); add the missing `cli.md` sections | 3 |
| QF-2 | Site: bump `softwareVersion` 0.6.0 to 0.7.2 at `index.html:33` and `pt-br/index.html:33`; fix the stale companions `lastmod` | 1 |
| QF-3 | Fix the false comments at `internal/workers/launch.go:86-88` and `:264-266` (the code fix is WK-01) | 1 |

## Phases

At most three PRs open at a time across the whole roadmap. Each phase lists
what can run in parallel, because their files do not overlap.

### Phase 1: quick fixes, hardening, first guards

| Parallel lane | Tasks |
|---------------|-------|
| Docs | QF-1, then DR-02, DR-03, DR-04, DR-05, DR-12 |
| Workers hardening | WK-01, WK-02, WK-03, WK-04, WK-06, WK-07 |
| Adoption baseline | WK-12a, WK-12b |
| Bench | BN-1, BN-2 |
| Site | QF-2, ST-1, ST-2; owner answers ST-5 |
| Pane | UI-1, UI-3 |

Serial: DR-02 to DR-05 start after QF-1, so they land green.

### Phase 2: resolver, real guards, site sync

| Parallel lane | Tasks |
|---------------|-------|
| Workers | WK-08, then WK-09, WK-10, WK-11 |
| Bench | BN-3 (after BN-1), BN-4, BN-5 (after BN-2) |
| Docs | DR-01, then DR-06, DR-07; DR-08 alone |
| Site | ST-4, then ST-3 or ST-6 per ST-5; ST-7 |
| Pane | UI-2 (after UI-1) |

Serial: WK-08 needs WK-03 and WK-07; DR-06 and DR-07 need DR-01.

### Phase 3: the default flip and cross-OS

| Parallel lane | Tasks |
|---------------|-------|
| Workers | WK-15 and WK-14, then WK-13, then WK-16 (docs PR) |
| Bench | BN-6, BN-7 (split), BN-8 |
| Docs | DR-09, then DR-10 |
| Site | ST-8 |
| Pane | UI-4 |

Serial: WK-13 needs everything in WK phases 0 and 1 and the owner's WK-Q1.

### Phase 4: write mode behind consent, site manifest check

| Parallel lane | Tasks |
|---------------|-------|
| Workers | WK-05, WK-17 |
| Site | DR-11 (after DR-10 and ST-5) |

## Coverage inventory per phase

Surfaces from [docs/contrib/coverage.md](../../../contrib/coverage.md). No cell
is `unknown`, so every phase is ready to implement once its decisions are in.

| Surface | Phase 1 | Phase 2 | Phase 3 | Phase 4 |
|---------|---------|---------|---------|---------|
| Routing | in: WK-04 cap in `routing/policy`, WK-07 caps in `core` | in: WK-08 resolver in `routing/policy` | in: WK-13 default constant | in: WK-05 drop list key |
| Shell | in: BN-1 fixtures and `.gitattributes` | in: BN-3, BN-4, BN-5 tests in `compressor` | in: BN-6 CI, BN-7 fixtures | out |
| Telemetry | in: WK-12a new `mcp_tool` event, no prompts or paths | out | out | out |
| Accounting | in: WK-12a excluded from spawn counts; WK-12b report line (tokens only) | out | out | out |
| Adapters | out: no adapter changes; WK-07 is caps data only | out | out | out |
| Quality | in: DR guards in `docscheck` | in: DR-06 to DR-08, BN guard | in: DR-09 manifest golden | out |
| UI | in: UI-1 guard | in: UI-2 goldens, WK-11 doctor row | in: WK-14 setup wizard cards | in: WK-17 prompt |

Workers (`internal/workers`, `internal/mcp`) sit under the MCP server, which
serves every harness with an MCP client. Harnesses without one get the
computed state through `doctor` and `report`, recorded in
`docs/reference/harness-matrix.md` (WK-16).

Invariants every phase keeps:

- Doubt stays allow. No task changes R7 or R8.
- Telemetry never stores prompts, task text, dirs or file paths. WK-12a has a
  test for it.
- `updatedInput` changes only `model` and `reasoning_effort`.
- Savings are counted in tokens only.

## Owner decisions

Implementation of a gated task waits for its answer. The rest does not.

| ID | Question | Recommendation | Gates |
|----|----------|----------------|-------|
| WK-Q1 | Read-only workers on by default, with no env, for CLIs with an observed refused write (Claude today), macOS and Linux first? | Yes | WK-13 |
| WK-Q2 | Interactive setup asks off/read/write and saves it to `~/.downshift/policy.json`; `--yes` writes nothing? | Yes | WK-14 |
| WK-Q3 | A project `policy.json` may only lower the worker mode? | Yes | WK-08 |
| WK-Q4 | Write workers start without secret-like env vars, keeping the CLI's own auth? | Yes | WK-05 |
| BN-D1 | A lowered bench floor needs a reason in the PR and a CHANGELOG line? | Both | BN-3, BN-8 |
| ST-5 | Docs source of truth: A, static HTML (delete the MDX), or B, MDX generates the HTML? | A | ST-3, ST-4, ST-6, ST-7, DR-11 |

## Out of scope

- The planner choosing between a native spawn and a worker. The orchestrator
  chooses (O8).
- Any change to adapters or to the routing rules R1 to R9.
- Write mode on by default.
- Dollar figures anywhere.

## Measure at close

Per task, record estimated points against what it took (tool calls or tokens,
where the harness shows them) in the PR, and use the gap for the next phase.
