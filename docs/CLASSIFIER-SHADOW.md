# Candidate classifier shadow (public summary)

Downshift by Tiago de Carvalho Vilas Boas · https://github.com/tiagovilasboas/downshift

Production hooks use **`core.Route`** (deterministic classifier). Shadow mode is
**opt-in observation only**: it records a local candidate’s tier beside the
production recommendation. It does **not** change hook output or spawn models.

## Quick start

```bash
downshift train dataset.json --output=candidate.json
# or: downshift train --from-events --output=candidate.json

export DOWNSHIFT_SHADOW_WEIGHTS=/absolute/path/candidate.json
# run harness hooks as usual

downshift feedback <id> success
# explicit minimum tier only when you reviewed it:
downshift feedback <id> retry --retry-tier=FRONTIER --required-tier=FRONTIER

downshift shadow-report
```

Unset `DOWNSHIFT_SHADOW_WEIGHTS` to disable. Never overwrite production weights
with a candidate file.

## Rules

- Train only on **consented / reviewed** data, not on eval-only splits.
- `success` without `--required-tier` is **not** a supervised tier label.
- Shadow disagreements are **not** proof the candidate model would have passed;
  paired outcome experiments still matter.

## Maintainer detail

Promotion thresholds, full report field guide, and safety boundaries:
**downshift-labs** (`docs/maintainer/CLASSIFIER-SHADOW-FULL.md`).

User-facing loop: [ENGINEERING-LOOP.md](ENGINEERING-LOOP.md).
