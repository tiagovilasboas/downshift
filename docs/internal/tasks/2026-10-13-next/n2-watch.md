<!-- Copyright (c) 2026 Tiago Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# N2: `downshift watch`

- `downshift watch [--session=<id>] [--interval=1s]`: redraws the current session tree (reuse
  `telemetry.SessionTree`) in the terminal every interval by tail-reading events.jsonl; plain ANSI,
  no external deps; exits on Ctrl-C; works for every harness (no session id: most recent session).
- Running children (`…`) with no end/usage record after 2 h render as `?` (unknown end) in the
  tree, in `session --tree` too.
- `dsmon` stays buildable but README/install point to `downshift watch`; note in docs.
- Tests: frame rendering with a fake clock and fixture log, timeout to `?`, no-session fallback.
