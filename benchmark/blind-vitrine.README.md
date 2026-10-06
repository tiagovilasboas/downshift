# blind-vitrine.json: externally authored blind split

Evaluation-only. Guarded like `fresh.json` and `heldout2.json`: no tuning code
may read it, and CI (`scripts/fresh-guard.sh`) rejects a change set that edits
it together with router tuning files. Report it, never fit to it.

## Status: seen (no longer blind)

This set was scored on 2026-10-03: once for the upshift-only design (#35)
and once per frozen variant of the small over-routing cut, with plans
frozen beforehand on tuning data (`docs/design/router-generalization.md`).
Its aggregate results and a few individual items have been read, so it is
**no longer a blind test** for future router decisions. It stays
evaluation-only under the guard: report it, never fit to it. The next
decision needs a new, independently authored split.

## Provenance

- **Author:** a separate AI agent that never saw this repository. It did not
  open, clone, search or read `tiagovilasboas/downshift` (including the
  other benchmark splits), did not read any downshift-related files, and used
  no web search. The only thing it wrote was its own output directory.
- **Written:** 2026-10-03, in a single pass from general software-engineering
  knowledge. Received 2026-10-03 07:07 BRT and committed unchanged
  (sha256 `e1c30e670ec992e11512feca0df89c073a58a9f2b8112ad1836eb775e3461ab6`).
- **Generator:** a Python script that also kept language and task-type tags.
  It is not committed, because it carries the labels next to the prompts and
  must not be fed to any model. Item order was shuffled with
  `random.seed(20261003)`. Generator sha256
  `3e70052c7fb8d1baf043d417686418ef98ab5dcdaf8516ea843f270ccf201e9b`.
- **Checks by the author:** valid JSON, exactly 60 items, 20 per label, no
  duplicate prompts. Checks here: max token-Jaccard to any prompt in
  `tasks.json`, `holdout.json`, `fresh.json` or `heldout2.json` is 0.19, and
  the guard's shape, templating and leakage tests pass.

## Format and labels

A JSON array of `{"prompt", "label"}` with **tier** labels (`small`, `mid`,
`frontier`, 20 each), not the four complexity labels used by the other
splits. The label is the smallest model tier that would reliably solve the task:

- **small**: a ~30B coder-class model. Trivial edits, regex, simple functions,
  renames, obvious bug fixes, unit tests for pure functions, basic SQL or Bash.
- **mid**: multi-step refactors, debugging that takes some reasoning, API
  integration with retries and pagination, joins and window functions, basic
  concurrency, moderate review, CI pipelines.
- **frontier**: subtle concurrency or memory-model bugs, `unsafe` soundness,
  cross-service architecture, non-obvious security review (path traversal,
  SSRF, JWT), hard algorithmic optimization, migration plans with tradeoffs,
  and vague requirements that call for judgment.

Production complexity maps to tiers as in `Complexity.Tier()`: TRIVIAL and
SIMPLE map to small, MEDIUM to mid, COMPLEX to frontier.

Languages: Python 15, TypeScript 8, SQL 7, Go 5, Rust 5, architecture 4,
Java 4, Bash 3, React 3, YAML/CI 2, C++ 1, Docker 1, JavaScript 1, Kubernetes 1.

## Limits

- **Small and unvalidated:** 60 items, one author's labels, no human review and
  no outcome calibration. A 95% CI on tier accuracy is about ±12pp.
- **Judgment calls:** the author flags two borderline items: the zero-downtime
  PostgreSQL column migration (labelled mid) and the deadlock analysis
  (labelled frontier).
- **Distribution shift:** prompts are long and include inline code (median
  472 characters vs 70 in `heldout2.json`), so results here mix label
  disagreement with a length/format shift.
- **Open-ended items:** frontier design prompts have no single correct answer;
  grade them against the requirements stated in each prompt.
