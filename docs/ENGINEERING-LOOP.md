# Feedback loop (user and contributor)

Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift

## What happens on each hook

- A deterministic routing decision runs at the harness boundary (shared `internal/core` policy).
- Telemetry appends one **prompt-free** JSON line per decision under your [state dir](CONFIG.md) (`events.jsonl`).
- Each decision gets a `correlation_id` for later review.

`rewrite_emitted` means Downshift **sent** a compatible rewrite. It is not proof
the harness **honored** it. See [HARNESS-MATRIX.md](HARNESS-MATRIX.md) and
[evidence/](evidence/).

## Safety properties (summary)

- Fail-open on errors; spawn is not blocked by router failures.
- Owner-only permissions on local event files.
- Low-confidence paths do not silently downshift the harness model.
- Training data rejects unknown labels; feedback is append-only.

## How to improve routing (community)

1. **Misroute reports** — best contribution. See [CONTRIBUTING.md](../CONTRIBUTING.md).
2. **Optional shadow** — `DOWNSHIFT_SHADOW_WEIGHTS` + [CLASSIFIER-SHADOW.md](CLASSIFIER-SHADOW.md) (observation only).
3. **Offline train/compare** — `downshift train --from-events`, `downshift benchmark --compare` (does not change the production hook).

Classifier tuning detail: [contrib/classifier.md](contrib/classifier.md).

## Maintainer-only detail

Promotion thresholds, historical eval notes, and dsmon internals:
[internal/ENGINEERING-LOOP-MAINTAINER.md](internal/ENGINEERING-LOOP-MAINTAINER.md).
