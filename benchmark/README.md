# Benchmark seed dataset

## What this is

`tasks.json` is a 30-task seed (9 TRIVIAL, 7 SIMPLE, 7 MEDIUM, 7 COMPLEX)
used as a **regression net** for the downshift classifier: it catches
accidental behaviour changes between edits. Each entry is a
`{prompt, label}` pair; run it with `downshift benchmark benchmark/tasks.json`.

## What this is not

It is **not proof of generalisation**. Thirty hand-picked prompts cannot
represent real-world traffic, and accuracy on this file says nothing about
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
    "total_tasks": 108,
    "tier_accuracy": 0.6296,
    "tier_accuracy_ci_95": {
      "low": 0.5370,
      "high": 0.7130
    }
  }
  ```
  The interval bounds prevent mistaking variance across small sample sizes for genuine classifier regressions or gains.

