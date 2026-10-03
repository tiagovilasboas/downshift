# Benchmark seed dataset

## What this is

`tasks.json` is a curated 200-task seed dataset (50 TRIVIAL, 50 SIMPLE, 50 MEDIUM, 50 COMPLEX)
used as a **regression net** for the downshift classifier: it catches
accidental behaviour changes between edits. Each entry is a
`{prompt, label}` pair; run it with `downshift benchmark benchmark/tasks.json`.

## What this is not

It is **not proof of generalisation**. Curated prompts cannot
represent all real-world traffic, and accuracy on this file says nothing about
unseen tasks. Do not quote seed accuracy as a quality claim; treat a green
run as "no obvious regression", nothing more.

## Label rubric

**TRIVIAL** — mechanical work with no design decision: a rename, a typo
fix, running a command, or deleting dead code. Examples: "fix the typo in
the README" and "run git status".

**SIMPLE** — one small, well-scoped change inside existing structure: a
new field, a single-function fix, basic validation, or one unit test.
Examples: "add a new field email to the User struct" and "write a unit
test for the formatDate function".

**MEDIUM** — multi-file work reusing existing architecture: a feature
behind current patterns, a refactor of one module, or adding a
cross-cutting concern like caching or retry. Examples: "implement dark
mode toggle using the existing theme context" and "add caching layer
using Redis to the product catalog queries".

**COMPLEX** — open-ended engineering with unknown unknowns: rearchitecting
a subsystem, distributed-systems design, or debugging a race, leak, or
failure under load. Examples: "rearchitect the authentication system to
support multi-tenant organisations" and "investigate and fix the race
condition in the subscription renewal worker".

## Contribution protocol

1. Prefer **real tasks** from actual usage over invented ones.
2. No synthetic paraphrases of existing rows; each task must add new signal.
3. Keep labels balanced: new tasks should fill the scarcest label first.
4. Check health before submitting: no empty prompts, no duplicates
   (case-insensitive trim), no unknown labels — see `DatasetHealth`.
5. Run `downshift benchmark` before and after your change and paste both
   confusion matrices in the PR description.

## Expansion roadmap

- **Held-out split.** Reserve a subset contributors never see during
  tuning, so reported accuracy is honest (`benchmark/holdout.json`).
- **500-1000 task target.** Grow the seed with reviewed real tasks until
  per-label counts support stable metrics.
- **Statistical reporting.** Added 95% bootstrap confidence intervals for Tier Accuracy via `downshift benchmark <file> --report`:
  ```json
  {
    "total_tasks": 200,
    "tier_accuracy": 0.74,
    "tier_accuracy_ci_95": {
      "low": 0.68,
      "high": 0.795
    }
  }
  ```
  The interval bounds prevent mistaking variance across small sample sizes for genuine classifier regressions or gains.

## Fresh split (`fresh.json`): evaluation only

`fresh.json` holds 100 hand-written, non-templated prompts (25 per label, same
rubric) that are **never used to write or tune signals**. It measures how the
router generalises; `holdout.json` stays as a templated regression net.

Rules, enforced by `internal/benchmark/fresh_guard_test.go`, the CI step
`Fresh split guard` and `scripts/fresh-guard.sh`:

- No fresh prompt may appear, even as a near-duplicate (token Jaccard ≥ 0.6),
  in `tasks.json`, `holdout.json` or the classifier/semantic test cases.
- No label may be templated (more than 20% sharing a 4-word prefix).
- No code or tool may read `fresh.json`; only CI (report step) and docs name it.
- A PR that edits `fresh.json` must not also touch `internal/core/{signals,risk,classifier}.go`,
  `internal/semantic/data/`, learned weights or `tools/minilm/`.
- CI prints its numbers (`go run ./cmd/downshift benchmark benchmark/fresh.json`)
  but does not gate on them: a gate would invite tuning against it.

A misroute found on `fresh.json` is fixed by writing a **new** prompt that shows
the same pattern into `tasks.json` or the edge tests, never by copying the
fresh prompt.

## MiniLM holdout (P4.10)

Evaluation on the 300-task held-out split (`benchmark/holdout.json`). Centroids for the neural row were fit only on `benchmark/tasks.json` by `tools/minilm/eval_holdout.py`.

| Mode | Tier accuracy | FRONTIER→MID | FRONTIER→SMALL |
|---|---|---|---|
| Regex only (`DOWNSHIFT_MINILM=0`) | 100% | 0.0% | 0.0% |
| In-process hash boost (default) | 100% | 0.0% | 0.0% |
| all-MiniLM-L6-v2 nearest centroid | 96.3% | 0.0% | 0.0% |

The neural model does not raise tier accuracy on this split. It clears the remaining FRONTIER→MID misses. The default embedder stays the in-process hash (no Python, no network). Raw neural summary: `benchmark/minilm-holdout.json`.

