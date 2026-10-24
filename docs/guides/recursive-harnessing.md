# Recursive harness lab (local) — Downshift + Harness²

Downshift by Tiago Vilas Boas  
https://github.com/tiagovilasboas/downshift

## Intent

Downshift is already in production on harnesses marked **Completed** in
[../reference/harness-matrix.md](../reference/harness-matrix.md). The lab matures
that matrix **in the git checkout**, on the maintainer machine. It does not run
inside CI and it does not edit routing policy by itself.

Google [Harness²](https://github.com/google-research/harness2) stays an optional
outer install in [harness-core](https://github.com/tiagovilasboas/harness-core)
(`features/recursive-harnessing/`). This repository owns the lab contract.

## Roles

| Matrix status | Lab role | Mutation |
|---|---|---|
| Completed | `mutate` | Eligible when the binary has a hook. Today: `claude-code`. |
| Functional, Beta, Experimental | `observe` | Same routing is computed. No recursive edits until the status becomes Completed. |
| Config only | `classifier_only` | `try` and reasoning only. Today: `grok`. |

The status table in the harness matrix is the source of truth.
`internal/harness2export` is the sensor: the test fails if the Go catalog drifts.

## Commands

```bash
downshift harness2 matrix
downshift harness2 lab
downshift harness2 eval benchmark/routing-tasks.public.json --json
```

`harness2 lab` judges the role invariant, runs the public dataset as a **smoke
fixture**, and writes `benchmark/harness2-lab/receipt.json` into this checkout.
That file is the lab effect. Commit and push it so the remote project carries
the same result. The command does not change classifier weights, hooks, or policy.

`benchmark/routing-tasks.public.json` is a CLI smoke file. Promotion still uses
the authorized dataset in **downshift-labs**, `make check`, the shared
conformance suite, and `internal/core/classifier_edge_test.go`.

## Promotion

1. Raise a harness to Completed in the matrix only with rewrite-honored evidence. The lab role follows on the next `harness2 lab`.
2. A Harness² proposal is a diff on a copy. A human opens the pull request.
3. The remote effect is the pushed commit, including the receipt. There is no auto-merge.
