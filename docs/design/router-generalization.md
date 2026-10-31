# Router generalization (public summary)

Downshift ships a deterministic classifier (`internal/classify/signals.go`).
Maintainer regression nets live in **downshift-labs**; the public repo publishes
only [benchmark/report.md](../../benchmark/report.md).

> **Note (2026-10-31).** This document records the naive-Bayes second-opinion
> design, including the upshift-only variant and its frozen margin. The NB
> **upshift** opinion was removed from the product on 2026-10-31: Downshift never
> raises a tier on its own (the only lift is the operator tier floor). The text
> below is kept as the record of that design; only the TRIVIAL **downshift**
> opinion remains, and it is opt-in with no hook registered in production.

## What moved to maintainer eval

The following are **not** in the public repository:

- Eval-only splits: `fresh`, `heldout2`, `blind-vitrine`
- Executable **outcome** suite (40 Go tasks + recorded model runs)
- Full tables, naive-Bayes baseline, and blind-vitrine methodology

Maintainers run those checks from **downshift-labs** (private). See
[benchmark/eval-private.md](../../benchmark/eval-private.md).

## How to contribute without private access

1. Open an issue with a prompt, `downshift try` output, and expected tier.
2. Add **new** seed tasks that express the same pattern (never copy eval-only prompts).
3. Maintainer: run regression gates in downshift-labs; update `benchmark/report.md` when publishing new figures.

Classifier contribution guide: [contrib/classifier.md](../contrib/classifier.md).
