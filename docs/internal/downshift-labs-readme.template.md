# downshift-labs (private) — template

Create a **private** GitHub repository `tiagovilasboas/downshift-labs` for content
that must not ship in the public OSS repo.

## Suggested layout

| Path | Purpose |
|------|---------|
| `eval/benchmark/` | `tasks.json`, `holdout.json`, eval-only splits, `minilm-holdout.json` |
| `eval/outcomes/` | Executable outcome suite + recorded runs |
| `tools/minilm/`, `tools/baseline/` | Prototype refresh, NB baseline, holdout eval |
| `training/nbtier/` | `golden.json`, `trivial_golden.json` parity fixtures |
| `docs/maintainer/` | Full `CLASSIFIER-SHADOW`, `ENGINEERING-LOOP-MAINTAINER`, `CAPABILITY-ROUTER-V2-FULL`, `router-generalization` |
| `backlog.md`, `next-steps.md` | Execution contract, weekly checklist |
| `eval/labels/` | Production-derived training exports (redacted) |
| `commercial/` | Cloud SKU drafts, pricing, enterprise pipeline |

## CI

`.github/workflows/ci-eval.yml` checks out public `downshift`, builds CLI, runs
regression gates and outcome verify against `eval/`.

## Do not copy here

- Apache-licensed **router source** (single canonical repo: `downshift`).
- Corporate `voomp-kb` or personal vault dumps without redaction.

## Public link

[benchmark/REPORT.md](../../benchmark/REPORT.md) and [EVAL-PRIVATE.md](../../benchmark/EVAL-PRIVATE.md).
