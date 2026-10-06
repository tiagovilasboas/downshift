# Router generalization (public summary)

Downshift ships a deterministic classifier (`internal/core/signals.go`).
Maintainer regression nets live in **downshift-labs**; the public repo publishes
only [benchmark/REPORT.md](../../benchmark/REPORT.md).

## What moved to maintainer eval

The following are **not** in the public repository:

- Eval-only splits: `fresh`, `heldout2`, `blind-vitrine`
- Executable **outcome** suite (40 Go tasks + recorded model runs)
- Full tables, naive-Bayes baseline, and blind-vitrine methodology

Maintainers run those checks from **downshift-labs** (private). See
[benchmark/EVAL-PRIVATE.md](../../benchmark/EVAL-PRIVATE.md).

## How to contribute without private access

1. Open an issue with a prompt, `downshift try` output, and expected tier.
2. Add **new** seed tasks that express the same pattern (never copy eval-only prompts).
3. Maintainer: run regression gates in downshift-labs; update `benchmark/REPORT.md` when publishing new figures.

Classifier contribution guide: [contrib/classifier.md](../contrib/classifier.md).
