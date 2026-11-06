# Accounting

> **Status: in development.** It works, but its interface may change between releases.

Downshift's accounting layer measures tokens. It counts the tokens that ran below
the session model and the tokens that compaction kept out of the context
window, per harness, never summed across harnesses. It reports tokens only and never converts them to money.

## The token account

```text
Routing tokens   = Σ billable tokens (input + output + cache write) of the spawns of one harness
                   that ran on a lower tier than the session model; cache read apart
Compactor tokens = Σ max(raw bytes − emitted bytes, 0) / 4     # estimate: one token per 4 bytes, per harness
(no sum of the two, and no sum across harnesses)
```

- **Billable tokens** are a child's input, output and cache write tokens. Cache read is a
  separate field and is never inside billable or below.
- A spawn counts as below when its model ranks lower than the session model
  (`parent_model`); with no usable session model the requested model is the
  labelled fallback, kept in separate fields. Its whole token count counts; the account does not subtract
  what the baseline model would have used.
- Routing tokens come from each child's usage record. Compactor tokens are an
  estimate from the bytes of command output.

## What the account does not measure

- **Money.** No figure is a price or a cost. Catalog prices are an internal
  routing signal (see [Price book](#price-book)).
- **Extra turns.** A lower-tier child can take more turns than the requested model
  would. The account counts its tokens as moved and does not model that difference.
- **Quality and capability trade-offs.** That is a policy question, not accounting.
- **Quota and overage.**

Routing tokens therefore mean tokens that ran below the session model, not tokens avoided
against it. Of those, the tokens whose spawn a Downshift rewrite moved are counted apart
(`rewritten_spawns`, `rewritten_billable`; the line `of which by a Downshift rewrite`): a
spawn that already ran lower, by the caller's or the session's choice, is in the first figure
and not in the second.

## Scopes

`accounting.Compute` takes one scope for every surface: `ScopeDays` (last N days),
`ScopeSession` (one hashed session id) or `ScopeAll`. The result carries the routing
tokens (`Routing`, a per-harness `telemetry.RoutingView`), the compactor totals and `TokensSavedEstimate`, the compactor tokens alone (routing tokens ran below the session model and were spent).

## Worker tokens

`Ledger.Workers` (JSON `workers`, under `token_ledger` in `stats --export` and
`numbers.ledger` in `report --json`) lists what the MCP delegate workers spent per CLI
(`claude`, `codex`): `workers`, `input`, `output`, `cache_read`, `cache_write`,
`total`. It comes from the `worker_ended` events over the ledger window, and
`downshift stats` prints it as `Workers (additional to routed spawns): ...`. These tokens are
**additional to** the routing tokens, not included: those count routed-spawn usage
events, and worker events are skipped, so nothing is counted twice. The key is absent
when no worker ended in the window; a session scope has none (worker events carry no
session). It only measures: the worker token budget stays opt-in and off.

## Per-harness aggregation

Exec events carry `harness` and `entry` (hook or direct). `accounting.AggregateByHarness`
groups the compactor totals by harness. Telemetry events carry the harness too.

## Price book

The catalog (`internal/catalog/catalog.json`) keeps model prices as an internal
routing signal. `catalog.PriceBook.PriceFor(model)` returns a price and a quality:

| Quality | Meaning |
|---------|---------|
| `exact` | All four price fields present |
| `upper_bound` | Partial data; cache prices assumed zero or max |
| `inferred` | Slug mapped to a family member (e.g., HAIKU → claude-haiku-5-5) |
| `unpriced` | No resolvable id |

No accounting output shows a price or a quality value.
The old price formula (`Saved`, package `internal/accounting/formula`) was removed; no code computes a dollar figure.

## API

```go
// One scope: routing and compactor tokens, plus the text that surfaces print.
n := accounting.Compute(events, execEvents, accounting.Scope{Kind: accounting.ScopeDays, Days: 7})
// n.TokensSavedEstimate == n.Compactor.Tokens

// Routing tokens from telemetry usage events.
routing := telemetry.RoutingTokens(events) // routing.ByHarness, no total

// Compactor totals per harness from exec events.
summary := accounting.AggregateByHarness(execEvents)
```

## Caveats to print

When publishing a token figure, include:

1. **Compactor caveat:** "Compactor tokens are an estimate: bytes saved divided by four."
2. **Routing caveat:** "Routing tokens are tokens that ran below the session model. They count the whole child, without modelling its extra turns."
3. **Scope caveat:** "Tokens only, no money. Excludes quota, overage and quality trade-offs."

## Evidence

| Spike | Finding |
|-------|---------|
| S0: Formula validation | GO with +15% bias caveat (measured the price formula, now retired from reporting) |
| S1: Compaction persistence | D2 confirmed (v1 pricing) |
| S2: Price coverage | 100% battery coverage |

See `docs/design/accounting.md` for the full specification.
