<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Accounting — Specification v1

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Status (2026-10-27):** owner decision O1 makes the accounting account tokens only. This spec
defines the routing and compactor token figures (§1). The price formula (§5) is kept as an internal
pure function with no production caller and is not reported. Go surfaces that still print dollars
are tracked separately, so until they change this spec describes the intended output, not the
current one. Plan history: [`../internal/tasks/2026-10-23-spike-progress-kiro-accounting/README.md`](../internal/tasks/2026-10-23-spike-progress-kiro-accounting/README.md).

---

## 1. Definitions

### 1.1 Token account

```text
Routing tokens   = Σ (work + cache) tokens of child spawns with a cheaper model than requested
Compactor tokens = Σ max(raw_bytes − emitted_bytes, 0) / 4        # estimate
Tokens saved     = routing tokens + compactor tokens               # TokensSavedEstimate
```

- Work tokens: a child's input and output tokens. Cache tokens: cache read and cache write tokens.
- Direction: `telemetry.TokenSavings` compares the requested model with the model that ran. Only a
  cheaper child adds to routing savings.
- `accounting.Compute` returns the account for one scope: `ScopeDays`, `ScopeSession` or `ScopeAll`.

### 1.2 Scope

The account measures **routing and compaction tokens only**. It does not include:
- Any money figure (no price, cost or saving in currency)
- Quota/overage (not measured here)
- Quality/capability trade-offs (user policy, not accounting)
- Turn-count differences (see §2.1)

---

## 2. Known limitations

### 2.1 No counterfactual for extra turns

**Root cause:** a cheaper child often takes more turns (more tokens, more cache reads). The account
counts its whole token count as moved and does not model what the requested model would have spent.

**Consequence:** routing tokens measure tokens moved to a cheaper model, not tokens avoided against
the requested model. Do not publish them as a saving against the requested model without the caveat
in §7.

A future version could model the counterfactual turn structure, but that requires speculative
modeling and is out of scope for v1.

### 2.2 Compactor tokens are an estimate

Compactor tokens are bytes saved divided by four (`execwrap.BytesPerToken`). Bytes are measured;
the token count is an approximation.

### 2.3 Gateway slugs may be unknown

Some harnesses (e.g., KiroCrew gateway) report model slugs (`haiku`, `sonnet`, `unknown`) instead of
canonical ids (`claude-haiku-5-5`). A gateway child whose slug the catalog does not recognise is
counted as gateway-unattributed (`GatewayUnattributed`) and adds nothing to routing savings.

See `docs/evidence/price-coverage-2026-10-21.md` for coverage analysis.

---

## 3. Price book (internal routing signal)

The catalog (`internal/catalog/catalog.json`) keeps model prices as an internal routing signal.
Prices are not reported. `catalog.PriceBook.PriceFor(model)` returns a price and a `Quality`:

| Quality | Meaning |
|---------|---------|
| `exact` | All four price fields (input, output, cache_read, cache_write) present |
| `upper_bound` | Partial data; cache prices assumed zero or max |
| `inferred` | Slug mapped to family member |
| `unpriced` | No resolvable id |

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
The price book uses the base tier price. Future versions may track per-request token counts and
apply the correct tier, but this is out of scope for v1.

### 4.2 Staleness

The `prices_as_of` field is the last date the catalog was verified against provider pricing pages.
A catalog older than 30 days should trigger a warning.

---

## 5. Price formula (retired from reporting)

`internal/accounting/formula` implements the price formula as pure functions. No report prints its
result, and no production code calls it.

```text
C_actual   = Σ price(model_i, tokens_i)
C_baseline = price(requested_model, Σ tokens_i)
Saved      = C_baseline − C_actual
```

- Entry point: `formula.Saved(requestedModel, buckets, pb)` returns `SavedResult` with `ActualCost`,
  `BaselineCost` and `Saved` in catalog price units. The JSON tags still end in `_usd` (legacy
  names); the values are not reported money.
- The formula prices a cheaper child's extra turns at the baseline rate, which causes the +15%
  positive bias recorded in the S0 evidence (see §2.1 for the token-side equivalent).

### 5.1 Testing

- Golden tests with known inputs → expected outputs (`formula/testdata/golden_cases.json`)
- Fuzz testing on token counts and model combinations
- Property: `Saved >= 0` when all buckets used the requested model
- Property: `Saved >= 0` when all buckets used cheaper or equal models

---

## 6. Exec event attribution (T1 done)

Exec events now carry `harness` and `entry` fields (PR #433, merged). This enables per-harness
aggregation (`accounting.AggregateByHarness`). The fields are:

| Field | Values | Source |
|-------|--------|--------|
| `harness` | `kirocrew`, `claude-code`, `codex`, `cursor`, etc. | `DOWNSHIFT_HARNESS` env var |
| `entry` | `hook`, `direct`, `unknown` | Presence of `--session=` arg |

---

## 7. Assumptions summary

When publishing a token figure, include:

1. **Compactor caveat:** "Compactor tokens are an estimate: bytes saved divided by four."
2. **Routing caveat:** "Routing tokens are tokens that ran on a cheaper model. They count the whole child, without modelling its extra turns."
3. **Scope caveat:** "Tokens only, no money. Excludes quota, overage and quality trade-offs."

---

## 8. Evidence

| Spike | File | Verdict |
|-------|------|---------|
| S0: Formula validation | `evidence/accounting-formula-validation-2026-10-21.md` | GO with +15% bias caveat (measured the price formula, retired from reporting) |
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
| T4a | ✅ merged | Golden tests JSON fixtures for regression testing (PR #439) |
| T4b | ✅ merged | Per-harness aggregation using T1 fields (PR #439) |

---

## 10. References

- ADR: `docs/design/accounting-layer-adr.md` — isolated accounting layer architecture
- Evidence: `docs/evidence/accounting-formula-validation-2026-10-21.md`
- Evidence: `docs/evidence/compaction-persistence-2026-10-21.md`
- Evidence: `docs/evidence/price-coverage-2026-10-21.md`
- Catalog: `internal/catalog/catalog.json`
