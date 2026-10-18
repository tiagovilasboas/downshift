<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# Next wave (2026-10-13)

| ID | Task | Roadmap | Files |
|----|------|---------|-------|
| N1 | Built-in control group + cost per completed task + savings report with CI | P1.1-P1.3 | shared runner, `internal/telemetry` (new files) |
| N2 | `downshift watch` (live session tree, replaces the `dsmon` build); running children time out to `?` | U4 | `cmd/downshift/watch.go`, `telemetry/session_tree.go` |
| N3 | Stale quota stops holding after 1 h (becomes unknown) + warnings; `downshift config` | quota fix, X7 | quota gate, `cmd/downshift/config.go`, `docs/guides/quota.md` |
| N4 | README first screen and quickstart around `setup`, `doctor`, `session` | X5 | `README.md`, `docs/guides/install.md` |
| N5 | One shared child-end layer for every harness (`downshift end <harness>`) | layering | `internal/childend`, `internal/hookport`, adapters, [N5](n5-shared-child-end.md) |

Still owner decisions (not in this wave): P2.3 upshift target, P2.4 measure Haiku/Sonnet 5.5,
P1.6 CI eval gate, P2.5 learned scorer labels, downshift-labs suite freeze.
