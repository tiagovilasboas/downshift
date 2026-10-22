<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Accounting — Specification v1

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Status:** Wave 1 complete (T0-T3 merged). Next: T4a golden fixtures, T4b per-harness aggregation.

---

## 1. Definitions

### 1.1 The Saved formula

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

### 1.2 Scope

The formula measures **routing saving only**: the cost avoided by running some
work on a cheaper model than requested. It does not include:
- Compactor saving (bytes → tokens, additive, computed separately)
- Quota/overage saving (prevented overage billing, not measured here)
- Quality/capability trade-offs (not a cost; user policy, not accounting)

---

## 2. Known limitations

### 2.1 Positive bias on downshifted runs (~15%)

**Root cause:** When a child runs on a cheaper model (e.g., haiku instead of
sonnet), it often generates more tokens (more turns, more cache reads). The
formula prices those extra tokens at the baseline model rate, making `C_baseline`
larger than the true counterfactual.

**Measured bias:** +15% mean, ±7% std dev (n=9 runs with actual downshifts in
four-arm battery part 1). See `docs/evidence/accounting-formula-validation-2026-10-21.md`.

**Mitigation:** Every published `Saved` figure must carry the caveat:
> (estimate, may overstate by ~15% when routing to cheaper models)

This is a D4 design decision (inferred baselines included and labelled), not a
bug. A future v2 formula could adjust for turn-count differences, but that
requires modeling the counterfactual turn structure, which is speculative.

### 2.2 Compaction priced once as input (conservative)

Compacted output blocks are priced at input cost once, when injected into the
context. If the block stays in cache across multiple turns, the actual cost is
lower (cache read rate vs. input rate).

**Measured impact:** <$0.001 per run — below the S0 noise floor. See
`docs/evidence/compaction-persistence-2026-10-21.md`.

**Assumption to print:**
> Compactor saving is priced at input cost, once per compacted block; actual
> cost may be lower if the block stays in cache across multiple turns.

### 2.3 Gateway slugs may be unpriced

Some harnesses (e.g., KiroCrew gateway) report model slugs (`haiku`, `sonnet`,
`unknown`) instead of canonical ids (`claude-haiku-5-5`). These affect exec
event attribution but not the formula itself (which uses child `modelUsage`).

See `docs/evidence/price-coverage-2026-10-21.md` for coverage analysis.

---

## 3. Price quality vocabulary

The `PriceFor(model)` function (T2) returns a `Quality` value alongside the price:

| Quality | Meaning | Formula treatment |
|---------|---------|-------------------|
| `exact` | All four price fields (input, output, cache_read, cache_write) present | Used directly |
| `upper_bound` | Partial data; cache prices assumed zero or max | Used with caveat |
| `inferred` | Slug mapped to family member | Used with caveat |
| `unpriced` | No resolvable id | Excluded from Saved; counted separately |

**Inference rules:**
- `haiku` → `claude-haiku-5-5` (current default family member)
- `sonnet` → `claude-sonnet-5-5`
- `opus` → `claude-opus-5-5`
- `unknown`, empty, unrecognized → `unpriced`

---

## 4. Catalog requirements

The catalog (`internal/catalog/catalog.json`) must provide:

```json
{
  "prices_as_of": "2026-10-19",
  "models": {
    "claude-sonnet-5-5": {
      "input_cost_per_1m": 2.00,
      "output_cost_per_1m": 8.00,
      "cache_read_cost_per_1m": 0.10,
      "cache_write_cost_per_1m": 2.50
    }
  }
}
```

### 4.1 Tier thresholds

Some models have tier pricing (e.g., haiku-5-5 above 100k input, grok above 200k).
The current formula uses the base tier price. Future versions may track per-request
token counts and apply the correct tier, but this is out of scope for v1.

### 4.2 Staleness

The `prices_as_of` field is the last date the catalog was verified against
provider pricing pages. A catalog older than 30 days should trigger a warning.

---

## 5. Formula package (T3 implementation)

Location: `internal/accounting/formula.go`

```go
// PriceResult contains the cost and quality for a priced model.
type PriceResult struct {
    InputCost      float64 // cost for input tokens
    OutputCost     float64 // cost for output tokens
    CacheReadCost  float64 // cost for cache read tokens
    CacheWriteCost float64 // cost for cache write tokens
    Total          float64 // sum of above
    Quality        Quality // exact, upper_bound, inferred, unpriced
}

// Quality indicates the confidence level of the price.
type Quality string

const (
    QualityExact      Quality = "exact"
    QualityUpperBound Quality = "upper_bound"
    QualityInferred   Quality = "inferred"
    QualityUnpriced   Quality = "unpriced"
)

// TokenBucket represents one unit of token usage.
type TokenBucket struct {
    Model           string // model id or slug
    InputTokens     int64
    OutputTokens    int64
    CacheReadTokens int64
    CacheWriteTokens int64
}

// Saved computes the routing saving for a set of token buckets.
// Returns (actual_cost, baseline_cost, saved, quality_notes).
func Saved(requestedModel string, buckets []TokenBucket, catalog *Catalog) SavedResult
```

### 5.1 Testing

- Golden tests with known inputs → expected outputs
- Fuzz testing on token counts and model combinations
- Property: `Saved >= 0` when all buckets used the requested model
- Property: `Saved >= 0` when all buckets used cheaper or equal models

---

## 6. Exec event attribution (T1 done)

Exec events now carry `harness` and `entry` fields (PR #433, merged). This
enables per-harness aggregation in future reporting (T4b). The fields are:

| Field | Values | Source |
|-------|--------|--------|
| `harness` | `kirocrew`, `claude-code`, `codex`, `cursor`, etc. | `DOWNSHIFT_HARNESS` env var |
| `entry` | `hook`, `direct`, `unknown` | Presence of `--session=` arg |

---

## 7. Assumptions summary (for README 1.1 rule 6)

When publishing any `Saved` figure, include:

1. **Bias caveat:** "(estimate, may overstate by ~15% when routing to cheaper models)"
2. **Compaction caveat:** "Compactor saving priced at input cost once per block"
3. **Scope caveat:** "Routing saving only; excludes quota/overage and quality trade-offs"
4. **Catalog date:** "Prices as of YYYY-MM-DD"

---

## 8. Evidence

| Spike | File | Verdict |
|-------|------|---------|
| S0: Formula validation | `evidence/accounting-formula-validation-2026-10-21.md` | GO with +15% bias caveat |
| S1: Compaction persistence | `evidence/compaction-persistence-2026-10-21.md` | D2 confirmed (v1 pricing) |
| S2: Price coverage | `evidence/price-coverage-2026-10-21.md` | 100% battery coverage |

---

## 9. Implementation roadmap

| Task | Status | Description |
|------|--------|-------------|
| T0 | ✅ done | This spec |
| T1 | ✅ merged | Exec event `harness` and `entry` fields (PR #433) |
| T2 | ✅ merged | `catalog.PriceBook{Version, AsOf}` and `PriceFor(model)` (PR #437) |
| T3 | ✅ merged | `internal/accounting/formula` with pure functions (PR #437) |
| T4a | pending | Golden tests JSON fixtures for regression testing |
| T4b | pending | Per-harness aggregation using T1 fields |

---

## 10. References

- ADR: `docs/design/accounting-layer-adr.md` — isolated accounting layer architecture
- Evidence: `docs/evidence/accounting-formula-validation-2026-10-21.md`
- Evidence: `docs/evidence/compaction-persistence-2026-10-21.md`
- Evidence: `docs/evidence/price-coverage-2026-10-21.md`
- Catalog: `internal/catalog/catalog.json`
