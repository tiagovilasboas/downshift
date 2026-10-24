# Benchmarks (public)

Downshift ships a **small public routing sample** for CI and Harness² maturation;
full regression nets stay in **downshift-labs**.

| File | Role |
|------|------|
| [routing-tasks.public.json](routing-tasks.public.json) | **Smoke fixture** for `downshift harness2 lab` and `harness2 eval`. Not a promotion gate. |
| [harness2-lab/receipt.json](harness2-lab/receipt.json) | Receipt written by `downshift harness2 lab` and pushed with the checkout |
| [report.md](report.md) | **Published final metrics** (maintainer-updated) |
| [eval-private.md](eval-private.md) | Where raw eval data and business intelligence live |

Recursive maturation (Harness² + per-harness matrix): [../docs/guides/recursive-harnessing.md](../docs/guides/recursive-harnessing.md).

Contributors: use `downshift try`, issues with misroutes, and
`internal/core/classifier_edge_test.go`. Maintainer regression nets and outcome
suites run from **downshift-labs**.
