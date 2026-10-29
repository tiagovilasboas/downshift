<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Accounting — Specification v1

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

**Status (2026-10-29):** the accounting is a token ledger, tokens only, forever (owner decision). It
reports three numbers and never a dollar or cost figure, not even as an estimate:

1. **Tokens spent:** observed child tokens, printed by `downshift stats` (`Observed child tokens`).
2. **Tokens saved by routing:** child tokens that ran on a cheaper model than requested
   (`telemetry.TokenSavings`; `Numbers.Routing`, section 1.1).
3. **Tokens saved by the compactor:** output avoided, saved bytes divided by four. `downshift report`
   reads them from the exec event log (`accounting.ReadExecEvents`) and passes them to `accounting.Compute`.

`downshift report` and `downshift stats` show tokens only. Go computes no dollar figure; the price
book stays as an internal routing signal (section 3). The price formula was removed (section 5), and
the dollar accounting layer in [accounting-layer-adr.md](accounting-layer-adr.md) was rejected. Sections
that mention prices or dollars are kept as rejected history. Plan history:
[`../internal/tasks/2026-10-23-spike-progress-kiro-accounting/README.md`](../internal/tasks/2026-10-23-spike-progress-kiro-accounting/README.md).

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

## 5. Price formula (removed; rejected history)

The price formula (`C_actual`, `C_baseline`, `Saved`) and its package, `internal/accounting/formula`,
were removed in the tokens-only cut. Nothing in Downshift computes or prints a dollar figure. The token
account in section 1.1 is the only saving measure; the price book in section 3 stays as a routing signal.
The formula's positive bias on extra turns went with it; section 2.1 names the token-side equivalent.

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
| T3 | ✅ merged, removed | `internal/accounting/formula` with pure functions (PR #437); removed in the tokens-only cut (section 5) |
| T4a | ✅ merged | Golden tests JSON fixtures for regression testing (PR #439) |
| T4b | ✅ merged | Per-harness aggregation using T1 fields (PR #439) |

---

## 10. References

- ADR: `docs/design/accounting-layer-adr.md` — isolated accounting layer architecture (rejected 2026-10-29)
- Evidence: `docs/evidence/accounting-formula-validation-2026-10-21.md`
- Evidence: `docs/evidence/compaction-persistence-2026-10-21.md`
- Evidence: `docs/evidence/price-coverage-2026-10-21.md`
- Catalog: `internal/catalog/catalog.json`
