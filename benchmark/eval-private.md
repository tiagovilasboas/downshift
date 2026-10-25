# Private eval & business intelligence

All **benchmark JSON**, **outcome tasks**, **eval-only splits**, **training
labels from production**, and **detailed generalization studies** live in the
private **downshift-labs** repository.

The public OSS repo carries only **[report.md](report.md)**: curated final
numbers for press, README, and adopters. Update REPORT when you re-measure in
labs and choose to publish.

## Public CI

`go test`, adapter smoke tests, and classifier edge tests run here without
private datasets. Regression gates on seed/holdout run in **downshift-labs** CI.

## Harness subprocess battery (labs)

**harnesssim** in downshift-labs runs the real `downshift` binary against
documented hook JSON (Claude Code, Cursor, Codex, KiroCrew) with isolated
`events.jsonl`, plus subprocess checks for **model routing** (`try`), **compact**,
and **DCI** (`dci hook` / `inspect`). It complements in-repo `TestHarnessConformance`
and live `tools/harness-runs/`. Maintainer: `make harnesssim` in labs after building
`../downshift/downshift`. Design: labs `docs/spikes/harness-sim-battery-spike.md`.

## Policy

- Do not commit raw prompts or competitive eval artifacts to this repo.
- See [publishing-boundaries.md](../docs/reference/publishing-boundaries.md) and
  [content-classification.md](../docs/reference/content-classification.md).
