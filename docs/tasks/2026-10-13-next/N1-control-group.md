<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# N1: control group, cost per completed task, savings report

- Control group in the shared hook runner(s): `DOWNSHIFT_CONTROL_SHARE` (default 0.10; 0 disables)
  picks spawns deterministically by a hash of the routing/correlation id; a control spawn is
  classified and logged but never rewritten (`outcome: baseline`, reason code `CONTROL`). Applies to
  every harness through the shared runner; the existing `DOWNSHIFT_NO_ROUTE` and the session pause
  keep working.
- Task records (`internal/telemetry`, new file): join decision, usage and child status per spawn;
  rework = a respawn in the same session within 10 min after error/interrupt (reuse
  `RespawnAfterFailure`). Cost per completed task = cost of completed children plus their rework.
- `downshift report [--days=N] [--json]`: routed vs control cost per completed task, rework rate per
  group, savings with a 95% bootstrap CI (fixed seed, deterministic), and "not enough data" below
  20 completed tasks per group. Prompt-free.
- Tests: share accuracy within 1 pp over 2,000 ids, determinism, control never rewrites through
  `runHookAdapter`, rework linking, CI on synthetic data, small-sample message.
- Docs: `docs/stats-export.md` and a short `docs/report.md`.
