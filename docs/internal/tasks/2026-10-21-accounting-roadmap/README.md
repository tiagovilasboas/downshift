<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Accounting roadmap: one logic, one set of numbers (2026-10-21)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

> **Superseded** by [`../2026-10-21-accounting-v1/README.md`](../2026-10-21-accounting-v1/README.md)
> (adds the `net_counterfactual_cost_v1` formula as its own layer and the task cards). Kept as the
> first draft: its principles and definitions carried over.

Owner's brief: logic and standards before numbers; less is more; `downshift report`, the
savvy-progress tiles and the Agents panel must show the **same final numbers**. This roadmap is
the spike for that. Nothing here is implemented yet. Each part is one PR of at most 5 story points
and can merge alone. Facts come from the 2026-10-21 curation
([`../2026-10-21-curation/README.md`](../2026-10-21-curation/README.md)).

## 1. Principles (the standard every surface follows)

1. **One ruler.** A token is counted the way the harness counts it: `input + output + cache_write`
   of a child whose end was observed. Cache reads are a separate figure, never inside a headline.
2. **One definition per word, in one place.** `internal/accounting` is the only package that turns
   events into numbers. `report`, `session --panel`, `stats` and the mod consume its struct; none of
   them sums anything on their own.
3. **Measured beats inferred; inferred is labelled; estimated is labelled.** Three words, used
   exactly: *measured* (counted from a record), *inferred* (derived from a correlation), *estimate*
   (a proxy such as bytes / 4). A number without its word is a bug.
4. **Scope is part of the number.** Every result carries `scope` (`session`, `days`, `all`) and the
   same scope applies to routing and Compactor alike. Two scopes are never added.
5. **Downshift only claims what Downshift did.** "Moved to a cheaper model" counts a child that
   Downshift rewrote down **and** that was observed running on the smaller model. Held, control,
   blocked, parent-pinned and "verdict OK without a rewrite" never count.
6. **Less is more.** Four numbers in the headline, everywhere, in this order: Tokens, Moved cheaper,
   Compactor, Held. One final estimate line. Anything else lives in `--json` or is deleted.
7. **Fail honest.** When a figure cannot be measured in a harness or a scope, the surface prints
   *not measurable* with the reason, never zero.

## 2. Definitions (the spec `internal/accounting` implements)

| Term | Definition | Kind |
|---|---|---|
| child | one subagent spawn with a decision record; several usage records fold into the last cumulative one | measured |
| ended | a `child_ended` or usage record exists for the child; `launched` alone is not an end | measured |
| tokens | Σ over ended children of `input + output + cache_write` | measured |
| cache reads | Σ `cache_read`; shown only in `--json` and `--detail` | measured |
| moved cheaper | tokens of children with `rewrite_emitted`, verdict DOWNSHIFT, **and** `resolved` on a smaller tier than requested | measured |
| held | decisions whose rewrite was withheld by a guardrail (R1, R5, R7, R8, R9, credit gate); upshifts not applied are listed apart, never inside held | measured |
| effort-only | rewrite that changed only `reasoning_effort`: never a switch, never a saving; counted as `kept` | measured |
| control | decisions with the CONTROL code; excluded from every saving, included in tokens | measured |
| blocked | policy-mode harness refused the spawn (KiroCrew); listed, never a downshift | measured |
| completed task | ended child with usage, plus the failed attempts it redid (same session, same group, same agent type, ≤10 min) | inferred |
| rework | failed attempts / completed tasks; *not measurable* until a failure end status has been seen in the log | inferred |
| Compactor bytes cut | Σ `raw − emitted` over runs that executed (never `refused` / `not_found`), within the same scope, tagged with `entry: hook \| direct` and `harness` | measured |
| saved (estimate) | `moved cheaper tokens × (1 − price_small / price_large)` + `Compactor bytes cut / 4`. Cache reads excluded. | estimate |

Why the weight in "saved": moving 1M tokens from opus to sonnet does not save 1M tokens; it saves
the price gap. Without the weight the figure is "tokens that ran elsewhere", which is what today's
15.9M is. **Decision D1** below fixes the formula.

## 3. Architecture (single source)

```
events.jsonl, exec-events.jsonl
        │ parse only (internal/telemetry: readers, record types, no sums)
        ▼
internal/accounting.Compute(scope) → Numbers{Scope, Tokens, CacheReads, MovedCheaper, Held,
                                              UpshiftNotApplied, Control, Blocked, Compactor{Runs, BytesCut, Entry},
                                              Completed, Rework (nullable), SavedEstimate, Measurability[harness]}
        │
        ├── downshift report            (text = Numbers rendered; --json = Numbers verbatim)
        ├── downshift session --panel   (Numbers for scope=session, plus the per-spawn tree)
        ├── downshift stats             (alias of report --days=N; --detail adds cache reads)
        └── savvy-progress / Agents     (reads --panel JSON; tiles are field→label, no arithmetic)
```

Parity is a test, not a promise: a golden `events.jsonl` + `exec-events.jsonl` fixture is committed;
`report --json --session=<hash>`, `session --panel` and the mod's tile model (TypeScript test reading
the same golden JSON) must be byte-equal on the four headline numbers and the estimate.

## 4. Parts (each one PR, ≤5 points, in order)

| # | Part | What changes | Pts | Closes when |
|---|---|---|---|---|
| P0 | **Spec** | `docs/design/accounting.md` = sections 1–2 of this page, with the owner's decisions filled in; linked from AGENTS.md in one line | 2 | owner approves the definitions |
| P1 | **Event fields** | exec events gain `harness` and `entry` (hook/direct); direct calls stop borrowing `CLAUDE_CODE_SESSION_ID`; usage/end records already carry `harness` | 2 | a direct KiroCrew call is never credited to Claude Code |
| P2 | **accounting = single source** | `internal/accounting` rewritten to `Compute(scope) Numbers` per section 2; moves `TokenSavings`, `tokenDirection`, held split, task/rework logic out of `telemetry` into `accounting` (telemetry keeps parsing); table tests per definition (A2, A4, A5, A6, A7, A9, A11, A15, A16, T2, T3 from the audits become tests) | 5 | one package sums; `go test ./internal/accounting` proves every row of section 2 |
| P3 | **report consumes it** | text = four numbers + estimate + measurability lines; `--json` = `Numbers`; drop normalised savings, plan lens, "Tokens moved" prose, per-permission-mode table (to `--detail`); bootstrap interval printed when data suffices; harness named next to routed/control | 3 | report text has ≤ 12 lines by default |
| P4 | **session --panel consumes it** | `Numbers` for the session + tree; `tokens_saved_estimate` replaced by `saved_estimate`; `compactor.scope` = session | 2 | panel JSON == report JSON for the same session on the golden log |
| P5 | **stats folds into report** | `stats` = `report --days`; `--export` kept; `--detail` = cache reads, permission modes, models mix (from resolved models) | 2 | one code path |
| P6 | **mod: tiles are labels** | savvy-progress reads the four fields and the estimate; removes its own sums; tiles `Tokens · Moved cheaper · Compactor · Held`; band summary = the same four; version bump via the local patch | 3 | TypeScript parity test on the golden JSON |
| P7 | **Parity test** | golden fixtures + a Go test that runs `report --json` and `session --panel` on them and diffs the headline; CI job | 3 | green in CI; any surface drift fails the build |
| P8 | **Docs align** | README telemetry section, docs/guides/report.md, daily-truth.md, stats-export.md, spawn-state.md, mods/savvy-progress/README.md say exactly section 2; `--period` and `serve` gone | 2 | docscheck passes; grep finds no stale term |
| P9 | **Delete** | `normalised savings`, effort-only "switched", inferred honor in `doctor`, 2 MiB tail read in `session` (read by session instead), `accounting` legacy functions with no callers | 2 | fewer lines than before P2 |

Total 26 points, nine PRs, in order P0 → P9. P1 and P0 can run in parallel; P3, P4, P5 depend on
P2; P6 depends on P4; P7 depends on P3+P4+P6.

## 5. Out of scope here

Batteries, site numbers, the four-arm evidence page, KiroCrew child-end capture. They resume after
P7, on the new definitions (the battery's own metric becomes section 2's `tokens`).

## 6. Decisions for the owner

- **D1. Estimate formula.** (a) weighted by price gap (recommended, section 2), or (b) unweighted
  "tokens that ran on a cheaper model" (today's meaning, relabelled), or (c) no estimate line at all.
- **D2. `stats` becomes an alias of `report`** (recommended) or stays a separate command.
- **D3. Cache reads.** Hidden from every default view (recommended) or shown as a fifth tile.
- **D4. Completed task.** Keep the respawn-within-10-minutes inference, or count only ended children
  until a real failure end status exists.

## 7. Acceptance for the whole roadmap

1. The same session shows the same four numbers and the same estimate in `report --json`,
   `session --panel` and the Agents tiles; a CI test proves it.
2. No surface shows a number without its word (measured / inferred / estimate) and its scope.
3. `internal/accounting` is the only package with arithmetic over events; `go list` and an archtest
   rule keep `telemetry` free of sums.
4. Fewer numbers and fewer lines of code than on 2026-10-21.
