<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Accounting v1: task cards

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Roadmap, formula and decisions: [`README.md`](README.md). Each card is one PR (spikes: one note, no
product code) of at most 5 points. "Audit" ids point to
[`../2026-10-21-curation`](../2026-10-21-curation/README.md) and its scratchpad tables. Nothing here
has been executed.

Execution premises (spikes first, parallel, MVP, compatibility, tests, one accounting source,
small commits, merge only with green CI) and the wave 1 briefing are in [`wave1.md`](wave1.md).

Every code card follows AGENTS.md: shared layers hold policy, adapters parse and emit, copyright
header on new Go files, OS-agnostic paths, table-driven tests, `go test ./...` and
`./examples/run-all.sh` green, tests isolate `DOWNSHIFT_STATE_DIR` and `HOME`.

## Spikes (phase 0; output is a dated note under `docs/evidence/` or `docs/design/`)

### S0. Validate the formula on the battery (3)
- **Question:** how far is `Saved` from the measured difference between arms?
- **Method:** replay the 46 runs of `four-arm-part1-2026-10-21` (private runs, `modelUsage` per
  model) through the formula as a throwaway script in downshift-labs: for arms C and D compute
  `C_baseline` (same tokens priced on the requested model) and `C_actual`; compare with the measured
  B−C and B−D (Claude Code's own `costUSD` is the ground truth for `C_actual`).
- **Output:** table of estimate vs measured per task, the bias in points, and a go / no-go with the
  assumptions that must be printed (README 1.1 rule 6).
- **Done when:** the note states the bias with an interval and names the tasks where it breaks.
- **Depends on:** none. **Files:** labs script, `docs/evidence/accounting-formula-validation-2026-10-21.md`.

### S1. How long does compacted output stay in context? (2)
- **Question:** is "priced once as input" too conservative to be useful?
- **Method:** from the same battery transcripts count, per Bash result inside subagents, the number
  of later turns of that agent; compute the cost of a compacted block under v1 (once) and v2
  (cache write + reads over remaining turns).
- **Output:** v1 vs v2 difference on real runs; recommendation for D2.
- **Depends on:** none. **Files:** note only.

### S2. Price coverage of the log (2)
- **Question:** for every model id seen in events (all harnesses), is there a complete price
  (input, output, cache read, cache write)? How often does a tier threshold apply (haiku-5-5 above
  100k, grok above 200k)?
- **Output:** list of unpriced and upper-bound ids, share of children affected, proposal for
  `quality` values (`exact`, `upper_bound`, `inferred`, `unpriced`).
- **Depends on:** none. **Files:** note only.

## Phase 1: spec and inputs

### T0. Spec `docs/design/accounting.md` (2)
- Content = README sections 1, 3 and 4 with the owner's decisions D1–D6 filled in, the identity
  `Saved = S_routing + S_compaction − overhead`, the quality vocabulary, and the list of
  assumptions. One line in AGENTS.md pointing to it (no growth: AGENTS.md must end ≤ 200 lines, see
  curation 4.1).
- **Done when:** the owner approves the definitions in the PR.
- **Depends on:** S0, S1, S2 (to settle D2, D4 and the quality values). **Files:** docs only.

### T1. Event fields: `harness`, `entry`, baseline (2)
- Exec events gain `harness` (from the adapter that wrapped) and `entry` (`hook` | `direct`); direct
  calls stop reading `CLAUDE_CODE_SESSION_ID` (audit S1, A22); every decision keeps `baseline_source`.
- Tests: a KiroCrew direct call is never attributed to Claude Code; old rows without the fields read
  as `entry: unknown`.
- **Depends on:** none. **Files:** `internal/execwrap/metrics.go`, `cmd/downshift/exec.go`,
  `session.go`, hook runner, docs/reference/spawn-state.md.

### T2. L1 `PriceBook` (2)
- `catalog.PriceBook{Version, AsOf}` and `PriceFor(model) (Price, Quality, error)`; `ErrUnpriced`
  for gateway ids without a snapshot (never a tier price, audit #423); cache fallback marks
  `upper_bound`; tier thresholds exposed, not applied (documented limitation from S2).
- Tests: every embedded entry resolves; unpriced stays unpriced; version stamps are stable.
- **Depends on:** S2. **Files:** `internal/catalog/*`.

## Phase 2: the core

### T3. L2 formula package (5)
- `internal/accounting/formula`: `ChildRun{Requested, Actual, Tokens, Rework[]}`, `CompactRun{BytesCut, Model}`,
  `Result{Formula, PriceVersion, Baseline, Actual, Saved, SavedPct, Routing, Compaction, Overhead,
  Unpriced, Quality}`. Pure: no I/O, no clock, no globals.
- Golden tests (table-driven): single downshift; upshift gives a negative `Saved`; held, control,
  blocked, parent-pinned, effort-only and verdict-OK-without-rewrite give 0; unpriced excluded and
  counted; rework added to `C_actual`; compaction priced at the requested model; the identity
  holds for random inputs (fuzz); `Saved` never changes when the order of children changes.
- **Depends on:** T0, T2 (types only). **Files:** `internal/accounting/formula/*`.

### T4a. L3 `Compute`: children (5)
- Select ended children by scope; fold usage records; apply the definitions of README section 4
  (held split, control, blocked, completed task, rework `nil` when no failure end was ever seen);
  build `ChildRun`; call L2. Moves `TokenSavings`, `tokenDirection`, held split and task/respawn
  logic out of `telemetry` (which keeps parsing only).
- Tests are the audit findings as cases: A2/T2 held never saved, A4 effort-only never a switch,
  A6/A7 rework rules, A9 one population, A11 model that ran, A15/T3 held counts, A16 records, A19
  per-harness measurability.
- **Depends on:** T0, T1, T2, T3. **Files:** `internal/accounting/*`, `internal/telemetry/*` (removals).

### T4b. L3 `Compute`: Compactor and scope (2)
- Compactor runs that executed (never `refused`/`not_found`), same scope as routing, `entry` and
  `harness` split in the detail; `CompactRun` priced at the session's requested model when known,
  otherwise reported in bytes and excluded from `Saved` with `unpriced`.
- Tests: scope parity (session / days / all) for routing and Compactor alike; hook vs direct.
- **Depends on:** T1, T3, T4a. **Files:** `internal/accounting/compactor.go`, `execwrap/metrics.go`.

### T4c. Display contract (1)
- One function turns `Result` into the display fields shipped in every JSON and printed by every
  text surface: `saved_pct` (one decimal), `saved_usd` (two decimals, `< $0.01` below a cent),
  `label` ("API-equivalent at list price, estimate"), `sign` (the word "saved" becomes "cost" when
  negative), `coverage` ("38 of 42 children priced"), `quality`, `scope`, `n/a` when no child is
  priced. Surfaces and the mod print these strings verbatim.
- Tests: table of values (positive, negative, zero, below a cent, rounding at .05 and .005, no
  priced child, partial coverage); the percent and the USD always come from the same `Result`.
- **Depends on:** T3. **Files:** `internal/accounting/display.go`.

## Phase 3: surfaces (render only)

### T5. `report` (3)
- Text: the four headline numbers, scope, `Saved` as percent with the API-equivalent USD beside it
  (display contract T4c, coverage and quality shown), measurability per harness, and the
  routed-vs-control table with a harness label and the interval when data suffices. `--json` is
  `Numbers` verbatim plus formula id, price version and assumptions. `--detail` adds cache reads,
  hook vs direct, the routing/compaction split of `Saved`, unpriced children.
- Removed from the default view: normalised savings, plan lens, permission-mode table.
- **Done when:** the default text is ≤ 12 lines.
- **Depends on:** T4a, T4b. **Files:** `cmd/downshift/report*.go`.

### T6. `session --panel` (2)
- Same `Numbers` for `scope=session` plus the per-spawn tree; `tokens_saved_estimate` replaced by
  `saved` carrying the T4c display fields (`saved_pct`, `saved_usd`, `label`, `quality`,
  `coverage`); `compactor.scope` follows the session.
- **Depends on:** T4a, T4b. **Files:** `cmd/downshift/sessionpanel.go`, `session*.go`.

### T7. `stats` folds into `report` (2) — only if D6 = alias
- `stats` = `report --days=N`; `--export` kept (same schema as `--json`); fixes audit A4 (shifts),
  A10 (normalised line gone), A11, A23.
- **Depends on:** T5. **Files:** `cmd/downshift/cmd_stats.go`.

### T8. savvy-progress / Agents (3)
- The mod reads `session --panel` and shows `Tokens · Saved · Compactor · Held`; no arithmetic, no
  `reduce` over event fields; band summary shows the same four; `Saved` tile prints the percent big and
  the labelled USD small, both strings from the panel JSON verbatim, plus the quality word; older binaries without `saved` show `n/a`, not a guess. Version bump through the local
  patch process (README section, `downshift-local.patch`, NOTICE).
- Tests: TypeScript tests read the golden panel JSON (T9) and assert the tiles.
- **Depends on:** T6. **Files:** `mods/savvy-progress/**`.

## Phase 4: proof

### T9. Parity test and golden log (3)
- Commit `testdata/golden/events.jsonl` + `exec-events.jsonl` (synthetic, prompt-free, covering every
  row of README section 1.1 and every harness). A Go test runs `report --json --session=<hash>` and
  `session --panel` on it and asserts the four headline numbers and the `Saved` percent and USD strings are
  byte-equal (including negative, no-baseline and partial-coverage cases); the mod's
  test consumes the same panel JSON. CI job `parity`.
- **Done when:** changing any surface's number makes CI fail.
- **Depends on:** T5, T6, T8. **Files:** `cmd/downshift/*_test.go`, `testdata/golden/*`, CI.

## Phase 5: clean

### T10. Docs align (2)
- README telemetry section, `docs/guides/report.md`, `daily-truth.md`, `stats-export.md`,
  `spawn-state.md`, `mods/savvy-progress/README.md`, `agent-reference.md` say exactly the spec,
  including that the USD is an API-equivalent estimate and not a bill (the docs that say "report
  prints no dollar amounts" are rewritten);
  `--period` and `serve` removed (audit A23–A27); skill `harness-engineer` line fixed.
- **Done when:** docscheck passes and a grep for "never summed", "tokens moved", "normalised" is empty.
- **Depends on:** T0, T9. **Files:** docs, README, skills.

### T11. Delete the legacy (2)
- Remove code left without callers after T4–T8: old `accounting` functions (audit A13), `telemetry`
  sums, the normalised-savings path, inferred honor in `doctor` (A29), the 2 MiB tail read in
  `session` (A28), effort-only "switched" labels (A5).
- **Done when:** the diff removes more lines than it adds.
- **Depends on:** T9. **Files:** as above.

### T12. Split the large telemetry files (3)
- Owner rule (2026-10-21): about 300 lines per file, never above 500. On `main` today
  `internal/telemetry/telemetry.go` has 891 lines and `session_tree.go` 514. Split each by
  responsibility, with no change in behaviour or output:
  - `telemetry.go`: event types and records (`Event`, DCI, NB, E5, shadow records), event
    constructors and outcome predicates (`FromDecision`, `Failure`, `AppliedRewrite`, ...), log
    writing (`Record`, `AppendTo`, `defaultEventPath`), log reading (`ReadEvents`, `parseEvents`,
    `FilterByDays`) and `Stats` with its tallies.
  - `session_tree.go`: building the tree (`BuildSessionTree`, `buildRow`, `foldUsage`) and rendering
    it (`Render`, `ShortReason`, `padRight`, ordering).
  - Recount `tokens_rerouted.go`, `aggregate.go` and `report.go` (326, 317 and 304 lines today)
    after T11; split any that is still above 300.
- Fix `AGENTS.md`: the architecture block calls `Summarize` "the one savings definition"; after T4
  the definition lives in `internal/accounting` (L3), so the line names that layer.
- Pure moves: no renamed exported symbol, no new package, copyright header on every new file.
- **Done when:** no non-test file in `internal/telemetry` is above 500 lines and none of the split
  files is above 300; `go test ./...`, `make e2e` and the T9 parity test are green with the same
  output; `git diff --stat` shows moves, not rewrites.
- **Depends on:** T11 (it deletes code from the same files). **Files:** `internal/telemetry`,
  `AGENTS.md`.

## Order and waves (cap 5 active agents including the root)

| Wave | Cards in parallel | Why they do not collide |
|---|---|---|
| 1 | S0, S1, S2, T1 | notes, notes, notes, execwrap/exec fields |
| 2 | T0, T2 | docs; catalog |
| 3 | T3 | formula package alone (types shared with T4) |
| 4 | T4a, then T4b; T4c in parallel with T4a | T4c is `display.go` only; T4a/T4b share the package |
| 5 | T5, T6 | different commands; T7 after T5 |
| 6 | T8 | mod |
| 7 | T9 | needs all surfaces |
| 8 | T10, T11 | docs; deletions |
| 9 | T12 | file split after the deletions; moves only |

Points: spikes 7, T0 2, T1 2, T2 2, T3 5, T4a 5, T4b 2, T4c 1, T5 3, T6 2, T7 2, T8 3, T9 3, T10 2,
T11 2, T12 3 = **46**, 19 cards. If D6 keeps `stats` separate, T7 becomes the audit fixes only and still costs 2.

## Not in this plan

Batteries, site numbers and the four-arm evidence page (resume after T9, on this formula); KiroCrew
child-end capture; the bug fixes of curation section 5 (separate PRs, #428 and #429 are open).
