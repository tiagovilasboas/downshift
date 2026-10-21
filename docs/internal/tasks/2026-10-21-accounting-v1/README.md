<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Accounting v1: `net_counterfactual_cost_v1` (roadmap, 2026-10-21)

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

Status: **plan only, nothing executed.** Task cards: [`tasks.md`](tasks.md). Wave 1 briefing and
the owner's execution premises: [`wave1.md`](wave1.md) (approved, not started). Supersedes
[`../2026-10-21-accounting-roadmap/README.md`](../2026-10-21-accounting-roadmap/README.md) (same
goal; this version adds the cost formula as its own layer). Audit trail and public-claims cleanup:
[`../2026-10-21-curation/README.md`](../2026-10-21-curation/README.md).

Owner's brief: logic and standards first; less is more; `downshift report`, the savvy-progress
tiles and the Agents panel show the **same final numbers**; the saving is a signed counterfactual
cost, computed once, in one layer.

## 1. The formula (one layer, versioned)

```
Saved = C_baseline − C_actual          (signed; negative when an optimisation costs more)

C = ( T_in·P_in + T_out·P_out + T_cr·P_cr + T_cw·P_cw ) / 10^6
    T_in  input tokens not read from cache     P_in  input price per 1M
    T_out output tokens                         P_out output price per 1M
    T_cr  cache-read tokens                     P_cr  cache-read price per 1M
    T_cw  cache-write tokens                    P_cw  cache-write price per 1M
```

Id: `net_counterfactual_cost_v1`. Prices come from the embedded catalog (`prices_as_of`, versioned);
every result carries `formula`, `price_version`, `prices_as_of`.

### 1.1 Baseline and actual, per mechanism (no double counting)

| Term | Definition |
|---|---|
| `C_actual` | measured tokens of the run, priced on the model that ran, **plus** the tokens of every failed attempt the run redid (rework is a cost, not a footnote), **plus** `overhead` (0 in v1: the routing decision calls no model; the field exists so the formula is complete) |
| `C_baseline` | the same task as if Downshift had not acted: tokens as measured, priced on the **requested** model, plus the tokens compaction removed from context |

Routing and compaction are separated by a fixed ordering, so the parts add up to the total exactly:

```
S_compaction = C( ΔT_compact , P_requested )                        ΔT_compact = (raw − emitted) bytes / 4   (estimate)
S_routing    = C( T_actual   , P_requested ) − C( T_actual , P_actual )
Saved        = S_routing + S_compaction − overhead          (identity; asserted in tests)
```

Rules that fall out of the definition (each is a golden test):

1. **Held, control, blocked, parent-pinned, kept, effort-only, "verdict OK without a rewrite"** have
   `P_requested = P_actual`, so `S_routing = 0` by construction. No special cases.
2. **An upshift is a negative saving** and is included (net, signed). Today's code ignores it.
3. **Unpriced model** (gateway ids with no price, missing cache price): the child is excluded from
   the cost figures and counted in `unpriced_children`; never priced at zero. A missing cache price
   falls back to the input rate and marks the result `quality: upper_bound`.
4. **Inferred baseline** (`baseline_source` is not `requested`): included but marks the result
   `quality: inferred`; the percentage is shown with that word.
5. **Compaction priced once, as input at the requested model** (conservative; ignores that the text
   stays in context for later turns). A multi-turn version is `v2`, only if spike S1 supports it.
6. **Equivalent result is assumed**; it is the weakest link of any counterfactual. The battery's
   acceptance rate and the measured routed-vs-control comparison (20 tasks per group) are the
   check, printed beside the estimate when available. Spike S0 measures the bias.

## 2. Layers (the standard)

```
L0 events      internal/telemetry     parse only: records, readers, privacy; no sums
L1 prices      internal/catalog       PriceBook{version, as_of}; PriceFor(model) → price + quality | ErrUnpriced
L2 formula     internal/accounting/formula   pure: ChildRun[], CompactRun[], PriceBook → Result; no I/O, no clock
L3 numbers     internal/accounting    Compute(scope) → Numbers: select events, apply definitions, call L2
L4 surfaces    report · session --panel · stats · savvy-progress   render Numbers; zero arithmetic
```

Dependency rule: each layer imports only the one below. `archtest` forbids `report`, `sessionpanel`
and the mod from importing L0/L2 directly; a lint on the mod forbids `+`/`reduce` over event fields.

## 3. Headline (less is more): four numbers, same everywhere

| # | Number | Kind | Where it comes from |
|---|---|---|---|
| 1 | **Tokens** | measured | `input + output + cache_write` of ended children (the Claude Code ruler); cache reads apart in `--detail` |
| 2 | **Saved** | estimate | percent (primary) and API-equivalent USD (secondary) on one line, with `quality`, coverage and `scope`; section 3.1 |
| 3 | **Compactor** | measured | commands that ran, bytes cut (hook vs direct apart in `--detail`) |
| 4 | **Held** | measured | guardrail holds only; upshifts not applied listed apart |

Scope (`session`, `days`, `all`) is printed with the numbers and is the same for all four. A run of
an unmeasurable harness shows `not measurable: <reason>`, never `0`.

### 3.1 Display contract (decision D1, owner, 2026-10-21)

**Percent is the primary saving metric; API-equivalent USD is the secondary one, on every surface**
(`report`, `session --panel`, `stats`, the Agents tiles, the site when it resumes). Both come from
the same `Result`, so they cannot disagree.

1. **One source, one rounding.** The percent and the USD are computed once, rounded once (percent
   to one decimal; USD to two decimals, `< $0.01` below a cent) and shipped as ready strings in the
   JSON (`saved_pct`, `saved_usd`, `display`). The mod and the text renderers print them verbatim;
   no surface rounds or divides.
2. **Always labelled.** The dollar figure is written as "API-equivalent at list price, estimate" and
   never as money saved or a bill reduction. In a subscription or corporate plan nothing is
   refunded; the label says so in `--detail` and in the docs.
3. **Signed.** A negative saving is shown as negative (upshifts, rework that cost more than it
   saved). The word "saved" becomes "cost" when the sign is negative.
4. **Missing baseline is not zero.** Children with no known requested model, or no price, leave both
   numerator and denominator and are reported as `no_baseline` / `unpriced`; the headline prints
   its coverage ("38 of 42 children priced"). A scope with no priced child prints `n/a`.
5. **No double counting.** Routing and compaction are one identity (section 1.1); the percent is
   `Saved / C_baseline` of that same baseline, never a sum of two percentages.
6. **Same percent denominator everywhere:** `C_baseline` of the priced children of the scope.

The old public rule "no dollar figures" is amended by this decision: a dollar figure may appear
only with the label of point 2 and only after phase 4 (parity proven in CI).

## 4. Definitions of the measured terms

| Term | Definition |
|---|---|
| child | one subagent spawn with a decision; several usage records fold into the last cumulative one |
| ended | usage or `child_ended` exists; `launched` alone is not an end |
| completed task | an ended child plus the failed attempts it redid (same session, group and agent type, ≤ 10 min) |
| rework | failed attempts / completed tasks; `not measurable` until a failure end status has ever been seen |
| held | rewrite withheld by R1/R5/R7/R8/R9 or the credit gate; upshift-not-applied is its own count |
| control | CONTROL code; counted in tokens, contributes `Saved = 0` |
| blocked | policy-mode refusal (KiroCrew); listed, never a switch |

## 5. Phases

| Phase | Tasks (cards in `tasks.md`) | Pts | Exit |
|---|---|--:|---|
| 0 Spikes | S0 formula vs battery, S1 compaction persistence, S2 price coverage | 7 | go / no-go on the formula, written down |
| 1 Spec + inputs | T0 spec, T1 event fields, T2 PriceBook | 6 | spec approved; events attributable; prices versioned |
| 2 Core | T3 formula layer, T4a children, T4b compactor + scope, T4c display contract | 13 | `Compute(scope)` passes the golden tests; rounding and labels fixed once |
| 3 Surfaces | T5 report, T6 panel, T7 stats fold, T8 mod | 10 | each surface renders `Numbers`, no own sums |
| 4 Proof | T9 parity test | 3 | CI fails if any surface drifts |
| 5 Clean | T10 docs align, T11 delete legacy | 4 | fewer lines than on 2026-10-21 |

Total 43 points, 18 cards, each one PR of at most 5 points. Dependencies and parallel waves are in
`tasks.md`. Public claims (retraction, site, evidence page) are not here: see the curation report,
section 1; they resume only after phase 4.

## 6. Decisions (all confirmed by the owner, 2026-10-21; S1 and S0 may reopen D2 and D4)

- **D1. Display unit. DECIDED (owner, 2026-10-21):** percent primary, API-equivalent USD secondary,
  on every surface, both from the same source (section 3.1).
- **D2. Compaction counterfactual.** Priced once as input (v1, recommended) or multiplied by the
  turns the output stays in context (needs S1).
- **D3. Upshifts.** Count them as negative saving (recommended) or keep them out of the net.
- **D4. Inferred baselines.** Include and label (recommended) or exclude from the headline.
- **D5. Control calibration.** Print the measured routed-vs-control ratio next to the estimate once
  there are 20 completed tasks per group (recommended).
- **D6. `stats`.** Becomes `report --days` (recommended) or stays a separate command.

## 7. Acceptance for the whole roadmap

1. The same session shows the same four numbers in `report --json`, `session --panel` and the
   Agents tiles; a CI test on a committed golden log proves it.
2. `Saved` is one function of one layer; its identity (`routing + compaction − overhead`) and its
   zero cases are tests.
3. No surface prints a number without its kind (measured, inferred, estimate), its scope and, for
   `Saved`, its quality.
4. The formula, price version and assumptions are printed by `report --detail` and `--json`.
   The percent and the USD of the same scope are identical strings on every surface, the USD
   always carries its "API-equivalent, estimate" label, negatives stay negative, and a missing
   baseline prints coverage or `n/a`, never zero.
5. Fewer numbers and fewer lines of code than on 2026-10-21.
