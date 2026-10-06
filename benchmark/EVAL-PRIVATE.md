# Private eval & business intelligence

All **benchmark JSON**, **outcome tasks**, **eval-only splits**, **training
labels from production**, and **detailed generalization studies** live in the
private **downshift-labs** repository.

The public OSS repo carries only **[REPORT.md](REPORT.md)**: curated final
numbers for press, README, and adopters. Update REPORT when you re-measure in
labs and choose to publish.

## Public CI

`go test`, adapter smoke tests, and classifier edge tests run here without
private datasets. Regression gates on seed/holdout run in **downshift-labs** CI.

## Policy

- Do not commit raw prompts or competitive eval artifacts to this repo.
- See [publishing-boundaries.md](../docs/publishing-boundaries.md) and
  [content-classification.md](../docs/content-classification.md).
