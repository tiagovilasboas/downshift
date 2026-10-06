# Classifier (v1 production path)

Contributor reference for the deterministic classifier that powers **hook routing**.
User-facing Portuguese overview: [pt/02-classificador.md](../pt/02-classificador.md).

Experimental v2 is documented in [capability-router-v2.md](../capability-router-v2.md).

---

## Complexity classes

| Class | Typical work |
|-------|----------------|
| **TRIVIAL** | Rename, format, typo, trivial git |
| **SIMPLE** | Single-file or isolated change |
| **MEDIUM** | Multi-file feature work |
| **COMPLEX** | Architecture, concurrency, infra, security review depth |

If signals are ambiguous, policy may leave the session model unchanged (fail-open
toward **no rewrite** when confidence is low).

---

## Scoring (`internal/core/signals.go` + `classifier.go`)

1. **Weighted regex vote** — each pattern adds to a complexity bucket; highest sum wins.
2. **Tie-break** — adjacent ties resolve toward the **higher** capability tier (never
   under-route on a tie).
3. **Confidence margin** — a win needs at least **2 points** over the runner-up, or
   the classifier is treated as low-confidence (conservative routing).
4. **Graphify (optional)** — `DOWNSHIFT_GRAPHIFY_CMD` can supply a graph label on stdin;
   failures/timeouts do not escalate complexity. `DOWNSHIFT_GRAPHIFY=0` disables.
5. **Semantic boost (MiniLM)** — `internal/semantic`: centroid distance can **raise**
   complexity when regex is hesitant; it **never lowers** a class. `DOWNSHIFT_MINILM=0` disables.

---

## Tuning workflow

1. Reproduce with `downshift try "<prompt>" [harness]`.
2. Open a `misroute:` issue (see [CONTRIBUTING.md](../../CONTRIBUTING.md)).
3. Add a case to `internal/core/classifier_edge_test.go` (or related edge tests).
4. Adjust `RawSignals` in `signals.go`; never tune against maintainer eval-only splits (downshift-labs).

---

## Shadow / v2 (observation only)

`DOWNSHIFT_SHADOW_WEIGHTS` records a v2 candidate next to the production decision.
It does **not** change hook JSON. Labels for supervised training require explicit
`--required-tier` on feedback. See [classifier-shadow.md](../classifier-shadow.md).

---

## Tests and benchmarks

| Asset | Role |
|-------|------|
| `classifier_edge_test.go` | Regression from misroute reports |
| `benchmark/seed.json` | Train/tune signals (not in same PR as signal edits + fresh) |
| Eval-only splits (labs) | Never use to edit signals in the same change set |
