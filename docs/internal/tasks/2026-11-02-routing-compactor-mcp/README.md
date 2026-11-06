<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Routing, compactor and MCP at 9 (2026-11-02)

Goal: the three pieces that ship on Claude Code (routing, the output compactor and the MCP server) each reach a
self-assessed 9 out of 10, backed by tests and by one real measured window. Scope: Claude Code only. Milestone:
[Routing, compactor and MCP at 9](https://github.com/tiagovilasboas/downshift/milestone/1).

## Starting point (binary `v0.4.3-29-g2b194330`, `main` at `0972026e`)

| Piece | Score | What holds it back | Source |
|---|--:|---|---|
| Compactor | 8 | A `go test` run that writes to stderr streams raw; failures compact only for `go test`; `pnpm test`, `yarn test` and `make` have no failing fixture | `internal/execwrap/run.go`, `internal/compressor/go_test_failed.go`, `internal/compressor/testdata/corpus` |
| MCP | 8 | The doctor probe stops at `tools/list`; the "hooks wired" row never reads the installed plugin | `cmd/downshift/doctor_mcp.go`, `cmd/downshift/doctor_hooks.go` |
| Routing | 6 | Measured only on labelled sets; no real window (cleared on 2026-11-02); MEDIUM never moves; decisive COMPLEX precision 61% on the blind set, so `raise_complex` is off; two files frozen above 300 lines | #734, #735, #103, `internal/archtest/filesize_test.go` |
| Process | 6 | The only required check on `main` is `test`, so #735 merged with `e2e` red; `make e2e` depends on the caller's environment | branch protection, `Makefile` |

## Definition of done

- Every code card below merged with all checks green, `e2e` included.
- One seven-day window under the #103 protocol on a single fixed binary, published under `docs/evidence/` with the
  compactor section of #744.
- The two routing questions of #745 decided from that window, each with a decision record.

## Tasks

| Card | Issue | What | Points | Depends on | Wave |
|---|---|---|--:|---|--:|
| C1 | #737 | Compactor: hold `go test` output that writes to stderr; failing fixtures for pnpm, yarn and make | 5 | | 1 |
| M1 | #739 | Doctor: one MCP `tools/call`; compare the installed plugin hooks with the binary | 5 | | 1 |
| P1 | #740 | CI: require `e2e` on `main`; `make e2e` independent of harness env | 2 | | 1 |
| C2 | #738 | Compactor: failing vitest and playwright runs compacted without losing a failure | 5 | C1 | 2 |
| R1 | #741 | Code health: `classifier.go` and `claudecode.go` under 300 lines | 3 | | 2 |
| R2 | #743 | Report: rework per routing verdict against control | 3 | | 2 |
| C3 | #742 | Code health: `execwrap/policy_tooling.go` under 300 lines | 3 | C1, C2 | 3 |
| W | #103 | Seven-day routed-versus-control window, fixed binary | | all code cards | 4 |
| C4 | #744 | Evidence: compactor real savings over the same window | 1 | W | 5 |
| R3 | #745 | Routing: MEDIUM to mid and the `raise_complex` default, misroutes into a frozen holdout | 5 | W, R2 | 5 |

Total: 32 points on new cards, none over 5. At most three pull requests open at a time, so each wave holds three.
C1, C2 and C3 run in that order because they share `internal/execwrap`; the other cards share no file.

## Expected score after each wave

| Piece | After waves 1 to 3 | After the window (waves 4 and 5) |
|---|--:|--:|
| Compactor | 9 | 9, confirmed by C4 |
| MCP | 9 | 9 |
| Process | 9 | 9 |
| Routing | 7 | 9 only if neither guard of #103 gets worse |

If a guard gets worse, routing stays at 7 and the per-verdict report (R2) names the verdict to turn off.

## Out of scope

- Worker tools, the plan runner and multi-harness fan-out: off on the Claude Code setup today.
- A lexical scorer swap, stochastic selection or an automatic tier raise taken from the reference router study: the
  labelled-set numbers did not support them (maintainer labs).
- Harnesses other than Claude Code.
