<!-- Copyright (c) 2026 Tiago de Carvalho Vilas Boas. SPDX-License-Identifier: Apache-2.0 -->

# U1: session summary in the Claude Code statusline

Extend the existing opt-in bridge `downshift quota claude-statusline` (it already
reads the statusline JSON on stdin and prints a quota line) so it also prints a
routing summary for the CURRENT session:
`◆ downshift · 6 subagents · 4 on haiku · saved $0.42 · 0 on doubt`
- Match the session: the statusline input has `session_id`; events store a hashed
  session id; hash with the same telemetry function to filter.
- Counts from `events.jsonl` (bounded tail read, e.g. last 2 MB, no full scan):
  spawns, per-model counts (top one), saved USD (signed delta from T3; show a
  loss as "over $x"), frontier-on-doubt count. Omit segments with no data.
- Must render in < 50 ms on a 50 MB log (tail read), never error to the
  statusline (print the quota part alone on any failure).
- Shared formatter `telemetry.SessionSummary(events, sessionHash) Summary` and
  `Summary.Line()`; the bridge only parses, calls and prints.
Tests: formatter table tests (empty, only quota, loss, doubt), session filter,
tail bound, failure fallback. Docs: `docs/quota.md` statusline section.
