# Accounting

Downshift's accounting layer measures what routing saves: the difference
between what you would have paid if every token ran on the requested model
and what you actually paid with routing.

## The Saved formula

```
C_actual   = Σ price(model_i, tokens_i)           # actual models and tokens
C_baseline = price(requested_model, Σ tokens_i)  # all tokens at session model
Saved      = C_baseline − C_actual
```

Where:
- `model_i` is the model that actually executed token bucket `i`
- `tokens_i` is the token count (input/output/cache_read/cache_write) for that bucket
- `requested_model` is the session's requested model (the model the user asked for)
- `price(model, tokens)` applies the catalog rates to compute cost

## Price quality

Not every model lookup returns exact pricing. The `PriceFor(model)` function
returns a quality alongside the price:

| Quality | Meaning | Treatment |
|---------|---------|-----------|
| `exact` | All four price fields present | Used directly |
| `upper_bound` | Partial data; cache prices assumed zero or max | Used with caveat |
| `inferred` | Slug mapped to family member (e.g., HAIKU → claude-haiku-5-5) | Used with caveat |
| `unpriced` | No resolvable id | Excluded from Saved; counted separately |

## Known limitations

### Positive bias on downshifted runs (~15%)

When a child runs on a cheaper model (e.g., haiku instead of sonnet), it often
generates more tokens (more turns, more cache reads). The formula prices those
extra tokens at the baseline model rate, making `C_baseline` larger than the
true counterfactual.

**Measured bias:** +15% mean, ±7% std dev (n=9 runs with actual downshifts in
the four-arm battery part 1).

**Mitigation:** Every published `Saved` figure carries the caveat:
> (estimate, may overstate by ~15% when routing to cheaper models)

### Compaction priced once as input

Compacted output blocks are priced at input cost once, when injected into the
context. If the block stays in cache across multiple turns, the actual cost is
lower (cache read rate vs. input rate).

**Measured impact:** <$0.001 per run — below the noise floor.

## Per-harness aggregation

Exec events carry `harness` (kirocrew, claude-code, cursor, etc.) and `entry`
(hook vs. direct) fields. The `AggregateByHarness()` function groups events by
harness, enabling per-harness cost and compression reporting.

## API

### PriceBook

```go
pb := catalog.NewPriceBook(catalog.Load())

result := pb.PriceFor("claude-haiku-4-5")
// result.Quality == QualityExact
// result.InputCostM, result.OutputCostM, ...

cost := result.ComputeCost(inputTokens, outputTokens, cacheRead, cacheWrite)
```

### Saved

```go
buckets := []formula.TokenBucket{
    {Model: "claude-haiku-4-5", InputTokens: 100000, OutputTokens: 50000},
}

result := formula.Saved("claude-sonnet-5-5", buckets, pb)
// result.Saved > 0 when haiku is cheaper than sonnet
// result.Quality indicates confidence (exact, mixed, partial, unpriced)
// result.IsValid() returns false if baseline is unpriced
```

### AggregateByHarness

```go
events := execwrap.LoadEvents(path)
summary := accounting.AggregateByHarness(events)
// summary.ByHarness[0].Harness == "kirocrew"
// summary.ByHarness[0].Commands == 42
// summary.ByHarness[0].SavedBytes == 123456
```

## Assumptions summary

When publishing any `Saved` figure, include:

1. **Bias caveat:** "(estimate, may overstate by ~15% when routing to cheaper models)"
2. **Compaction caveat:** "Compactor saving priced at input cost once per block"
3. **Scope caveat:** "Routing saving only; excludes quota/overage and quality trade-offs"
4. **Catalog date:** "Prices as of YYYY-MM-DD"

## Evidence

| Spike | Finding |
|-------|---------|
| S0: Formula validation | GO with +15% bias caveat |
| S1: Compaction persistence | D2 confirmed (v1 pricing) |
| S2: Price coverage | 100% battery coverage |

See `docs/design/accounting.md` for the full specification.
