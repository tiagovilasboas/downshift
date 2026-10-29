# Accounting

Downshift's accounting layer measures tokens. It counts the tokens that routing
moved to a cheaper model and the tokens that compaction kept out of the context
window, per harness. It reports tokens only and never converts them to money.

## The token account

```text
Routing tokens   = Σ (work + cache tokens) of child spawns that ran on a cheaper model than requested
Compactor tokens = Σ max(raw bytes − emitted bytes, 0) / 4     # estimate: one token per 4 bytes
Tokens saved     = routing tokens + compactor tokens
```

- **Work tokens** are a child's input and output tokens. **Cache tokens** are its
  cache read and cache write tokens.
- A child counts as routing savings when its model is cheaper than the one the
  harness requested. Its whole token count counts; the account does not subtract
  what the requested model would have used.
- Routing tokens come from each child's usage record. Compactor tokens are an
  estimate from the bytes of command output.

## What the account does not measure

- **Money.** No figure is a price or a cost. Catalog prices are an internal
  routing signal (see [Price book](#price-book)).
- **Extra turns.** A cheaper child can take more turns than the requested model
  would. The account counts its tokens as moved and does not model that difference.
- **Quality and capability trade-offs.** That is a policy question, not accounting.
- **Quota and overage.**

Routing tokens therefore mean tokens moved to a cheaper model, not tokens avoided
against the requested model.

## Scopes

`accounting.Compute` takes one scope for every surface: `ScopeDays` (last N days),
`ScopeSession` (one hashed session id) or `ScopeAll`. The result carries the routing
tokens, the compactor totals and `TokensSavedEstimate`, the sum of both token parts.

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
// n.TokensSavedEstimate == n.Routing.WorkSaved + n.Routing.CacheSaved + n.Compactor.Tokens

// Routing tokens from telemetry usage events.
routing := telemetry.TokenSavings(events).Total

// Compactor totals per harness from exec events.
summary := accounting.AggregateByHarness(execEvents)
```

## Caveats to print

When publishing a token figure, include:

1. **Compactor caveat:** "Compactor tokens are an estimate: bytes saved divided by four."
2. **Routing caveat:** "Routing tokens are tokens that ran on a cheaper model. They count the whole child, without modelling its extra turns."
3. **Scope caveat:** "Tokens only, no money. Excludes quota, overage and quality trade-offs."

## Evidence

| Spike | Finding |
|-------|---------|
| S0: Formula validation | GO with +15% bias caveat (measured the price formula, now retired from reporting) |
| S1: Compaction persistence | D2 confirmed (v1 pricing) |
| S2: Price coverage | 100% battery coverage |

See `docs/design/accounting.md` for the full specification.
