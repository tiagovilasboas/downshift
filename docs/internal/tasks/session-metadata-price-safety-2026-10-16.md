# Session metadata and unknown price safety — 2026-10-16

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

A native session can prove a model's identity, tier and supported effort without
proving its list prices. Session discovery must preserve that distinction. A
missing or zero input/output rate means an unknown price, never free usage.
Catalog aliases must not conflate different model generations to obtain prices.

## Confirmed defect and correction

Before this correction, `internal/childend/record.go` treated any
catalog-resolved model as priced. A known model with missing prices generated a
non-null $0 actual or baseline cost; the session tree could also display a $0
estimate. `SavingsRatio` and `RealSavingsUSD` could credit savings against an
unpriced target.

`telemetry.HasModelPrices` now requires positive input and output rates.
Actual and baseline costs are retained independently when their own rates are
known. Comparative savings require both; token counts remain available when
prices are unknown. Cached-token pricing retains the documented fallback to a
known input rate.

The current Codex native model `gpt-6.1-sol` was absent from the embedded catalog
at the start of this review. Its native identity is distinct from `gpt-6-sol`.
This review adds neither an alias nor invented prices. Session metadata is the
source for availability and supported effort; catalog enrichment is a separate
source for prices.

## Evidence and boundary

An overlay of the original production files reproduced failures in all four
regression tests (including $0 actual/baseline pointers, a `~$0.00` tree row and
100% savings on an unpriced target). With the correction,
`go test ./internal/childend ./internal/telemetry ./internal/catalog` passes.

Regression tests cover unpriced actuals, unpriced baselines, both absent,
partial prices, comparative savings and the session-tree estimate. These are
synthetic cases; no private task text, secrets or native session payloads are
stored here.

| Concern | Guia | Sensor | Eixo |
|---|---|---|---|
| Model identity is independent from price knowledge | This price contract, inferencial | `TestBuildUsageEvent_UnpricedModelKeepsCostsUnknown`, computacional | behaviour |
| Unknown price cannot become free spend or savings | Missing rates remain absent, inferencial | `TestFillRealCost_UnpricedModelsStayUnknown`, `TestSavingsRatio_MissingPricesStayUnknown`, computacional | behaviour |
| Cost display preserves uncertainty | Only known rates allow estimates, inferencial | `TestSessionTree_UnpricedModelDoesNotShowFreeEstimate`, computacional; absent cost fields in runtime telemetry | behaviour |

This is a scoped correction. It does not establish live executor acknowledgement,
native quotas, vendor prices, or completeness of every harness's session metadata.
