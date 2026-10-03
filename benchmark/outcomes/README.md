# Outcome eval

The classifier benchmark (`benchmark/tasks.json`) checks agreement with a human
complexity rubric. It never shows that a cheaper model *succeeds* at the task.
This directory measures that: each task comes with an executable check, gets
solved on a given tier, and is scored by running the check.

## Layout

`testdata/<id>/` holds one task. Go tooling ignores `testdata`, so the stub and the
reference can declare the same symbols.

| File | Role |
|---|---|
| `task.json` | `id`, rubric `label` (TRIVIAL/SIMPLE/MEDIUM/COMPLEX), `prompt` |
| `stub.go` | Starting point shown to the model (signatures that panic) |
| `reference.go` | Known-good solution, never shown to the model |
| `check_test.go` | Hidden executable check, never shown to the model |

There are 40 tasks, 10 per label. They use only the standard library and run
with `go test` in a throwaway module, with `solution.go` sitting next to the check.

## Commands

```bash
# Offline, runs in CI. Every stub must fail its check, every reference must pass,
# and every recorded result must reproduce from its recorded solution.
downshift eval-outcome --verify

# Makes model calls. Refused when $CI is set, so it never runs in CI.
# The solver is any shell command that reads the prompt on stdin and prints the answer.
downshift eval-outcome --record --tier=small    --model=claude-haiku-4-5 --solver='claude -p --model claude-haiku-4-5'
downshift eval-outcome --record --tier=frontier --model=claude-opus-4-8  --solver='claude -p --model claude-opus-4-8'

# Regenerate the README numbers block. CI runs --check and fails when the block is stale.
downshift eval-outcome --report --write README.md   # or: make report
```

`--record` writes `runs/<tier>/<id>.go` plus `runs/<tier>/run.json`. Commit both:
`--verify` re-runs every recorded solution, so a result that does not reproduce
fails CI. The report shows pass rates only for runs that were actually recorded.
Until then it says "not measured".

## Reading the result

The report gives the pass rate per run tier on three subsets: all tasks, the
TRIVIAL+SIMPLE labels (small-capable according to the rubric), and the tasks the
router actually sends to small, each with a 95% Wilson interval. If small trails
frontier by more than 5pp on the tasks routed to small, revisit the downshift.

Adding a task: create the four files, check that `--verify` passes, then run
`make report`. Do not copy prompts from `benchmark/fresh.json`.
