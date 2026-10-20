# Domain-routing precision: bilingual evaluation (2026-10-20)

## Scope and limits

This is a **routing-tier label agreement** experiment, not a measurement of model
task completion, real-world quality, token consumption or total cost. The new
corpus was authored to cover the 96 developer-domain rules, so its uplift
cannot be generalized to arbitrary real user requests. The EN, PT-BR and
mixed-language variants of each scenario are correlated and must not be
treated as 90 independent problems.

- **New targeted suite:** 30 development scenarios, 3 language variants each,
  90 examples; 30 expected `small`, 30 expected `mid`, 30 expected
  `frontier`. Labels were assigned by intended task scope before testing.
- **Original control:** 200 existing general-purpose corpus examples, including
  previously identified known gaps.
- **Decision path:** production `core.Route` in Go with the same resolver,
  comparing `DOWNSHIFT_DOMAIN_SIGNALS=0`,
  96 domains with `DOWNSHIFT_DOMAIN_INTERACTIONS=0`, and 96 domains with
  cross-domain interactions enabled. Domain deduplication is enabled in all
  three configurations (irrelevant when domains are disabled).
- **Execution:** GitHub Actions [diagnostic run #1201](https://github.com/tiagovilasboas/downshift/actions/runs/39439045990).
  This *intentionally failed* only to reveal Go's normally hidden `t.Logf`
  output. The temporary diagnostic test failures were subsequently removed.
- **Source:** `internal/core/testdata/domain_precision/dev_tasks.json` and
  `internal/core/domain_precision_eval_test.go`.

## Results

| Evaluation | Baseline, no domains | 96 domains, no interactions | 96 domains + interactions |
| --- | ---: | ---: | ---: |
| Targeted tier agreement (90) | 56/90 (62.2%) | 84/90 (93.3%) | 84/90 (93.3%) |
| Targeted under-routing | 25 | 0 | 0 |
| Targeted over-routing | 9 | 6 | 6 |
| Expected `small` correctly selected | 24/30 | 24/30 | 24/30 |
| Expected `mid` correctly selected | 27/30 | 30/30 | 30/30 |
| Expected `frontier` correctly selected | 5/30 | 30/30 | 30/30 |
| EN/PT-BR/mixed tier parity (30 scenarios) | 26/30 | 28/30 | 28/30 |
| Original corpus tier agreement (200) | 140/200 (70.0%) | 141/200 (70.5%) | 141/200 (70.5%) |
| Original corpus under-routing | 26 | 25 | 25 |
| Original corpus over-routing | 34 | 34 | 34 |

On the targeted suite, the new vocabulary corrected **28 predictions** and
regressed **zero**; the difference is **+31.1 percentage points** of tier-label
agreement. Most recovered scenarios were specialized complex engineering
requests previously assigned `mid`. Three observability examples were
corrected from `frontier` to `mid`.

The original 200-case general corpus improved by **one** correctly labeled
case (**+0.5 percentage points**). On these two fixed corpora, the additional
pairwise mathematical interaction bonus **changed zero final tier decisions**.
Its numerical formula and counterfactual score behavior have separate unit
tests, but **there is no demonstrated tier-accuracy uplift attributable to
interactions yet**.

## Interpretation and follow-up

The strong targeted result is **evidence that the added domain vocabulary
recognizes this curated set of engineering tasks**, not proof of an equivalent
general improvement. In particular:

1. The targeted labels are hand-curated, not verified by actual execution
   across candidate LLMs. A model classified as `frontier` may not be
   necessary to complete every labeled task.
2. Most targeted examples use terminology supported by the new lexicons;
   an independent naturally occurring evaluation set is needed to assess
   precision, recall and over-routing on unseen prompts.
3. The translations share a scenario and are not 90 independent samples.
4. No new benchmarks of p95 latency, memory, tokens, cost per successful
   task, or model completion were executed for this report.
5. The cross-domain bonus should be re-evaluated on genuinely multi-domain
   requests before claims of mathematical improvement.

### Reproduce

```sh
go test -v -count=1 ./internal/core -run TestDomainPrecision
go test ./internal/classify ./internal/core
go test -bench BenchmarkDomainPrecisionRoute -run '^$' ./internal/core
```

The `TestDomainPrecisionComparison` test guards against falling below the
no-domain baseline in tier agreement or raising under-routing on the targeted
fixture. Avoid tuning future weights against the same examples and then using
them as an unbiased holdout.
