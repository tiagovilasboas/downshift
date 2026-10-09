# Feedback loop (user and contributor)

Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift

## What happens on each hook

- A deterministic routing decision runs at the harness boundary (shared `internal/core` policy).
- Telemetry appends one **prompt-free** JSON line per decision under your [state dir](config.md) (`events.jsonl`).
- Each decision gets a `correlation_id` for later review.

`rewrite_emitted` means Downshift **sent** a compatible rewrite. It is not proof
the harness **honored** it. See [harness-matrix.md](harness-matrix.md) and
[evidence/](evidence/).

## Safety properties (summary)

- Fail-open on errors; spawn is not blocked by router failures.
- Owner-only permissions on local event files.
- Low-confidence paths do not silently downshift the harness model.
- Training data rejects unknown labels; feedback is append-only.

## Availability, quota, and report evidence

Availability and quota are separate observations. A model listing proves that a
harness offers a model; it does not prove included credit. A shared usage window
applies to its reported harness or pool, never to a guessed list of model names.
An expired or missing observation cannot establish remaining credit. Keep
`explicit_upshift` off unless the operator enables it.

| Concern | Guia | Sensor | Classification and eixo |
|---|---|---|---|
| Discovery | Documented CLI/cache contracts and precedence | Bounded parsers, source failures, discovery cache expiry tests | Guia inferencial; sensor computacional; architecture fitness and behaviour |
| Quota | Source, scope, freshness and shared-window contract | Quota gate plus malformed, stale, exhausted and cross-harness tests | Guia inferencial; sensor computacional; behaviour |
| Dashboard counts | Count emitted rewrites consistently in every tab | Server computes `applied`; API regression compares per-harness totals | Guia inferencial; sensor computacional; behaviour |

The hooks provide the local runtime sensor. The Go tests run in repository CI;
they establish implementation behaviour against fixtures. A live observation
and a hook result are separate acceptance evidence. Neither a parser test nor
`rewrite_emitted` establishes that the executor honored a rewrite.

## How to improve routing (community)

1. **Misroute reports** — best contribution. See [CONTRIBUTING.md](../CONTRIBUTING.md).
2. **Optional shadow** — `DOWNSHIFT_SHADOW_WEIGHTS` + [classifier-shadow.md](classifier-shadow.md) (observation only).
3. **Offline train/compare** — `downshift train --from-events`, `downshift benchmark --compare` (does not change the production hook).

Classifier tuning detail: [contrib/classifier.md](contrib/classifier.md).

## Maintainer-only detail

Promotion thresholds, historical eval notes, and dsmon internals:
[internal/engineering-loop-maintainer.md](internal/engineering-loop-maintainer.md).
